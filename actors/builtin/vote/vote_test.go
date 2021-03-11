package vote_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/vote"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/filecoin-project/specs-actors/v2/support/mock"
	tutil "github.com/filecoin-project/specs-actors/v2/support/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, vote.Actor{})
}

func TestConstruction(t *testing.T) {

	actor := vote.Actor{}
	builder := mock.NewBuilder(context.Background(), builtin.VoteFundActorAddr).
		WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("fallback not set", func(t *testing.T) {
		rt := builder.Build(t)
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "fallback not a ID-Address", func() {
			rt.Call(actor.Constructor, &address.Undef)
		})
	})

	t.Run("simple construction", func(t *testing.T) {

		fb := tutil.NewIDAddr(t, 101)

		rt := builder.Build(t)
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		ret := rt.Call(actor.Constructor, &fb)
		assert.Nil(t, ret)
		rt.Verify()
	})
}

func TestVote(t *testing.T) {
	caller := tutil.NewIDAddr(t, 100)
	fallback := tutil.NewIDAddr(t, 101)
	candidate1 := tutil.NewIDAddr(t, 102)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), builtin.VoteFundActorAddr).
			WithActorType(fallback, builtin.AccountActorCodeID)
		rt := builder.Build(t)

		actor := newHarness(t, fallback)
		actor.constructAndVerify(rt)

		return rt, actor
	}

	t.Run("not signable caller", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(builtin.RewardActorAddr, builtin.RewardActorCodeID)

		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectAbort(exitcode.SysErrForbidden, func() {
			rt.Call(actor.Vote, &candidate1)
		})
	})

	t.Run("non positive votes", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(caller, builtin.AccountActorCodeID)

		// failed if negative
		rt.SetReceived(big.NewInt(-1))
		rt.SetBalance(big.Zero())
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "non positive votes to vote", func() {
			rt.Call(actor.Vote, &candidate1)
		})

		// failed if zero
		rt.SetReceived(big.Zero())
		rt.SetBalance(big.Zero())
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "non positive votes to vote", func() {
			rt.Call(actor.Vote, &candidate1)
		})
	})

	t.Run("non positive votes", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(caller, builtin.AccountActorCodeID)

		rt.SetReceived(big.NewInt(1))
		rt.SetBalance(big.Zero())
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "non positive votes to vote", func() {
			rt.Call(actor.Vote, &candidate1)
		})
	})
}

type actorHarness struct {
	vote.Actor
	t *testing.T

	fallback address.Address
}

func newHarness(t *testing.T, fallback address.Address) *actorHarness {
	assert.NotEqual(t, fallback, address.Undef)
	return &actorHarness{
		Actor:    vote.Actor{},
		t:        t,
		fallback: fallback,
	}
}

func (h *actorHarness) constructAndVerify(rt *mock.Runtime) {
	rt.SetCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	ret := rt.Call(h.Actor.Constructor, &h.fallback)
	assert.Nil(h.t, ret)

	rt.Verify()

	var st vote.State
	rt.GetState(&st)

	assert.Equal(h.t, st.FallbackReceiver, h.fallback)

	candiates, err := adt.AsMap(adt.AsStore(rt), st.Candidates, builtin.DefaultHamtBitwidth)
	assert.NoError(h.t, err)
	keys, err := candiates.CollectKeys()
	require.NoError(h.t, err)
	assert.Empty(h.t, keys)

	votes, err := adt.AsMap(adt.AsStore(rt), st.Voters, builtin.DefaultHamtBitwidth)
	assert.NoError(h.t, err)
	keys, err = votes.CollectKeys()
	require.NoError(h.t, err)
	assert.Empty(h.t, keys)
}
