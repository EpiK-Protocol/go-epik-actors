package knowledge_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/knowledge"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/filecoin-project/specs-actors/v2/support/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tutil "github.com/filecoin-project/specs-actors/v2/support/testing"
	cid "github.com/ipfs/go-cid"
)

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, knowledge.Actor{})
}

func TestConstruction(t *testing.T) {

	actor := knowledge.Actor{}
	builder := mock.NewBuilder(context.Background(), builtin.KnowledgeFundActorAddr).
		WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("initial payee not set", func(t *testing.T) {
		rt := builder.Build(t)
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "must be an ID address", func() {
			rt.Call(actor.Constructor, &address.Undef)
		})
	})

	t.Run("simple construction", func(t *testing.T) {

		payee := tutil.NewIDAddr(t, 101)

		rt := builder.Build(t)
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		ret := rt.Call(actor.Constructor, &payee)
		assert.Nil(t, ret)
		rt.Verify()
	})
}

func TestChangePayee(t *testing.T) {
	governor := tutil.NewIDAddr(t, 101)
	initialPayee := tutil.NewIDAddr(t, 102)
	newPayee := tutil.NewIDAddr(t, 103)

	setupFunc := func(caller address.Address) (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), builtin.KnowledgeFundActorAddr).
			WithActorType(governor, builtin.AccountActorCodeID).
			WithActorType(initialPayee, builtin.AccountActorCodeID).
			WithActorType(newPayee, builtin.AccountActorCodeID)
		rt := builder.Build(t)

		actor := newHarness(t, governor, initialPayee)
		actor.constructAndVerify(rt)

		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		grantCode := exitcode.ErrForbidden
		if caller == governor {
			grantCode = exitcode.Ok
		}
		rt.ExpectSend(builtin.GovernActorAddr,
			builtin.MethodsGovern.ValidateGranted,
			&builtin.ValidateGrantedParams{
				Caller: caller,
				Method: builtin.MethodsKnowledge.ChangePayee,
			},
			big.Zero(),
			nil,
			grantCode,
		)

		return rt, actor
	}

	t.Run("caller is not granted", func(t *testing.T) {
		caller := tutil.NewIDAddr(t, 999)
		rt, actor := setupFunc(caller)
		rt.SetCaller(caller, builtin.AccountActorCodeID)

		rt.ExpectAbort(exitcode.ErrForbidden, func() {
			actor.changePayee(rt, newPayee)
		})
	})

	t.Run("unresolvable new payee", func(t *testing.T) {
		rt, actor := setupFunc(governor)
		rt.SetCaller(governor, builtin.AccountActorCodeID)

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "unable to resolve address", func() {
			actor.changePayee(rt, tutil.NewActorAddr(t, "fake"))
		})
	})

	t.Run("no code found for new payee", func(t *testing.T) {
		rt, actor := setupFunc(governor)
		rt.SetCaller(governor, builtin.AccountActorCodeID)

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "no code for address", func() {
			actor.changePayee(rt, tutil.NewIDAddr(t, 200))
		})
	})

	t.Run("new payee must be a principle", func(t *testing.T) {
		rt, actor := setupFunc(governor)
		rt.SetCaller(governor, builtin.AccountActorCodeID)

		np := tutil.NewIDAddr(t, 200)
		rt.SetAddressActorType(np, builtin.StorageMinerActorCodeID)

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "payee must be a principal", func() {
			actor.changePayee(rt, np)
		})
	})

	t.Run("successful", func(t *testing.T) {
		rt, actor := setupFunc(governor)
		rt.SetCaller(governor, builtin.AccountActorCodeID)

		actor.changePayee(rt, newPayee)
	})
}

func TestApplyRewards(t *testing.T) {
	governor := tutil.NewIDAddr(t, 101)
	initialPayee := tutil.NewIDAddr(t, 102)
	newPayee := tutil.NewIDAddr(t, 103)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), builtin.KnowledgeFundActorAddr).
			WithActorType(governor, builtin.AccountActorCodeID).
			WithActorType(initialPayee, builtin.AccountActorCodeID).
			WithActorType(newPayee, builtin.AccountActorCodeID)
		rt := builder.Build(t)

		actor := newHarness(t, governor, initialPayee)
		actor.constructAndVerify(rt)

		return rt, actor
	}

	t.Run("invalid caller", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.ExpectValidateCallerAddr(builtin.RewardActorAddr)

		rt.SetCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
		rt.ExpectAbort(exitcode.SysErrForbidden, func() {
			rt.Call(actor.ApplyRewards, nil)
		})
	})

	t.Run("non positive funds", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(builtin.RewardActorAddr, builtin.RewardActorCodeID)

		// failed if negative
		rt.SetReceived(big.NewInt(-1))
		rt.SetBalance(big.Zero())
		rt.ExpectValidateCallerAddr(builtin.RewardActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "non positive funds to apply", func() {
			actor.applyRewards(rt, big.NewInt(-1), false, false)
		})

		// failed if zero
		rt.SetReceived(big.Zero())
		rt.SetBalance(big.Zero())
		rt.ExpectValidateCallerAddr(builtin.RewardActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "non positive funds to apply", func() {
			actor.applyRewards(rt, big.Zero(), false, false)
		})
	})

	t.Run("positive funds", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(builtin.RewardActorAddr, builtin.RewardActorCodeID)

		// total 100
		rt.SetReceived(big.NewInt(100))
		rt.SetBalance(big.NewInt(100))
		rt.ExpectValidateCallerAddr(builtin.RewardActorAddr)
		rt.ExpectSend(initialPayee, builtin.MethodSend, nil, big.NewInt(100), nil, exitcode.Ok)
		actor.applyRewards(rt, big.NewInt(100), false, true)

		// total 1000
		rt.SetReceived(big.NewInt(1000))
		rt.SetBalance(big.NewInt(1000))
		rt.ExpectValidateCallerAddr(builtin.RewardActorAddr)
		rt.ExpectSend(initialPayee, builtin.MethodSend, nil, big.NewInt(1000), nil, exitcode.Ok)
		actor.applyRewards(rt, big.NewInt(1000), true, true)
	})
}

type actorHarness struct {
	knowledge.Actor
	t *testing.T

	initialPayee address.Address
	governor     address.Address
}

func newHarness(t *testing.T, governor, payee address.Address) *actorHarness {
	assert.NotEqual(t, payee, address.Undef)
	assert.NotEqual(t, governor, address.Undef)
	return &actorHarness{
		Actor:        knowledge.Actor{},
		t:            t,
		initialPayee: payee,
		governor:     governor,
	}
}

func (h *actorHarness) constructAndVerify(rt *mock.Runtime) {
	rt.SetCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	ret := rt.Call(h.Actor.Constructor, &h.initialPayee)
	assert.Nil(h.t, ret)

	rt.Verify()

	var st knowledge.State
	rt.GetState(&st)

	assert.Equal(h.t, st.Payee, h.initialPayee)
	verifyEmptyMap(h.t, rt, st.Tally)
}

func (h *actorHarness) changePayee(rt *mock.Runtime, newPayee address.Address) {
	rt.Call(h.Actor.ChangePayee, &newPayee)
	rt.Verify()

	st := getState(rt)
	require.Equal(h.t, newPayee, st.Payee)
}

func (h *actorHarness) applyRewards(rt *mock.Runtime, rewards abi.TokenAmount, expectedFoundBefore, expectedFoundAfter bool) {

	old, found := getPayeeTotal(h.t, rt)
	require.True(h.t, expectedFoundBefore == found && old.GreaterThanEqual(big.Zero()))

	if rewards.GreaterThan(big.Zero()) {
		require.Equal(h.t, rewards, rt.Balance())
	} else {
		require.Equal(h.t, big.Zero(), rt.Balance())
	}

	rt.Call(h.Actor.ApplyRewards, nil)
	rt.Verify()

	new, found := getPayeeTotal(h.t, rt)
	require.True(h.t, expectedFoundAfter == found)

	expectedPayeeTotal := big.Add(old, rewards)
	require.Equal(h.t, expectedPayeeTotal, new)
	require.True(h.t, expectedPayeeTotal.GreaterThanEqual(big.Zero()))

	require.True(h.t, rt.Balance().Equals(big.Zero()))
}

func getPayeeTotal(t *testing.T, rt *mock.Runtime) (amt abi.TokenAmount, found bool) {
	st := getState(rt)
	tally, err := adt.AsMap(adt.AsStore(rt), st.Tally)
	require.NoError(t, err)

	amt = big.Zero()
	found, err = tally.Get(abi.AddrKey(st.Payee), &amt)
	require.NoError(t, err)
	return
}

func verifyEmptyMap(t *testing.T, rt *mock.Runtime, cid cid.Cid) {
	mapChecked, err := adt.AsMap(adt.AsStore(rt), cid)
	assert.NoError(t, err)
	keys, err := mapChecked.CollectKeys()
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func getState(rt *mock.Runtime) *knowledge.State {
	var st knowledge.State
	rt.GetState(&st)
	return &st
}
