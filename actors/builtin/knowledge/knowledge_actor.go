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
		3:                         a.AssignUndistributed,
		4:                         a.ApplyRewards,
		5:                         a.WithdrawBalance,
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

func (a Actor) Constructor(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")

	st := ConstructState(emptyMap, addr.Undef)
	rt.StateCreate(st)
	return nil
}

type ChangePayeeParams struct {
	Payee addr.Address
}

func (a Actor) ChangePayee(rt Runtime, params *ChangePayeeParams) *abi.EmptyValue {
	// TODO: caller must be fundation?

	newPayee := params.Payee
	if newPayee != addr.Undef {
		newPayee, _ = resolvePayeeAddress(rt, newPayee)
	}

	var st State
	rt.StateTransaction(&st, func() {
		st.Payee = newPayee
	})
	return nil
}

type AssignUndistributedParams struct {
	Payee  addr.Address
	Amount abi.TokenAmount
}

// Assign undistributed fund to specified payee.
func (a Actor) AssignUndistributed(rt Runtime, params *AssignUndistributedParams) *abi.EmptyValue {
	builtin.RequireParam(rt, params.Payee != addr.Undef, "payee address is undef")
	// TODO: caller must be fundation?
	// TODO: test assign to burnt funds addr
	// TODO: test assign to undef
	payee, _ := resolvePayeeAddress(rt, params.Payee)
	amount := params.Amount

	builtin.RequireParam(rt, amount.GreaterThanEqual(big.Zero()), "negative amount to assign")

	var st State
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)

		builtin.RequireParam(rt, amount.LessThanEqual(st.TotalUndistributed),
			"insufficient undistributed %v, requested %v", st.TotalUndistributed, amount)
		st.TotalUndistributed = big.Sub(st.TotalUndistributed, amount)

		// to burnt fund address
		if payee == builtin.BurntFundsActorAddr {
			code := rt.Send(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, amount, &builtin.Discard{})
			builtin.RequireSuccess(rt, code, "failed to send %v to burnt funds address", amount)
			return
		}

		{
			st.TotalDistributed = big.Add(st.TotalDistributed, amount)

			distributions, err := adt.AsMap(store, st.Distributions)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load distributions")

			err = st.addDistribution(distributions, payee, amount)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add funds")

			st.Distributions, err = distributions.Root()
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush distributions")
		}
	})

	return nil
}

func (a Actor) ApplyRewards(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.RewardActorAddr)

	amount := rt.ValueReceived()
	builtin.RequireParam(rt, amount.GreaterThanEqual(big.Zero()), "negative funds to apply")

	var st State
	rt.StateTransaction(&st, func() {
		switch st.Payee {
		case addr.Undef, builtin.BurntFundsActorAddr:
			st.TotalUndistributed = big.Add(st.TotalUndistributed, amount)
			return
		// TODO:
		// case builtin.BurntFundsActorAddr:
		// 	code := rt.Send(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, amount, &builtin.Discard{})
		// 	builtin.RequireSuccess(rt, code, "failed to send %v to burnt funds address", amount)
		// 	return
		default:
			st.TotalDistributed = big.Add(st.TotalDistributed, amount)

			distributions, err := adt.AsMap(adt.AsStore(rt), st.Distributions)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load distributions")

			err = st.addDistribution(distributions, st.Payee, amount)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add funds")

			st.Distributions, err = distributions.Root()
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush distributions")
		}
	})
	return nil
}

type WithdrawBalanceParams struct {
	Recipient addr.Address
	Amount    abi.TokenAmount
}

func (a Actor) WithdrawBalance(rt Runtime, params *WithdrawBalanceParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	recipient := rt.Caller()
	amount := params.Amount
	builtin.RequireParam(rt, amount.GreaterThanEqual(big.Zero()), "negative amount %v", amount)

	var st State
	rt.StateTransaction(&st, func() {
		builtin.RequireParam(rt, amount.LessThanEqual(st.TotalDistributed), "insufficient balance")
		st.TotalDistributed = big.Sub(st.TotalDistributed, amount)

		distributions, err := adt.AsMap(adt.AsStore(rt), st.Distributions)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load distributions")

		err = st.withdrawDistribution(distributions, recipient, amount)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to withdraw balance")

		st.Distributions, err = distributions.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush distributions")

		code := rt.Send(recipient, builtin.MethodSend, nil, amount, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds")
	})
	return nil
}

func resolvePayeeAddress(rt Runtime, raw addr.Address) (addr.Address, []addr.Address) {
	resolved, ok := rt.ResolveAddress(raw)
	builtin.RequireParam(rt, ok, "unable to resolve address %v", raw)

	codeCID, ok := rt.GetActorCodeCID(resolved)
	builtin.RequireParam(rt, ok, "no code for address %v", resolved)

	if codeCID == builtin.StorageMinerActorCodeID {
		owner, worker, _ := builtin.RequestMinerControlAddrs(rt, resolved)
		// TODO: owner principal?
		return owner, []addr.Address{owner, worker}
	}

	//TODO:
	builtin.RequireParam(rt, builtin.IsPrincipal(codeCID), "actor type must be a principle, was: %v", codeCID)
	return resolved, []addr.Address{resolved}
}
