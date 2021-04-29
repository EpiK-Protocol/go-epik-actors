package vote_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/filecoin-project/go-address"
	abi "github.com/filecoin-project/go-state-types/abi"
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

func getState(rt *mock.Runtime) *vote.State {
	var st vote.State
	rt.GetState(&st)
	return &st
}

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

		rt.SetReceived(big.NewInt(1))
		rt.SetBalance(big.Zero())

		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.SetAddressActorType(candidate1, builtin.ExpertActorCodeID)
		rt.ExpectSend(candidate1, builtin.MethodsExpert.CheckState, nil, big.Zero(), &builtin.CheckExpertStateReturn{AllowVote: false}, exitcode.Ok)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "vote not allowed", func() {
			rt.Call(actor.Vote, &candidate1)
		})
	})

	t.Run("multi voters for one candidate in same epoch", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetBalance(big.Zero())

		sum := actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 0 && sum.VotersCount == 0)

		// first voter
		rt.SetEpoch(100)
		rt.SetBalance(big.NewInt(1100)) // 1000 votes and 100 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(1000))
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(100).withCurrEpochEffectiveVotes(1000).
			withTotalVotes(1000).withCurrEpochRewards(100).withLastRewardBalance(100))

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 1)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1000)))

		// second voter
		caller2 := tutil.NewIDAddr(t, 200)
		rt.SetBalance(big.NewInt(1550)) // +300 votes and 150 rewards
		actor.vote(rt, caller2, candidate1, big.NewInt(300))
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(100).withCurrEpochEffectiveVotes(1300).
			withTotalVotes(1300).withCurrEpochRewards(250).withLastRewardBalance(250))
		actor.expectWithdrawable(rt, caller, 0)
		actor.expectWithdrawable(rt, caller2, 0)

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 2)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1300)))
	})

	t.Run("multi voters for one candidate in different epochs", func(t *testing.T) {
		rt, actor := setupFunc()

		// first voter
		rt.SetEpoch(100)
		rt.SetBalance(big.NewInt(1100)) // 1000 votes and 100 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(1000))
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(100).withCurrEpochEffectiveVotes(1000).
			withTotalVotes(1000).withLastRewardBalance(100).withCurrEpochRewards(100))

		sum := actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 1)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.VoterTallyCount[caller] == 1)

		// second voter
		rt.SetEpoch(200)
		caller2 := tutil.NewIDAddr(t, 200)
		rt.SetBalance(big.NewInt(1550)) // +300 votes and 150 rewards
		actor.vote(rt, caller2, candidate1, big.NewInt(300))
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(200).withCurrEpochEffectiveVotes(1300).
			withPrevEpoch(100).withCurrEpochRewards(150).withPrevEpochEarningsPerVote(1e11).
			withTotalVotes(1300).withLastRewardBalance(250))
		actor.expectWithdrawable(rt, caller, 100)
		actor.expectWithdrawable(rt, caller2, 0)

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 2)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1300)))
	})

	t.Run("one voters for multi candidates", func(t *testing.T) {
		rt, actor := setupFunc()
		// first
		rt.SetEpoch(100)
		rt.SetBalance(big.NewInt(1100)) // 1000 votes and 100 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		// second
		rt.SetEpoch(200)
		rt.SetBalance(big.NewInt(1550)) // +300 votes and 150 rewards
		candidate2 := tutil.NewIDAddr(t, 200)
		actor.vote(rt, caller, candidate2, big.NewInt(300))
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(200).withCurrEpochEffectiveVotes(1300).
			withPrevEpoch(100).withCurrEpochRewards(150).withPrevEpochEarningsPerVote(1e11).
			withTotalVotes(1300).withLastRewardBalance(250))
		actor.expectWithdrawable(rt, caller, 100)

		sum := actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 2 && sum.VotersCount == 1)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1300)))
		require.True(t, sum.VoterTallyCount[caller] == 2)
	})

	t.Run("vote for one candidate more than one times", func(t *testing.T) {
		rt, actor := setupFunc()

		// first
		rt.SetEpoch(100)
		rt.SetBalance(big.NewInt(1100)) // 1000 votes and 100 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		// second
		rt.SetEpoch(200)
		rt.SetBalance(big.NewInt(1530)) // +300 votes and 130 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(300))
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(200).withCurrEpochEffectiveVotes(1300).
			withPrevEpoch(100).withCurrEpochRewards(130).withPrevEpochEarningsPerVote(1e11).
			withTotalVotes(1300).withLastRewardBalance(230))
		actor.expectWithdrawable(rt, caller, 100)

		// third, in the same epoch
		rt.SetBalance(big.NewInt(1780)) // +100 votes and 150 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(100))
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(200).withCurrEpochEffectiveVotes(1400).
			withPrevEpoch(100).withCurrEpochRewards(280).withPrevEpochEarningsPerVote(1e11).
			withTotalVotes(1400).withLastRewardBalance(380))
		actor.expectWithdrawable(rt, caller, 100)

		sum := actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 1)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1400)))
		require.True(t, sum.VoterTallyCount[caller] == 1)

		// voter 2
		rt.SetEpoch(300)
		caller2 := tutil.NewIDAddr(t, 200)
		rt.SetBalance(big.NewInt(1980)) // +100 votes and 100 rewards
		actor.vote(rt, caller2, candidate1, big.NewInt(100))
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(300).withCurrEpochEffectiveVotes(1500).
			withPrevEpoch(200).withCurrEpochRewards(100).withPrevEpochEarningsPerVote(3e11).
			withTotalVotes(1500).withLastRewardBalance(480))
		actor.expectWithdrawable(rt, caller, 380)
		actor.expectWithdrawable(rt, caller2, 0)

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 2)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1500)))
		require.True(t, sum.VoterTallyCount[caller] == 1)
		require.True(t, sum.VoterTallyCount[caller2] == 1)
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

		rt.SetBalance(abi.NewTokenAmount(1000))
		actor.vote(rt, caller, candidate1, big.NewInt(1000))
		rt.SetBalance(abi.NewTokenAmount(1500))
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

		rt.SetBalance(abi.NewTokenAmount(1000))
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "candidate not found", func() {
			actor.onCandidateBlocked(rt, candidate2)
		})
	})

	t.Run("one voter, multi candidates without reward", func(t *testing.T) {
		rt, actor := setupFunc()

		// vote for candidate1 & candidate2
		rt.SetEpoch(10)
		rt.SetBalance(abi.NewTokenAmount(1000))
		actor.vote(rt, caller, candidate1, big.NewInt(1000))
		rt.SetBalance(abi.NewTokenAmount(1500))
		actor.vote(rt, caller, candidate2, big.NewInt(500))

		sum := actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1500)))
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(10).withTotalVotes(1500).withCurrEpochEffectiveVotes(1500))

		// block candidate1
		rt.SetEpoch(99)
		actor.onCandidateBlocked(rt, candidate1)

		sum = actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(500)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.BlockedAt[candidate1] == 99)
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(99).withTotalVotes(1500).withCurrEpochEffectiveVotes(500).withPrevEpoch(10))

		// block candidate2
		rt.SetEpoch(100)
		actor.onCandidateBlocked(rt, candidate2)

		sum = actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(0)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(1500)))
		require.True(t, sum.BlockedAt[candidate1] == 99)
		require.True(t, sum.BlockedAt[candidate2] == 100)
		require.True(t, sum.VoterTallyCount[caller] == 2)
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(100).withTotalVotes(1500).withPrevEpoch(99))
	})

	t.Run("block without new reward", func(t *testing.T) {
		rt, actor := setupFunc()

		// vote
		rt.SetEpoch(10)
		rt.SetBalance(abi.NewTokenAmount(1100)) // 1000 votes, 100 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(1000))
		sum := actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1000)))
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(10).withTotalVotes(1000).withCurrEpochEffectiveVotes(1000).
			withCurrEpochRewards(100).withLastRewardBalance(100))
		actor.expectWithdrawable(rt, caller, 0)

		// block
		rt.SetEpoch(20)
		actor.onCandidateBlocked(rt, candidate1)
		sum = actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(0)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.BlockedAt[candidate1] == 20)
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(20).withTotalVotes(1000).withPrevEpoch(10).withLastRewardBalance(100).withPrevEpochEarningsPerVote(1e11))
		actor.expectWithdrawable(rt, caller, 100)
	})

	t.Run("block with new reward", func(t *testing.T) {
		rt, actor := setupFunc()

		// vote
		rt.SetEpoch(10)
		rt.SetBalance(abi.NewTokenAmount(1100)) // 1000 votes, 100 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(1000))
		sum := actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1000)))
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(10).withTotalVotes(1000).withCurrEpochEffectiveVotes(1000).
			withCurrEpochRewards(100).withLastRewardBalance(100))
		actor.expectWithdrawable(rt, caller, 0)

		// block
		rt.SetEpoch(20)
		rt.SetBalance(abi.NewTokenAmount(1120)) // 20 rewards
		actor.onCandidateBlocked(rt, candidate1)
		sum = actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(0)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.BlockedAt[candidate1] == 20)
		require.True(t, sum.VoterTallyCount[caller] == 1)
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(20).withTotalVotes(1000).withCurrEpochRewards(20).
			withPrevEpoch(10).withLastRewardBalance(120).withPrevEpochEarningsPerVote(1e11))
		actor.expectWithdrawable(rt, caller, 100)

		// re-block
		rt.SetEpoch(30)
		actor.onCandidateBlocked(rt, candidate1)
		sum = actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(0)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.BlockedAt[candidate1] == 20)
		require.True(t, sum.VoterTallyCount[caller] == 1)
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(30).withTotalVotes(1000).withFallbackDebt(20).
			withPrevEpoch(20).withLastRewardBalance(120).withPrevEpochEarningsPerVote(1e11))
		actor.expectWithdrawable(rt, caller, 100)
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
		// negative votes
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "non positive votes to rescind", func() {
			actor.rescind(rt, caller, candidate1, big.NewInt(-1))
		})
		// zero votes
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "non positive votes to rescind", func() {
			actor.rescind(rt, caller, candidate1, big.NewInt(0))
		})
	})

	t.Run("fail when voter has not voted", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "voter not found", func() {
			actor.rescind(rt, caller, candidate1, big.NewInt(1))
		})
	})

	t.Run("rescind", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetBalance(abi.NewTokenAmount(1000))
		actor.vote(rt, caller, candidate1, big.NewInt(1000))
		actor.rescind(rt, caller, candidate1, big.NewInt(1))

		actor.expectPool(rt, newExpectPoolInfo().withTotalVotes(1000).withCurrEpochEffectiveVotes(999))

		sum := actor.checkState(rt)
		require.True(t, sum.TotalRescindingVotes.Equals(big.NewInt(1)))
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(999)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(0)))
	})

	t.Run("withdraw after delay passed", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetEpoch(100)
		rt.SetBalance(big.NewInt(1000))
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		rt.SetEpoch(101)
		actor.rescind(rt, caller, candidate1, big.NewInt(1))
		actor.expectCandidate(rt, candidate1, true, 999)

		st := getState(rt)
		list, err := st.ListVotesInfo(adt.AsStore(rt), caller)
		require.NoError(t, err)
		require.True(t, len(list) == 1)
		require.True(t, list[candidate1].RescindingVotes.Equals(abi.NewTokenAmount(1)))
		require.True(t, list[candidate1].Votes.Equals(abi.NewTokenAmount(999)))
		require.True(t, list[candidate1].LastRescindEpoch == 101)

		sum := actor.checkState(rt)
		require.True(t, sum.TotalRescindingVotes.Equals(big.NewInt(1)))
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(999)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(0)))

		// last epoch that not withdrawable
		rt.SetEpoch(101 + vote.RescindingUnlockDelay)
		actor.withdraw(rt, caller, big.Zero(), big.Zero())
		require.True(t, rt.Balance().Equals(big.NewInt(1000)))

		rt.SetEpoch(102 + vote.RescindingUnlockDelay)
		actor.withdraw(rt, caller, abi.NewTokenAmount(1), big.Zero())
		require.True(t, rt.Balance().Equals(big.NewInt(999)))
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

	t.Run("no funds sent when voter not found", func(t *testing.T) {
		rt, actor := setupFunc()
		actor.withdraw(rt, caller, big.Zero(), big.Zero())
	})

	t.Run("no votes and rewards", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetBalance(abi.NewTokenAmount(10))
		actor.vote(rt, caller, candidate1, big.NewInt(10))

		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		v := rt.Call(actor.Withdraw, nil)
		amt := v.(*abi.TokenAmount)
		require.True(t, amt.IsZero())
	})

	t.Run("withdraw after re-block", func(t *testing.T) {
		rt, actor := setupFunc()

		// vote
		rt.SetEpoch(10)
		rt.SetBalance(abi.NewTokenAmount(1100)) // 1000 votes, 100 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		// block
		rt.SetEpoch(20)
		rt.SetBalance(abi.NewTokenAmount(1120)) // 20 rewards
		actor.onCandidateBlocked(rt, candidate1)

		// re-block
		rt.SetEpoch(30)
		actor.onCandidateBlocked(rt, candidate1)

		actor.withdraw(rt, caller, abi.NewTokenAmount(100), abi.NewTokenAmount(20))
		actor.expectWithdrawable(rt, caller, 0)
		sum := actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(0)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.BlockedAt[candidate1] == 20)
		require.True(t, sum.VoterTallyCount[caller] == 1)
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(30).withTotalVotes(1000).withFallbackDebt(0).
			withPrevEpoch(20).withLastRewardBalance(0).withPrevEpochEarningsPerVote(1e11))
	})

	t.Run("withdraw after block in same epoch", func(t *testing.T) {
		rt, actor := setupFunc()

		// vote
		rt.SetEpoch(10)
		rt.SetBalance(abi.NewTokenAmount(1100)) // 1000 votes, 100 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		// block
		rt.SetEpoch(20)
		rt.SetBalance(abi.NewTokenAmount(1120)) // 20 rewards
		actor.onCandidateBlocked(rt, candidate1)

		actor.withdraw(rt, caller, abi.NewTokenAmount(100), abi.NewTokenAmount(0))
		actor.expectWithdrawable(rt, caller, 0)
		sum := actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(0)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.BlockedAt[candidate1] == 20)
		require.True(t, sum.VoterTallyCount[caller] == 1)
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(20).withTotalVotes(1000).withCurrEpochRewards(20).
			withPrevEpoch(10).withLastRewardBalance(20).withPrevEpochEarningsPerVote(1e11))
	})

	t.Run("withdraw after block in different epochs", func(t *testing.T) {
		rt, actor := setupFunc()

		// vote
		rt.SetEpoch(10)
		rt.SetBalance(abi.NewTokenAmount(1100)) // 1000 votes, 100 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		// block
		rt.SetEpoch(20)
		rt.SetBalance(abi.NewTokenAmount(1120)) // 20 rewards
		actor.onCandidateBlocked(rt, candidate1)

		rt.SetEpoch(40)
		actor.withdraw(rt, caller, abi.NewTokenAmount(100), abi.NewTokenAmount(20))
		actor.expectWithdrawable(rt, caller, 0)
		sum := actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(0)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.BlockedAt[candidate1] == 20)
		require.True(t, sum.VoterTallyCount[caller] == 1)
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(40).withTotalVotes(1000).withFallbackDebt(0).
			withPrevEpoch(20).withLastRewardBalance(0).withPrevEpochEarningsPerVote(1e11))
	})

	t.Run("withdraw after vote in same epoch", func(t *testing.T) {
		rt, actor := setupFunc()

		// vote
		rt.SetEpoch(10)
		rt.SetBalance(abi.NewTokenAmount(1100)) // 1000 votes, 100 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		// withdraw
		actor.withdraw(rt, caller, abi.NewTokenAmount(0), abi.NewTokenAmount(0))
		actor.expectWithdrawable(rt, caller, 0)
		sum := actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(0)))
		require.True(t, len(sum.BlockedAt) == 0)
		require.True(t, sum.VoterTallyCount[caller] == 1)
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(10).withTotalVotes(1000).withCurrEpochEffectiveVotes(1000).
			withLastRewardBalance(100).withCurrEpochRewards(100))
	})

	t.Run("withdraw after vote in different epochs", func(t *testing.T) {
		rt, actor := setupFunc()

		// vote
		rt.SetEpoch(10)
		rt.SetBalance(abi.NewTokenAmount(1100)) // 1000 votes, 100 rewards
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		// withdraw
		rt.SetEpoch(20)
		rt.SetBalance(abi.NewTokenAmount(1120)) // 20 rewards
		actor.withdraw(rt, caller, abi.NewTokenAmount(100), abi.NewTokenAmount(0))
		actor.expectWithdrawable(rt, caller, 0)
		sum := actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(0)))
		require.True(t, len(sum.BlockedAt) == 0)
		require.True(t, sum.VoterTallyCount[caller] == 1)
		actor.expectPool(rt, newExpectPoolInfo().withCurrEpoch(20).withTotalVotes(1000).withCurrEpochRewards(20).withCurrEpochEffectiveVotes(1000).
			withPrevEpoch(10).withLastRewardBalance(20).withPrevEpochEarningsPerVote(1e11))
	})

	t.Run("withdraw and delete voter", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetEpoch(100)
		rt.SetBalance(big.NewInt(1000))
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		rt.SetEpoch(101)
		actor.rescind(rt, caller, candidate1, big.NewInt(1000))
		actor.expectCandidate(rt, candidate1, true, 0)

		st := getState(rt)
		list, err := st.ListVotesInfo(adt.AsStore(rt), caller)
		require.NoError(t, err)
		require.True(t, len(list) == 1)
		require.True(t, list[candidate1].RescindingVotes.Equals(abi.NewTokenAmount(1000)))
		require.True(t, list[candidate1].Votes.Equals(abi.NewTokenAmount(0)))
		require.True(t, list[candidate1].LastRescindEpoch == 101)

		sum := actor.checkState(rt)
		require.True(t, sum.TotalRescindingVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(0)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(0)))

		// last epoch that not withdrawable
		rt.SetEpoch(101 + vote.RescindingUnlockDelay)
		actor.withdraw(rt, caller, big.Zero(), big.Zero())
		require.True(t, rt.Balance().Equals(big.NewInt(1000)))

		rt.SetEpoch(102 + vote.RescindingUnlockDelay)
		actor.withdraw(rt, caller, abi.NewTokenAmount(1000), big.Zero())
		require.True(t, rt.Balance().Equals(big.NewInt(0)))
		st = getState(rt)
		_, found, err := st.GetVoter(adt.AsStore(rt), caller)
		require.True(t, err == nil && !found)
	})

	t.Run("withdraw and delete one of voters", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetEpoch(100)
		rt.SetBalance(big.NewInt(1000))
		actor.vote(rt, caller, candidate1, big.NewInt(1000))
		rt.SetBalance(big.NewInt(1500))
		candidate2 := tutil.NewIDAddr(t, 200)
		actor.vote(rt, caller, candidate2, big.NewInt(500))

		rt.SetEpoch(101)
		actor.rescind(rt, caller, candidate1, big.NewInt(1000))
		actor.expectCandidate(rt, candidate1, true, 0)
		actor.expectCandidate(rt, candidate2, true, 500)

		st := getState(rt)
		list, err := st.ListVotesInfo(adt.AsStore(rt), caller)
		require.NoError(t, err)
		require.True(t, len(list) == 2)
		require.True(t, list[candidate1].RescindingVotes.Equals(abi.NewTokenAmount(1000)))
		require.True(t, list[candidate1].Votes.Equals(abi.NewTokenAmount(0)))
		require.True(t, list[candidate1].LastRescindEpoch == 101)
		require.True(t, list[candidate2].RescindingVotes.Equals(abi.NewTokenAmount(0)))
		require.True(t, list[candidate2].Votes.Equals(abi.NewTokenAmount(500)))
		require.True(t, list[candidate2].LastRescindEpoch == 0)

		sum := actor.checkState(rt)
		require.True(t, sum.TotalRescindingVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(500)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(0)))

		rt.SetEpoch(102 + vote.RescindingUnlockDelay)
		actor.withdraw(rt, caller, abi.NewTokenAmount(1000), big.Zero())
		require.True(t, rt.Balance().Equals(big.NewInt(500)))

		st = getState(rt)
		_, found, err := st.GetVoter(adt.AsStore(rt), caller)
		require.True(t, err == nil && found)
		list, err = st.ListVotesInfo(adt.AsStore(rt), caller)
		require.NoError(t, err)
		require.True(t, len(list) == 1 && list[candidate2].Votes.Equals(abi.NewTokenAmount(500)))
	})
}

func calcExpectDeltaPerVote(funds, votes abi.TokenAmount) abi.TokenAmount {
	return big.Div(big.Mul(funds, vote.Multiplier1E12), votes)
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
	st := getState(rt)
	candidates, err := adt.AsMap(adt.AsStore(rt), st.Candidates, builtin.DefaultHamtBitwidth)
	require.NoError(h.t, err)
	var out vote.Candidate
	found, err := candidates.Get(abi.AddrKey(candidate), &out)
	require.NoError(h.t, err)
	newVotes := votes
	if found {
		newVotes = big.Add(votes, out.Votes)
	}

	rt.SetReceived(votes)
	rt.SetCaller(voter, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
	rt.SetAddressActorType(candidate, builtin.ExpertActorCodeID)
	rt.ExpectSend(candidate, builtin.MethodsExpert.CheckState, nil, big.Zero(), &builtin.CheckExpertStateReturn{AllowVote: true}, exitcode.Ok)
	rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.OnExpertVotesUpdated, &builtin.OnExpertVotesUpdatedParams{
		Expert: candidate,
		Votes:  newVotes,
	}, big.Zero(), nil, exitcode.Ok)
	rt.Call(h.Vote, &candidate)
	rt.Verify()
}

func (h *actorHarness) rescind(rt *mock.Runtime, voter, candidate address.Address, votes abi.TokenAmount) {
	st := getState(rt)
	candidates, err := adt.AsMap(adt.AsStore(rt), st.Candidates, builtin.DefaultHamtBitwidth)
	require.NoError(h.t, err)
	var out vote.Candidate
	found, err := candidates.Get(abi.AddrKey(candidate), &out)
	require.NoError(h.t, err)
	newVotes := big.Zero()
	if found && out.Votes.GreaterThanEqual(votes) {
		newVotes = big.Sub(out.Votes, votes)
	}

	params := vote.RescindParams{
		Candidate: candidate,
		Votes:     votes,
	}
	rt.SetCaller(voter, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
	rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.OnExpertVotesUpdated, &builtin.OnExpertVotesUpdatedParams{
		Expert: candidate,
		Votes:  newVotes,
	}, big.Zero(), nil, exitcode.Ok)
	rt.Call(h.Rescind, &params)
	rt.Verify()
}

func (h *actorHarness) onCandidateBlocked(rt *mock.Runtime, candidate address.Address) {
	rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
	ret := rt.Call(h.OnCandidateBlocked, &candidate)
	assert.Nil(h.t, ret)
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

func (h *actorHarness) withdraw(rt *mock.Runtime, voter address.Address, expectVoterAmount, expectFallbackAmount abi.TokenAmount) {
	rt.SetCaller(voter, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
	if !expectVoterAmount.IsZero() {
		rt.ExpectSend(voter, builtin.MethodSend, nil, expectVoterAmount, nil, exitcode.Ok)
	}
	if !expectFallbackAmount.IsZero() {
		rt.ExpectSend(h.fallback, builtin.MethodSend, nil, expectFallbackAmount, nil, exitcode.Ok)
	}
	rt.Call(h.Withdraw, nil)
	rt.Verify()
}

type expectPoolInfo struct {
	PrevEpochEarningsPerVote abi.TokenAmount
	PrevEpoch                abi.ChainEpoch
	CurrEpochRewards         abi.TokenAmount
	CurrEpoch                abi.ChainEpoch
	CurrEpochEffectiveVotes  abi.TokenAmount
	LastRewardBalance        abi.TokenAmount
	FallbackDebt             abi.TokenAmount
	TotalVotes               abi.TokenAmount
}

func newExpectPoolInfo() *expectPoolInfo {
	return &expectPoolInfo{
		PrevEpochEarningsPerVote: abi.NewTokenAmount(0),
		PrevEpoch:                abi.ChainEpoch(0),
		CurrEpochRewards:         abi.NewTokenAmount(0),
		CurrEpoch:                abi.ChainEpoch(0),
		CurrEpochEffectiveVotes:  abi.NewTokenAmount(0),
		LastRewardBalance:        abi.NewTokenAmount(0),
		FallbackDebt:             abi.NewTokenAmount(0),
		TotalVotes:               abi.NewTokenAmount(0),
	}
}
func (e *expectPoolInfo) withPrevEpochEarningsPerVote(v int64) *expectPoolInfo {
	e.PrevEpochEarningsPerVote = abi.NewTokenAmount(v)
	return e
}
func (e *expectPoolInfo) withCurrEpochRewards(v int64) *expectPoolInfo {
	e.CurrEpochRewards = abi.NewTokenAmount(v)
	return e
}
func (e *expectPoolInfo) withCurrEpochEffectiveVotes(v int64) *expectPoolInfo {
	e.CurrEpochEffectiveVotes = abi.NewTokenAmount(v)
	return e
}
func (e *expectPoolInfo) withTotalVotes(v int64) *expectPoolInfo {
	e.TotalVotes = abi.NewTokenAmount(v)
	return e
}
func (e *expectPoolInfo) withLastRewardBalance(v int64) *expectPoolInfo {
	e.LastRewardBalance = abi.NewTokenAmount(v)
	return e
}
func (e *expectPoolInfo) withFallbackDebt(v int64) *expectPoolInfo {
	e.FallbackDebt = abi.NewTokenAmount(v)
	return e
}
func (e *expectPoolInfo) withPrevEpoch(v int64) *expectPoolInfo {
	e.PrevEpoch = abi.ChainEpoch(v)
	return e
}
func (e *expectPoolInfo) withCurrEpoch(v int64) *expectPoolInfo {
	e.CurrEpoch = abi.ChainEpoch(v)
	return e
}

func (h *actorHarness) expectPool(rt *mock.Runtime, e *expectPoolInfo) {
	st := getState(rt)
	require.True(h.t, st.PrevEpochEarningsPerVote.Equals(e.PrevEpochEarningsPerVote) &&
		st.CurrEpochRewards.Equals(e.CurrEpochRewards) &&
		st.CurrEpochEffectiveVotes.Equals(e.CurrEpochEffectiveVotes) &&
		st.LastRewardBalance.Equals(e.LastRewardBalance) &&
		st.TotalVotes.Equals(e.TotalVotes) &&
		st.FallbackDebt.Equals(e.FallbackDebt) &&
		st.PrevEpoch == e.PrevEpoch &&
		st.CurrEpoch == e.CurrEpoch,
		fmt.Sprintf("%+v", st),
	)
}

func (h *actorHarness) expectWithdrawable(rt *mock.Runtime, voterAddr address.Address, amt int64) {
	st := getState(rt)
	voter, err := st.EstimateSettle(adt.AsStore(rt), voterAddr, rt.Epoch(), big.Sub(rt.Balance(), st.TotalVotes))
	require.NoError(h.t, err)
	require.True(h.t, voter.Withdrawable.Equals(abi.NewTokenAmount(amt)), voter.Withdrawable.Int64())
}

func (h *actorHarness) expectCandidate(rt *mock.Runtime, candAddr address.Address, expectFound bool, amt int64) {
	st := getState(rt)
	cand, found, err := st.GetCandidate(adt.AsStore(rt), candAddr)
	require.NoError(h.t, err)
	if expectFound {
		require.True(h.t, found && cand.Votes.Equals(abi.NewTokenAmount(amt)))
	} else {
		require.True(h.t, !found)
	}
}
