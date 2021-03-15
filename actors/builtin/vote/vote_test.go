package vote_test

import (
	"context"
	"strings"
	"testing"

	"github.com/filecoin-project/go-address"
	abi "github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
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

	t.Run("fail when caller is non-signable", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(builtin.RewardActorAddr, builtin.RewardActorCodeID)

		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectAbort(exitcode.SysErrForbidden, func() {
			rt.Call(actor.Vote, &candidate1)
		})
	})

	t.Run("fail when non positive votes", func(t *testing.T) {
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

	t.Run("fail when voting not allowed", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(caller, builtin.AccountActorCodeID)

		rt.SetReceived(big.NewInt(1))
		rt.SetBalance(big.Zero())
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(candidate1, builtin.MethodsExpert.CheckState, nil, big.Zero(), &expert.CheckStateReturn{AllowVote: false}, exitcode.Ok)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "vote not allowed", func() {
			rt.Call(actor.Vote, &candidate1)
		})
	})

	t.Run("multi voters for one candidate", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetBalance(big.Zero())

		sum := actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 0 && sum.VotersCount == 0)

		// first voter
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.SetReceived(big.NewInt(1000))
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(candidate1, builtin.MethodsExpert.CheckState, nil, big.Zero(), &expert.CheckStateReturn{AllowVote: true}, exitcode.Ok)

		rt.Call(actor.Vote, &candidate1)
		rt.Verify()

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 1)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1000)))

		// second voter
		caller2 := tutil.NewIDAddr(t, 200)
		rt.SetCaller(caller2, builtin.MultisigActorCodeID)
		rt.SetReceived(big.NewInt(300))
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(candidate1, builtin.MethodsExpert.CheckState, nil, big.Zero(), &expert.CheckStateReturn{AllowVote: true}, exitcode.Ok)

		rt.Call(actor.Vote, &candidate1)
		rt.Verify()

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 2)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1300)))
	})

	t.Run("one voters for multi candidates", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetBalance(big.Zero())

		sum := actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 0 && sum.VotersCount == 0)

		// first
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.SetReceived(big.NewInt(1000))
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(candidate1, builtin.MethodsExpert.CheckState, nil, big.Zero(), &expert.CheckStateReturn{AllowVote: true}, exitcode.Ok)

		rt.Call(actor.Vote, &candidate1)
		rt.Verify()

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 1)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.VoterTallyCount[caller] == 1)

		// second
		candidate2 := tutil.NewIDAddr(t, 200)
		rt.SetReceived(big.NewInt(300))
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(candidate2, builtin.MethodsExpert.CheckState, nil, big.Zero(), &expert.CheckStateReturn{AllowVote: true}, exitcode.Ok)

		rt.Call(actor.Vote, &candidate2)
		rt.Verify()

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 2 && sum.VotersCount == 1)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1300)))
		require.True(t, sum.VoterTallyCount[caller] == 2)
	})

	t.Run("vote for one candidate more than one times", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetBalance(big.Zero())

		sum := actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 0 && sum.VotersCount == 0)

		// first
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.SetReceived(big.NewInt(1000))
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(candidate1, builtin.MethodsExpert.CheckState, nil, big.Zero(), &expert.CheckStateReturn{AllowVote: true}, exitcode.Ok)

		rt.Call(actor.Vote, &candidate1)
		rt.Verify()

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 1)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.VoterTallyCount[caller] == 1)

		// second
		rt.SetReceived(big.NewInt(300))
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(candidate1, builtin.MethodsExpert.CheckState, nil, big.Zero(), &expert.CheckStateReturn{AllowVote: true}, exitcode.Ok)

		rt.Call(actor.Vote, &candidate1)
		rt.Verify()

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 1)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1300)))
		require.True(t, sum.VoterTallyCount[caller] == 1)
	})
}

func TestGetCandidates(t *testing.T) {
	caller := tutil.NewIDAddr(t, 100)
	fallback := tutil.NewIDAddr(t, 101)
	candidate1 := tutil.NewIDAddr(t, 102)
	candidate2 := tutil.NewIDAddr(t, 103)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), builtin.VoteFundActorAddr).
			WithActorType(fallback, builtin.AccountActorCodeID)
		rt := builder.Build(t)

		actor := newHarness(t, fallback)
		actor.constructAndVerify(rt)

		return rt, actor
	}

	t.Run("empty params", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetBalance(big.Zero())

		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAny()
		params := vote.GetCandidatesParams{}
		ret := rt.Call(actor.GetCandidates, &params)

		retV := ret.(*vote.GetCandidatesReturn)
		require.Zero(t, len(retV.Votes))
	})

	t.Run("fail when pass a non-ID address", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetBalance(big.Zero())

		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAny()
		params := vote.GetCandidatesParams{
			Addresses: []address.Address{tutil.NewBLSAddr(t, 1)},
		}
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "ID address required", func() {
			rt.Call(actor.GetCandidates, &params)
		})
	})

	t.Run("fail when candidate not found", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetBalance(big.Zero())

		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAny()
		params := vote.GetCandidatesParams{
			Addresses: []address.Address{candidate1},
		}
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "candidate not found", func() {
			rt.Call(actor.GetCandidates, &params)
		})
	})

	t.Run("candidate not exists", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.vote(rt, caller, candidate1, big.NewInt(1000))
		actor.vote(rt, caller, candidate2, big.NewInt(500))

		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAny()
		params := vote.GetCandidatesParams{
			Addresses: []address.Address{candidate1, candidate2},
		}

		v := rt.Call(actor.GetCandidates, &params)
		ret := v.(*vote.GetCandidatesReturn)
		require.True(t, len(ret.Votes) == 2 &&
			ret.Votes[0].Equals(big.NewInt(1000)) &&
			ret.Votes[1].Equals(big.NewInt(500)))
	})
}

func TestOnCandidateBlocked(t *testing.T) {
	caller := tutil.NewIDAddr(t, 100)
	fallback := tutil.NewIDAddr(t, 101)
	candidate1 := tutil.NewIDAddr(t, 102)
	candidate2 := tutil.NewIDAddr(t, 103)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), builtin.VoteFundActorAddr).
			WithActorType(fallback, builtin.AccountActorCodeID)
		rt := builder.Build(t)

		actor := newHarness(t, fallback)
		actor.constructAndVerify(rt)

		return rt, actor
	}

	t.Run("fail when candidate not found", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetBalance(big.Zero())

		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		rt.SetCaller(candidate2, builtin.ExpertActorCodeID)
		rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "candidate not found", func() {
			rt.Call(actor.OnCandidateBlocked, nil)
		})
	})

	t.Run("one voter and block candidates", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetBalance(big.Zero())

		// vote for candidate1 & candidate2
		actor.vote(rt, caller, candidate1, big.NewInt(1000))
		actor.vote(rt, caller, candidate2, big.NewInt(500))

		sum := actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1500)))
		var st vote.State
		rt.GetState(&st)
		require.True(t, st.TotalVotes.Equals(big.NewInt(1500)))

		// block candidate1
		rt.SetEpoch(99)
		rt.SetCaller(candidate1, builtin.ExpertActorCodeID)
		rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
		rt.Call(actor.OnCandidateBlocked, nil)

		sum = actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(500)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.BlockedAt[candidate1] == 99)
		rt.GetState(&st)
		require.True(t, st.TotalVotes.Equals(big.NewInt(500)))

		// block candidate2
		rt.SetEpoch(100)
		rt.SetCaller(candidate2, builtin.ExpertActorCodeID)
		rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
		rt.Call(actor.OnCandidateBlocked, nil)

		sum = actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(0)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(1500)))
		require.True(t, sum.BlockedAt[candidate1] == 99)
		require.True(t, sum.BlockedAt[candidate2] == 100)
		require.True(t, sum.VoterTallyCount[caller] == 2)
		rt.GetState(&st)
		require.True(t, st.TotalVotes.Equals(big.NewInt(0)))
	})
}

func TestRescind(t *testing.T) {
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

	t.Run("fail when non-positive votes", func(t *testing.T) {
		rt, actor := setupFunc()

		params := vote.RescindParams{
			Candidate: candidate1,
			Votes:     big.NewInt(-1),
		}
		// negative votes
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "non positive votes to rescind", func() {
			rt.Call(actor.Rescind, &params)
		})
		// zero votes
		params.Votes = big.Zero()
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "non positive votes to rescind", func() {
			rt.Call(actor.Rescind, &params)
		})
	})

	t.Run("fail when voter has not voted", func(t *testing.T) {
		rt, actor := setupFunc()

		params := vote.RescindParams{
			Candidate: candidate1,
			Votes:     big.NewInt(1),
		}
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "voter not found", func() {
			rt.Call(actor.Rescind, &params)
		})
	})

	t.Run("rescind normally", func(t *testing.T) {
		rt, actor := setupFunc()

		// vote for candidate1
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		params := vote.RescindParams{
			Candidate: candidate1,
			Votes:     big.NewInt(1),
		}
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.Call(actor.Rescind, &params)

		var st vote.State
		rt.GetState(&st)

		sum := actor.checkState(rt)
		require.True(t, sum.TotalRescindingVotes.Equals(big.NewInt(1)))
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(999)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(0)))
		require.True(t, st.TotalVotes.Equals(big.NewInt(999)))
		require.True(t, st.CumEarningsPerVote.IsZero())
	})
}

func TestApplyRewards(t *testing.T) {
	fallback := tutil.NewIDAddr(t, 101)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), builtin.VoteFundActorAddr).
			WithActorType(fallback, builtin.AccountActorCodeID)
		rt := builder.Build(t)

		actor := newHarness(t, fallback)
		actor.constructAndVerify(rt)

		return rt, actor
	}

	t.Run("fail when apply negative funds", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetReceived(big.NewInt(-1))
		rt.SetCaller(builtin.RewardActorAddr, builtin.RewardActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.RewardActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "negative amount to apply", func() {
			rt.Call(actor.ApplyRewards, nil)
		})
	})

	t.Run("apply funds", func(t *testing.T) {
		rt, actor := setupFunc()

		var st vote.State
		rt.GetState(&st)
		require.True(t, st.UnownedFunds.IsZero())

		// apply zero
		rt.SetReceived(big.NewInt(0))
		rt.SetCaller(builtin.RewardActorAddr, builtin.RewardActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.RewardActorAddr)
		rt.Call(actor.ApplyRewards, nil)
		rt.Verify()

		rt.GetState(&st)
		require.True(t, st.UnownedFunds.IsZero())

		// apply positive
		rt.SetReceived(big.NewInt(10))
		rt.SetCaller(builtin.RewardActorAddr, builtin.RewardActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.RewardActorAddr)
		rt.Call(actor.ApplyRewards, nil)
		rt.Verify()

		rt.GetState(&st)
		require.True(t, st.UnownedFunds.Equals(big.NewInt(10)))
	})
}

func TestWithdraw(t *testing.T) {
	caller := tutil.NewIDAddr(t, 100)
	fallback := tutil.NewIDAddr(t, 101)
	candidate1 := tutil.NewIDAddr(t, 102)
	exptf := tutil.NewIDAddr(t, 103)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), builtin.VoteFundActorAddr).
			WithActorType(exptf, builtin.ExpertFundActorCodeID).
			WithActorType(fallback, builtin.AccountActorCodeID)
		rt := builder.Build(t)

		actor := newHarness(t, fallback)
		actor.constructAndVerify(rt)

		return rt, actor
	}

	t.Run("fail when recipient is not principle", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "recipient is not principle", func() {
			rt.Call(actor.Withdraw, &exptf)
		})
	})

	t.Run("fail when voter not found", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "voter not found", func() {
			rt.Call(actor.Withdraw, &caller)
		})
	})

	t.Run("no votes and rewards", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.vote(rt, caller, candidate1, big.NewInt(10))

		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		v := rt.Call(actor.Withdraw, &caller)
		amt := v.(*abi.TokenAmount)
		require.True(t, amt.IsZero())
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

func (h *actorHarness) vote(rt *mock.Runtime, voter, candidate address.Address, votes abi.TokenAmount) {
	rt.SetReceived(votes)
	rt.SetCaller(voter, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
	rt.ExpectSend(candidate, builtin.MethodsExpert.CheckState, nil, big.Zero(), &expert.CheckStateReturn{AllowVote: true}, exitcode.Ok)
	rt.Call(h.Vote, &candidate)
	rt.Verify()
}

func (h *actorHarness) blockCandidate(rt *mock.Runtime, candidate address.Address) {
	rt.SetCaller(candidate, builtin.ExpertActorCodeID)
	rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
	rt.Call(h.OnCandidateBlocked, nil)
	rt.Verify()
}

func (h *actorHarness) cronTick(rt *mock.Runtime) {
	rt.ExpectValidateCallerAddr(builtin.CronActorAddr)
	rt.SetCaller(builtin.CronActorAddr, builtin.CronActorCodeID)
	param := abi.EmptyValue{}

	rt.Call(h.OnEpochTickEnd, &param)
	rt.Verify()
}

func (h *actorHarness) checkState(rt *mock.Runtime) *vote.StateSummary {
	var st vote.State
	rt.GetState(&st)
	sum, msgs := vote.CheckStateInvariants(&st, rt.AdtStore())
	require.True(h.t, msgs.IsEmpty(), strings.Join(msgs.Messages(), "\n"))
	return sum
}

func (h *actorHarness) getCandidates(rt *mock.Runtime, candidates ...address.Address) *vote.GetCandidatesReturn {
	rt.ExpectValidateCallerAny()
	params := vote.GetCandidatesParams{
		Addresses: candidates,
	}
	v := rt.Call(h.GetCandidates, &params)
	ret := v.(*vote.GetCandidatesReturn)
	require.True(h.t, len(ret.Votes) == len(candidates))
	rt.Verify()
	return ret
}

func (h *actorHarness) applyRewards(rt *mock.Runtime, amount abi.TokenAmount) {
	var st vote.State
	rt.GetState(&st)
	before := st.UnownedFunds

	rt.SetReceived(amount)
	rt.SetCaller(builtin.RewardActorAddr, builtin.RewardActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.RewardActorAddr)
	rt.Call(h.ApplyRewards, nil)
	rt.Verify()

	rt.GetState(&st)
	require.True(h.t, big.Sub(st.UnownedFunds, before).Equals(amount))
}

func (h *actorHarness) withdraw(rt *mock.Runtime, voter, recipient address.Address) *abi.TokenAmount {
	rt.SetCaller(voter, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
	v := rt.Call(h.Withdraw, &voter)
	return v.(*abi.TokenAmount)
}
