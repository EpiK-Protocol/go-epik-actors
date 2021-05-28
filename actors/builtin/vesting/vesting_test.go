package vesting_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/vesting"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/filecoin-project/specs-actors/v2/support/mock"
	"github.com/stretchr/testify/require"

	tutils "github.com/filecoin-project/specs-actors/v2/support/testing"
)

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, vesting.Actor{})
}

func getState(rt *mock.Runtime) *vesting.State {
	var st vesting.State
	rt.GetState(&st)
	return &st
}

func TestConstruction(t *testing.T) {
	actor := vesting.Actor{}
	builder := mock.NewBuilder(context.Background(), builtin.VestingActorAddr).
		WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("simple construction", func(t *testing.T) {
		rt := builder.Build(t)
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		ret := rt.Call(actor.Constructor, nil)
		require.Nil(t, ret)
		rt.Verify()
	})
}

var setupFunc = func(t *testing.T) (*mock.Runtime, *actorHarness) {
	builder := mock.NewBuilder(context.Background(), builtin.VestingActorAddr).
		WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
	rt := builder.Build(t)
	actor := newHarness(t)
	actor.constructAndVerify(rt)
	return rt, actor
}

func TestAddVestingFunds(t *testing.T) {
	coinbase1 := tutils.NewIDAddr(t, 100)
	miner1 := tutils.NewIDAddr(t, 1000)
	coinbase2 := tutils.NewIDAddr(t, 101)
	miner2 := tutils.NewIDAddr(t, 1001)

	t.Run("invalid param", func(t *testing.T) {
		rt, actor := setupFunc(t)

		rt.SetCaller(miner1, builtin.StorageMinerActorCodeID)
		rt.ExpectValidateCallerType(builtin.StorageMinerActorCodeID)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "received funds not equal to param", func() {
			rt.Call(actor.AddVestingFunds, &vesting.AddVestingFundsParams{
				Coinbase: coinbase1,
				Amount:   abi.NewTokenAmount(1),
			})
		})
	})

	t.Run("ok", func(t *testing.T) {
		rt, actor := setupFunc(t)

		rt.SetEpoch(100)
		actor.addLockedFunds(rt, miner1, coinbase1, abi.NewTokenAmount(16))
		actor.addLockedFunds(rt, miner1, coinbase1, abi.NewTokenAmount(100))

		st := getState(rt)
		amount1, err := st.GetMinerCumulation(adt.AsStore(rt), miner1)
		require.NoError(t, err)
		require.True(t, st.LockedFunds.Int64() == 116 && amount1.Int64() == 116)
		vf, found, err := st.LoadVestingFunds(adt.AsStore(rt), coinbase1) // &{[{24480 23} {27360 17} {30240 16} {33120 17} {36000 17} {38880 16} {41760 10}] 0}
		require.NoError(t, err)
		require.True(t, found && vf.UnlockedBalance.IsZero())

		rt.SetEpoch(2210)
		actor.addLockedFunds(rt, miner2, coinbase2, abi.NewTokenAmount(200))
		st = getState(rt)
		amount2, err := st.GetMinerCumulation(adt.AsStore(rt), miner2)
		require.True(t, st.LockedFunds.Int64() == 316 && amount2.Int64() == 200)
		vf, found, err = st.LoadVestingFunds(adt.AsStore(rt), coinbase2) //&{[{25920 35} {28800 28} {31680 29} {34560 28} {37440 29} {40320 29} {43200 22}] 0}
		require.NoError(t, err)
		require.True(t, found && vf.UnlockedBalance.IsZero())

		rt.SetEpoch(24480)
		actor.addLockedFunds(rt, miner1, coinbase1, abi.NewTokenAmount(100))
		st = getState(rt)
		require.True(t, st.LockedFunds.Int64() == 416)

		// some funds unlocked
		rt.SetEpoch(24481)
		actor.addLockedFunds(rt, miner2, coinbase2, abi.NewTokenAmount(100))
		st = getState(rt)
		require.True(t, st.LockedFunds.Int64() == 516, st.LockedFunds)

		actor.addLockedFunds(rt, miner1, coinbase1, abi.NewTokenAmount(100))
		st = getState(rt)
		require.True(t, st.LockedFunds.Int64() == 593) // 516 + 100 - 23

		vf, found, err = st.LoadVestingFunds(adt.AsStore(rt), coinbase1)
		require.NoError(t, err)
		require.True(t, found && vf.UnlockedBalance.Int64() == 23)
		vf, found, err = st.LoadVestingFunds(adt.AsStore(rt), coinbase2)
		require.NoError(t, err)
		require.True(t, found && vf.UnlockedBalance.IsZero())

		amount1, err = st.GetMinerCumulation(adt.AsStore(rt), miner1)
		require.NoError(t, err)
		amount2, err = st.GetMinerCumulation(adt.AsStore(rt), miner2)
		require.NoError(t, err)
		require.True(t, amount1.Int64() == 316 && amount2.Int64() == 300)
	})
}

func TestWithdraw(t *testing.T) {
	coinbase1 := tutils.NewIDAddr(t, 100)
	miner1 := tutils.NewIDAddr(t, 1000)
	coinbase2 := tutils.NewIDAddr(t, 101)
	miner2 := tutils.NewIDAddr(t, 1001)

	t.Run("ok", func(t *testing.T) {
		rt, actor := setupFunc(t)

		actor.withdrawBalance(rt, coinbase1, abi.NewTokenAmount(1000), big.Zero())

		rt.SetEpoch(100)
		actor.addLockedFunds(rt, miner1, coinbase1, abi.NewTokenAmount(16))
		actor.addLockedFunds(rt, miner1, coinbase1, abi.NewTokenAmount(100))
		rt.SetBalance(abi.NewTokenAmount(116))

		rt.SetEpoch(110)
		actor.addLockedFunds(rt, miner2, coinbase2, abi.NewTokenAmount(200))
		rt.SetBalance(abi.NewTokenAmount(316))

		rt.SetEpoch(24480)
		actor.withdrawBalance(rt, coinbase1, abi.NewTokenAmount(1000), big.Zero())

		rt.SetEpoch(24481)
		actor.withdrawBalance(rt, coinbase1, abi.NewTokenAmount(1000), abi.NewTokenAmount(23))
		st := getState(rt)
		require.True(t, st.LockedFunds.Int64() == 293)

		vf, found, err := st.LoadVestingFunds(adt.AsStore(rt), coinbase1)
		require.NoError(t, err)
		require.True(t, found && vf.UnlockedBalance.IsZero())
		vf, found, err = st.LoadVestingFunds(adt.AsStore(rt), coinbase2)
		require.NoError(t, err)
		require.True(t, found && vf.UnlockedBalance.IsZero())
	})
}

type actorHarness struct {
	vesting.Actor
	t *testing.T
}

func newHarness(t *testing.T) *actorHarness {
	return &actorHarness{
		Actor: vesting.Actor{},
		t:     t,
	}
}

func (h *actorHarness) constructAndVerify(rt *mock.Runtime) {
	rt.SetCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	ret := rt.Call(h.Actor.Constructor, nil)
	require.Nil(h.t, ret)
	rt.Verify()

	st := getState(rt)

	vestings, err := adt.AsMap(adt.AsStore(rt), st.CoinbaseVestings, builtin.DefaultHamtBitwidth)
	require.NoError(h.t, err)
	keys, err := vestings.CollectKeys()
	require.NoError(h.t, err)
	require.Empty(h.t, keys)

	cumulations, err := adt.AsMap(adt.AsStore(rt), st.MinerCumulations, builtin.DefaultHamtBitwidth)
	require.NoError(h.t, err)
	keys, err = cumulations.CollectKeys()
	require.NoError(h.t, err)
	require.Empty(h.t, keys)

	require.True(h.t, st.LockedFunds.IsZero())
}

func (h *actorHarness) addLockedFunds(rt *mock.Runtime, miner, coinbase address.Address, amount abi.TokenAmount) {
	rt.SetReceived(amount)
	rt.SetCaller(miner, builtin.StorageMinerActorCodeID)
	rt.ExpectValidateCallerType(builtin.StorageMinerActorCodeID)
	rt.Call(h.AddVestingFunds, &vesting.AddVestingFundsParams{
		Coinbase: coinbase,
		Amount:   amount,
	})
	rt.Verify()
}

func (h *actorHarness) withdrawBalance(rt *mock.Runtime, caller address.Address, requestedAmount, actual abi.TokenAmount) {
	rt.SetCaller(caller, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
	if !actual.IsZero() {
		rt.ExpectSend(caller, builtin.MethodSend, nil, actual, nil, exitcode.Ok)
	}

	rt.Call(h.WithdrawBalance, &vesting.WithdrawBalanceParams{
		AmountRequested: requestedAmount,
	})
	rt.Verify()
}
