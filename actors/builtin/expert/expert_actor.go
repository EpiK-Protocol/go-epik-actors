package expert

import (
	addr "github.com/filecoin-project/go-address"
	abi "github.com/filecoin-project/specs-actors/actors/abi"
	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	vmr "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	. "github.com/filecoin-project/specs-actors/actors/util"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
)

type Runtime = vmr.Runtime

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
		7:                         a.CheckDataDuplicated,
	}
}

var _ abi.Invokee = Actor{}

type ConstructorParams = power.ExpertConstructorParams

func (a Actor) Constructor(rt vmr.Runtime, params *ConstructorParams) *adt.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	owner := resolveOwnerAddress(rt, params.Owner)

	emptyArray, err := adt.MakeEmptyArray(adt.AsStore(rt)).Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to construct initial state: %v", err)
	}

	st := ConstructState(emptyArray, owner, params.PeerId, params.Multiaddrs)
	rt.State().Create(st)
	return nil
}

type GetControlAddressReturn struct {
	Owner addr.Address
}

func (a Actor) ControlAddress(rt Runtime, _ *adt.EmptyValue) *GetControlAddressReturn {
	rt.ValidateImmediateCallerAcceptAny()
	var st State
	rt.State().Readonly(&st)
	return &GetControlAddressReturn{
		Owner: st.Info.Owner,
	}
}

// Resolves an address to an ID address and verifies that it is address of an account or multisig actor.
func resolveOwnerAddress(rt Runtime, raw addr.Address) addr.Address {
	resolved, ok := rt.ResolveAddress(raw)
	if !ok {
		rt.Abortf(exitcode.ErrIllegalArgument, "unable to resolve address %v", raw)
	}
	Assert(resolved.Protocol() == addr.ID)

	ownerCode, ok := rt.GetActorCodeCID(resolved)
	if !ok {
		rt.Abortf(exitcode.ErrIllegalArgument, "no code for address %v", resolved)
	}
	if !builtin.IsPrincipal(ownerCode) {
		rt.Abortf(exitcode.ErrIllegalArgument, "owner actor type must be a principal, was %v", ownerCode)
	}
	return resolved
}

type ChangePeerIDParams struct {
	NewID abi.PeerID
}

func (a Actor) ChangePeerID(rt Runtime, params *ChangePeerIDParams) *adt.EmptyValue {
	var st State
	rt.State().Transaction(&st, func() interface{} {
		rt.ValidateImmediateCallerIs(st.Info.Owner)
		st.Info.PeerId = params.NewID
		return nil
	})
	return nil
}

type ChangeMultiaddrsParams struct {
	NewMultiaddrs []abi.Multiaddrs
}

func (a Actor) ChangeMultiaddrs(rt Runtime, params *ChangeMultiaddrsParams) *adt.EmptyValue {
	var st State
	rt.State().Transaction(&st, func() interface{} {
		rt.ValidateImmediateCallerIs(st.Info.Owner)
		st.Info.Multiaddrs = params.NewMultiaddrs
		return nil
	})
	return nil
}

type ChangeAddressParams struct {
	NewOwner addr.Address
}

func (a Actor) ChangeAddress(rt Runtime, params *ChangeAddressParams) *adt.EmptyValue {
	var st State
	rt.State().Transaction(&st, func() interface{} {
		rt.ValidateImmediateCallerIs(st.Info.Owner)

		owner := resolveOwnerAddress(rt, params.NewOwner)

		st.Info.Owner = owner
		return nil
	})
	return nil
}

type ExpertDataParams struct {
	PieceID cid.Cid
}

func (a Actor) ImportData(rt Runtime, params *ExpertDataParams) *adt.EmptyValue {
	store := adt.AsStore(rt)
	var st State
	rt.State().Transaction(&st, func() interface{} {
		rt.ValidateImmediateCallerIs(st.Info.Owner)

		newDataInfo := &DataOnChainInfo{
			PieceID: params.PieceID,
		}

		if err := st.PutData(store, newDataInfo); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to import data: %v", err)
		}
		return nil
	})
	return nil
}

func (a Actor) CheckDataDuplicated(rt Runtime, params *ExpertDataParams) *adt.EmptyValue {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.State().Readonly(&st)
	store := adt.AsStore(rt)

	if _, found, err := st.GetData(store, params.PieceID); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to load expert data %v", params.PieceID)
	} else if found {
		rt.Abortf(exitcode.ErrNotFound, "data %v has imported", params.PieceID)
	}
	return nil
}
