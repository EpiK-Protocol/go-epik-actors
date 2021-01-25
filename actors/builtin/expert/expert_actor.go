package expert

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
	_ "github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/ipfs/go-cid"
)

type Runtime = runtime.Runtime

// Actor of expert
type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.ControlAddress,
		3:                         a.ChangeAddress,
		4:                         a.ImportData,
		5:                         a.GetData,
		6:                         a.StoreData,
		7:                         a.Nominate,
		8:                         a.NominateUpdate,
		9:                         a.Block,
		10:                        a.BlockUpdate,
		11:                        a.FoundationChange,
		12:                        a.Vote,
		13:                        a.Validate,
	}
}

func (a Actor) Code() cid.Cid {
	return builtin.ExpertActorCodeID
}

func (a Actor) State() cbor.Er {
	return new(State)
}

var _ runtime.VMActor = Actor{}

type ConstructorParams = power.ExpertConstructorParams

func (a Actor) Constructor(rt Runtime, params *ConstructorParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.InitActorAddr)

	if params.Type > 0 && rt.ValueReceived().GreaterThan(ExpertApplyCost) {
		code := rt.Send(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, ExpertApplyCost, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to burn funds")
	}

	owner := resolveOwnerAddress(rt, params.Owner)

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct initial state")

	info, err := ConstructExpertInfo(owner, ExpertType(params.Type), params.ApplicationHash)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "failed to construct initial expert info")
	infoCid := rt.StorePut(info)

	eState := ExpertStateRegistered
	if info.Type == ExpertFoundation {
		eState = ExpertStateNormal
	}

	ownerChange := rt.StorePut(&PendingOwnerChange{
		ApplyOwner: owner,
		ApplyEpoch: abi.ChainEpoch(-1)})
	st := ConstructState(infoCid, emptyMap, eState, ownerChange)
	rt.StateCreate(st)
	return nil
}

type GetControlAddressReturn struct {
	Owner addr.Address
}

func (a Actor) ControlAddress(rt Runtime, _ *abi.EmptyValue) *GetControlAddressReturn {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	var aReturn GetControlAddressReturn
	rt.StateTransaction(&st, func() {
		info := getExpertInfo(rt, &st)
		aReturn.Owner = info.Owner
	})
	return &aReturn
}

func getExpertInfo(rt Runtime, st *State) *ExpertInfo {
	err := st.AutoUpdateOwnerChange(adt.AsStore(rt), rt.CurrEpoch())
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not update owner")
	info, err := st.GetInfo(adt.AsStore(rt))
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not read expert info")
	return info
}

// Resolves an address to an ID address and verifies that it is address of an account or multisig actor.
func resolveOwnerAddress(rt Runtime, raw addr.Address) addr.Address {
	resolved, ok := rt.ResolveAddress(raw)
	builtin.RequireParam(rt, ok, "unable to resolve address %v", raw)
	builtin.RequireParam(rt, resolved.Protocol() == addr.ID, "illegal address protocol %d (expect %d)", resolved.Protocol(), addr.ID)

	ownerCode, ok := rt.GetActorCodeCID(resolved)
	builtin.RequireParam(rt, ok, "no code for address %v", resolved)
	builtin.RequireParam(rt, builtin.IsPrincipal(ownerCode), "owner actor type must be a principal, was %v", ownerCode)
	return resolved
}

type ChangeAddressParams struct {
	NewOwner addr.Address
}

func (a Actor) ChangeAddress(rt Runtime, params *ChangeAddressParams) *abi.EmptyValue {
	var st State
	rt.StateTransaction(&st, func() {
		info := getExpertInfo(rt, &st)
		rt.ValidateImmediateCallerIs(info.Owner)
		owner := resolveOwnerAddress(rt, params.NewOwner)
		info.Owner = owner
		err := st.SaveInfo(adt.AsStore(rt), info)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to change address")
	})
	return nil
}

type ExpertDataParams struct {
	PieceID   cid.Cid `checked:"true"`
	PieceSize abi.PaddedPieceSize
}

func (a Actor) ImportData(rt Runtime, params *ExpertDataParams) *abi.EmptyValue {
	var st State
	store := adt.AsStore(rt)
	rt.StateTransaction(&st, func() {
		info := getExpertInfo(rt, &st)
		rt.ValidateImmediateCallerIs(info.Owner)

		err := st.Validate(adt.AsStore(rt), rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrForbidden, "invalid expert")

		_, found, err := st.GetData(store, params.PieceID.String())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get data")
		if found {
			builtin.RequireNoErr(rt, err, exitcode.ErrForbidden, "duplicate expert import")
		}

		newDataInfo := &DataOnChainInfo{
			PieceID:   params.PieceID.String(),
			PieceSize: params.PieceSize,
		}

		err = st.PutData(store, newDataInfo)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to import data")
	})
	builtin.NotifyExpertImport(rt, rt.Receiver(), params.PieceID)
	return nil
}

func (a Actor) GetData(rt Runtime, params *ExpertDataParams) *DataOnChainInfo {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)
	store := adt.AsStore(rt)

	data, found, err := st.GetData(store, params.PieceID.String())
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load expert data %v", params.PieceID)
	builtin.RequireParam(rt, found, "data %v has not imported", params.PieceID)
	return data
}

func (a Actor) StoreData(rt Runtime, params *ExpertDataParams) *DataOnChainInfo {
	rt.ValidateImmediateCallerType(builtin.ExpertFundActorCodeID)

	var out *DataOnChainInfo
	var st State
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)

		err := st.Validate(store, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrForbidden, "invalid expert")

		data, found, err := st.GetData(store, params.PieceID.String())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load expert data %v", params.PieceID)
		builtin.RequireParam(rt, found, "data %v has not imported", params.PieceID)
		data.Redundancy++
		err = st.PutData(store, data)
		builtin.RequireNoErr(rt, err, exitcode.ErrForbidden, "failed to store data")

		out = data
	})
	return out
}

func (a Actor) Validate(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)

	err := st.Validate(adt.AsStore(rt), rt.CurrEpoch())
	builtin.RequireNoErr(rt, err, exitcode.ErrForbidden, "failed to validate expert")
	return nil
}

type NominateExpertParams struct {
	Expert addr.Address
}

func (a Actor) Nominate(rt Runtime, params *NominateExpertParams) *abi.EmptyValue {

	var st State
	rt.StateTransaction(&st, func() {
		info := getExpertInfo(rt, &st)
		rt.ValidateImmediateCallerIs(info.Owner)

		err := st.Validate(adt.AsStore(rt), rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrForbidden, "invalid expert")
	})

	code := rt.Send(params.Expert, builtin.MethodsExpert.NominateUpdate, nil, abi.NewTokenAmount(0), &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to nominate expert")
	return nil
}

func (a Actor) NominateUpdate(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		info := getExpertInfo(rt, &st)
		info.Proposer = rt.Caller()
		err := st.SaveInfo(adt.AsStore(rt), info)
		builtin.RequireNoErr(rt, err, exitcode.ErrForbidden, "failed to update nominate")

		st.Status = ExpertStateNormal
	})
	return nil
}

func (a Actor) Block(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.GovernActorCodeID)

	var st State
	var info *ExpertInfo
	rt.StateTransaction(&st, func() {
		info = getExpertInfo(rt, &st)

		if info.Type == ExpertFoundation {
			rt.Abortf(exitcode.ErrIllegalArgument, "foundation expert cannot be blocked")
		}

		st.Status = ExpertStateBlocked
	})

	code := rt.Send(info.Proposer, builtin.MethodsExpert.BlockUpdate, nil, abi.NewTokenAmount(0), &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to nominate expert")

	builtin.NotifyExpertUpdate(rt, rt.Receiver())
	return nil
}

func (a Actor) BlockUpdate(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

	validate := true
	var st State
	rt.StateTransaction(&st, func() {
		info := getExpertInfo(rt, &st)
		if info.Type != ExpertFoundation {
			st.Status = ExpertStateImplicated
			if st.VoteAmount.GreaterThanEqual(ExpertVoteThreshold) &&
				st.VoteAmount.LessThan(ExpertVoteThresholdAddition) {
				st.LostEpoch = rt.CurrEpoch()
			}
		}
		if err := st.Validate(adt.AsStore(rt), rt.CurrEpoch()); err != nil {
			validate = false
		}
	})
	if !validate {
		builtin.NotifyExpertUpdate(rt, rt.Receiver())
	}
	return nil
}

type FoundationChangeParams struct {
	Owner addr.Address
}

func (a Actor) FoundationChange(rt Runtime, params *FoundationChangeParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.GovernActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		err := st.ApplyOwnerChange(adt.AsStore(rt), rt.CurrEpoch(), params.Owner)
		builtin.RequireNoErr(rt, err, exitcode.ErrForbidden, "failed to change expert owner")
	})
	return nil
}

type ExpertVoteParams struct {
	Amount abi.TokenAmount
}

func (a Actor) Vote(rt Runtime, params *ExpertVoteParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.VoteFundActorCodeID)

	validate := true
	var st State
	rt.StateTransaction(&st, func() {
		if st.Status == ExpertStateBlocked || st.Status == ExpertStateRegistered {
			rt.Abortf(exitcode.ErrForbidden, "expert cannot be vote")
		}
		st.VoteAmount = params.Amount

		if err := st.Validate(adt.AsStore(rt), rt.CurrEpoch()); err != nil {
			validate = false
		}
	})
	if !validate {
		builtin.NotifyExpertUpdate(rt, rt.Receiver())
	}
	return nil
}
