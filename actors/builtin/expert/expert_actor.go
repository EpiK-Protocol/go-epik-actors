package expert

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
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
		10:                        a.OnImplicated,
		11:                        a.ChangeOwner,
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

type ConstructorParams struct {
	Owner addr.Address // ID address
	// ApplicationHash expert application hash
	ApplicationHash string

	// owner itself or agent, diffs from ExpertInfo.Proposer
	Proposer addr.Address // ID address

	// Type expert type
	Type builtin.ExpertType
}

func (a Actor) Constructor(rt Runtime, params *ConstructorParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.InitActorAddr)

	if params.Type != builtin.ExpertFoundation {
		builtin.RequireParam(rt, rt.ValueReceived().GreaterThanEqual(ExpertApplyCost), "fund for expert proposal not enough")
		code := rt.Send(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, ExpertApplyCost, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to burn funds")

		change := big.Sub(rt.ValueReceived(), ExpertApplyCost)
		if !change.IsZero() {
			code := rt.Send(params.Proposer, builtin.MethodSend, nil, change, &builtin.Discard{})
			builtin.RequireSuccess(rt, code, "failed to send change funds")
		}
	}

	owner := resolveOwnerAddress(rt, params.Owner)
	info := &ExpertInfo{
		Owner:           owner,
		Proposer:        owner, // same with owner means not nominated
		Type:            params.Type,
		ApplicationHash: params.ApplicationHash,
	}
	infoCid := rt.StorePut(info)

	eState := ExpertStateRegistered
	if info.Type == builtin.ExpertFoundation {
		eState = ExpertStateNormal
	}

	ownerChange := rt.StorePut(&PendingOwnerChange{
		ApplyOwner: owner,
		ApplyEpoch: abi.ChainEpoch(-1)})
	st, err := ConstructState(adt.AsStore(rt), infoCid, eState, ownerChange)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct initial state")
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
	RootID    string
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
			RootID:    params.RootID,
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
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)
	builtin.ValidateCallerGranted(rt, rt.Caller(), builtin.MethodsExpert.Block)

	var st State
	var info *ExpertInfo
	rt.StateTransaction(&st, func() {
		info = getExpertInfo(rt, &st)

		builtin.RequireParam(rt, info.Type != builtin.ExpertFoundation, "foundation expert cannot be blocked")
		builtin.RequireParam(rt, st.Status != ExpertStateBlocked, "expert already blocked")
		builtin.RequireParam(rt, st.Status != ExpertStateRegistered, "non-nominated cannot be blocked")

		st.Status = ExpertStateBlocked
	})

	code := rt.Send(info.Proposer, builtin.MethodsExpert.OnImplicated, nil, abi.NewTokenAmount(0), &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to punish proposer expert")

	builtin.NotifyExpertFundReset(rt)
	return nil
}

func (a Actor) OnImplicated(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

	validate := true
	var st State
	rt.StateTransaction(&st, func() {
		info := getExpertInfo(rt, &st)
		if info.Type == builtin.ExpertFoundation {
			// foundation will not be punished forever
			return
		}

		// TODO: repeatly punish?
		st.Status = ExpertStateImplicated
		if st.VoteAmount.GreaterThanEqual(ExpertVoteThreshold) &&
			st.VoteAmount.LessThan(ExpertVoteThresholdAddition) {
			st.LostEpoch = rt.CurrEpoch()
		}
		if err := st.Validate(adt.AsStore(rt), rt.CurrEpoch()); err != nil {
			validate = false
		}
	})
	if !validate {
		builtin.NotifyExpertFundReset(rt)
	}
	return nil
}

type ChangeOwnerParams struct {
	Owner addr.Address
}

func (a Actor) ChangeOwner(rt Runtime, params *ChangeOwnerParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)
	builtin.ValidateCallerGranted(rt, rt.Caller(), builtin.MethodsExpert.ChangeOwner)

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
		builtin.NotifyExpertFundReset(rt)
	}
	return nil
}
