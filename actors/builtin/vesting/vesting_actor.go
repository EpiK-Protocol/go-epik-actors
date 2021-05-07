package vesting

import (
	"github.com/filecoin-project/go-address"
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
		2:                         a.AddVestingFunds,
		3:                         a.WithdrawBalance,
	}
}

func (a Actor) Code() cid.Cid {
	return builtin.VestingActorCodeID
}

func (a Actor) IsSingleton() bool {
	return true
}

func (a Actor) State() cbor.Er { return new(State) }

var _ runtime.VMActor = Actor{}

func (a Actor) Constructor(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	st, err := ConstructState(adt.AsStore(rt))
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")

	rt.StateCreate(st)
	return nil
}

type AddVestingFundsParams struct {
	Coinbase address.Address
	Amount   abi.TokenAmount
}

func (a Actor) AddVestingFunds(rt Runtime, params *AddVestingFundsParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)
	builtin.RequireParam(rt, rt.ValueReceived().Equals(params.Amount), "received funds %s not match expected %s", rt.ValueReceived(), params.Amount)

	var st State
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)

		vf, found, err := st.LoadVestingFunds(store, params.Coinbase)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load vesting fouds")
		if !found {
			vf = &VestingFunds{
				UnlockedBalance: abi.NewTokenAmount(0),
			}
		}

		err = st.AddLockedFunds(vf, rt.CurrEpoch(), params.Amount)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add locked funds")

		err = st.SaveVestingFunds(store, params.Coinbase, vf)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save vestings")
	})

	return nil
}

type WithdrawBalanceParams struct {
	AmountRequested abi.TokenAmount
}

func (a Actor) WithdrawBalance(rt Runtime, params *WithdrawBalanceParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	actual := abi.NewTokenAmount(0)
	var st State
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)

		vf, found, err := st.LoadVestingFunds(store, rt.Caller())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load vesting fouds")
		if !found {
			return
		}

		actual, err = st.WithdrawVestedFunds(vf, rt.CurrEpoch(), params.AmountRequested)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to unlock funds of %s", rt.Caller())

		err = st.SaveVestingFunds(store, rt.Caller(), vf)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save vestings")
	})

	if actual.GreaterThan(big.Zero()) {
		code := rt.Send(rt.Caller(), builtin.MethodSend, nil, actual, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds")
	}
	return nil
}
