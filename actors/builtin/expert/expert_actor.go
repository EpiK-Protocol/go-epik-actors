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
		7:                         a.CheckData,
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

	owner := resolveOwnerAddress(rt, params.Owner)

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct initial state")

	info, err := ConstructExpertInfo(owner, params.PeerId, params.Multiaddrs)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "failed to construct initial expert info")
	infoCid := rt.StorePut(info)

	st := ConstructState(infoCid, emptyMap)
	rt.StateCreate(st)
	return nil
}

type GetControlAddressReturn struct {
	Owner addr.Address
}

func (a Actor) ControlAddress(rt Runtime, _ *abi.EmptyValue) *GetControlAddressReturn {
	rt.ValidateImmediateCallerAcceptAny()
	var st State
	rt.StateReadonly(&st)
	info := getExpertInfo(rt, &st)
	return &GetControlAddressReturn{
		Owner: info.Owner,
	}
	return nil
}

func getExpertInfo(rt Runtime, st *State) *ExpertInfo {
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
	PieceID cid.Cid `checked:"true"`
	Bounty  string
}

func (a Actor) ImportData(rt Runtime, params *ExpertDataParams) *abi.EmptyValue {
	var st State
	store := adt.AsStore(rt)
	rt.StateTransaction(&st, func() {
		info := getExpertInfo(rt, &st)
		rt.ValidateImmediateCallerIs(info.Owner)

		newDataInfo := &DataOnChainInfo{
			PieceID: params.PieceID.String(),
			Bounty:  params.Bounty,
		}

		err := st.PutData(store, newDataInfo)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to import data")
	})
	return nil
}

func (a Actor) CheckData(rt Runtime, params *ExpertDataParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)
	store := adt.AsStore(rt)

	// if params.PieceID.String() != TestDataRootID {
	// 	rt.Abortf(exitcode.ErrNotFound, "data %v has not imported", params.PieceID)
	// }
	// fmt.Println("check data success")

	_, found, err := st.GetData(store, params.PieceID.String())
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load expert data %v", params.PieceID)
	builtin.RequireParam(rt, found, "data %v has not imported", params.PieceID)
	return nil
}
