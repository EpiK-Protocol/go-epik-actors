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
		4:                         a.ChangePeerID,
		5:                         a.ChangeMultiaddrs,
		6:                         a.ImportData,
		7:                         a.GetData,
		8:                         a.StoreData,
		9:                         a.Nominate,
		10:                        a.NominateUpdate,
		11:                        a.Block,
		12:                        a.BlockUpdate,
		13:                        a.FoundationChange,
		14:                        a.Vote,
		15:                        a.Validate,
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

	info, err := ConstructExpertInfo(owner, params.PeerId, params.Multiaddrs, ExpertType(params.Type), params.ApplicationHash)
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
	err := st.AutoUpdateOwnerChange(rt)
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

type ChangePeerIDParams struct {
	NewID abi.PeerID
}

func (a Actor) ChangePeerID(rt Runtime, params *ChangePeerIDParams) *abi.EmptyValue {
	var st State
	rt.StateTransaction(&st, func() {
		info := getExpertInfo(rt, &st)
		rt.ValidateImmediateCallerIs(info.Owner)
		info.PeerId = params.NewID
		st.SaveInfo(adt.AsStore(rt), info)

	})
	return nil
}

type ChangeMultiaddrsParams struct {
	NewMultiaddrs []abi.Multiaddrs
}

func (a Actor) ChangeMultiaddrs(rt Runtime, params *ChangeMultiaddrsParams) *abi.EmptyValue {
	var st State
	rt.StateTransaction(&st, func() {
		info := getExpertInfo(rt, &st)
		rt.ValidateImmediateCallerIs(info.Owner)
		info.Multiaddrs = params.NewMultiaddrs
		st.SaveInfo(adt.AsStore(rt), info)
	})
	return nil
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
		st.SaveInfo(adt.AsStore(rt), info)
	})
	return nil
}

type ExpertDataParams struct {
	PieceID   cid.Cid `checked:"true"`
	PieceSize abi.PaddedPieceSize
	Bounty    string
}

func (a Actor) ImportData(rt Runtime, params *ExpertDataParams) *abi.EmptyValue {
	var st State
	store := adt.AsStore(rt)
	rt.StateTransaction(&st, func() {
		info := getExpertInfo(rt, &st)
		rt.ValidateImmediateCallerIs(info.Owner)

		err := st.Validate(rt)
		builtin.RequireNoErr(rt, err, exitcode.ErrForbidden, "invalid expert")

		newDataInfo := &DataOnChainInfo{
			PieceID: params.PieceID.String(),
		}

		err = st.PutData(store, newDataInfo)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to import data")
	})
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

func (a Actor) StoreData(rt Runtime, params *ExpertDataParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertFundActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)
		data, found, err := st.GetData(store, params.PieceID.String())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load expert data %v", params.PieceID)
		builtin.RequireParam(rt, found, "data %v has not imported", params.PieceID)
		data.Redundancy++
		st.PutData(store, data)
	})
	builtin.NotifyExpertUpdate(rt, rt.Receiver(), params.PieceID)
	return nil
}

func (a Actor) Validate(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)

	err := st.Validate(rt)
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

		err := st.Validate(rt)
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
		st.SaveInfo(adt.AsStore(rt), info)

		st.Status = ExpertStateNormal
	})
	builtin.NotifyExpertUpdate(rt, rt.Receiver(), cid.Undef)
	return nil
}

func (a Actor) Block(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.GovernActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		info := getExpertInfo(rt, &st)

		st.Status = ExpertStateBlocked

		code := rt.Send(info.Proposer, builtin.MethodsExpert.BlockUpdate, nil, abi.NewTokenAmount(0), &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to nominate expert")
	})
	builtin.NotifyExpertUpdate(rt, rt.Receiver(), cid.Undef)
	return nil
}

func (a Actor) BlockUpdate(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

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
	})
	builtin.NotifyExpertUpdate(rt, rt.Receiver(), cid.Undef)
	return nil
}

type FoundationChangeParams struct {
	Owner addr.Address
}

func (a Actor) FoundationChange(rt Runtime, params *FoundationChangeParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.GovernActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		st.ApplyOwnerChange(rt, params.Owner)
	})
	return nil
}

type ExpertVoteParams struct {
	Amount abi.TokenAmount
}

func (a Actor) Vote(rt Runtime, params *ExpertVoteParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertFundActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		st.VoteAmount = params.Amount
	})
	builtin.NotifyExpertUpdate(rt, rt.Receiver(), cid.Undef)
	return nil
}
