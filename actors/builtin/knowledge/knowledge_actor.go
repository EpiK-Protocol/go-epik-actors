package knowledge

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/ipfs/go-cid"
)

type Runtime = runtime.Runtime

type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.ChangePayee,
		3:                         a.ApplyRewards,
	}
}

func (a Actor) Code() cid.Cid {
	return builtin.KnowledgeActorCodeID
}

func (a Actor) IsSingleton() bool {
	return true
}

func (a Actor) State() cbor.Er {
	return new(State)
}

var _ runtime.VMActor = Actor{}

////////////////////////////////////////////////////////////////////////////////
// Actor methods
////////////////////////////////////////////////////////////////////////////////

func (a Actor) Constructor(rt Runtime, payee addr.Address) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")

	st := ConstructState(emptyMap, payee)
	rt.StateCreate(st)
	return nil
}

type ChangePayeeParams struct {
	Payee addr.Address
}

func (a Actor) ChangePayee(rt Runtime, params *ChangePayeeParams) *abi.EmptyValue {
	// TODO: caller must be fundation?

	newPayee, ok := rt.ResolveAddress(params.Payee)
	builtin.RequireParam(rt, ok, "unable to resolve address %v", params.Payee)

	code, ok := rt.GetActorCodeCID(newPayee)
	builtin.RequireParam(rt, ok, "no code for address %v", newPayee)
	builtin.RequireParam(rt, builtin.IsPrincipal(code), "payee must be a principal")

	var st State
	rt.StateTransaction(&st, func() {
		st.Payee = newPayee
	})
	return nil
}

func (a Actor) ApplyRewards(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.RewardActorAddr)

	amount := rt.ValueReceived()
	builtin.RequireParam(rt, amount.GreaterThanEqual(big.Zero()), "negative funds to apply")

	if amount.IsZero() {
		return nil
	}

	var st State
	rt.StateTransaction(&st, func() {
		tally, err := adt.AsMap(adt.AsStore(rt), st.Tally)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load tally")

		var out abi.TokenAmount
		found, err := tally.Get(abi.AddrKey(st.Payee), &out)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get tally")

		if !found {
			out = amount
		} else {
			out = big.Add(out, amount)
		}

		err = tally.Put(abi.AddrKey(st.Payee), &out)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put tally")

		st.Tally, err = tally.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush distributions")

		code := rt.Send(st.Payee, builtin.MethodSend, nil, amount, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds")
	})

	return nil
}
