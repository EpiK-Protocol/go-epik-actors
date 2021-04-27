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

	t.Run("multi voters for one candidate", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetBalance(big.Zero())

		sum := actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 0 && sum.VotersCount == 0)

		// first voter
		rt.SetReceived(big.NewInt(1000))
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 1)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1000)))

		// second voter
		caller2 := tutil.NewIDAddr(t, 200)
		rt.SetReceived(big.NewInt(300))
		actor.vote(rt, caller2, candidate1, big.NewInt(300))

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
		rt.SetReceived(big.NewInt(1000))
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 1)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.VoterTallyCount[caller] == 1)

		// second
		rt.SetReceived(big.NewInt(300))
		candidate2 := tutil.NewIDAddr(t, 200)
		actor.vote(rt, caller, candidate2, big.NewInt(300))

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
		rt.SetReceived(big.NewInt(1000))
		actor.vote(rt, caller, candidate1, big.NewInt(1000))

		sum = actor.checkState(rt)
		require.True(t, sum.CandidatesCount == 1 && sum.VotersCount == 1)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.VoterTallyCount[caller] == 1)

		// second
		rt.SetReceived(big.NewInt(300))
		actor.vote(rt, caller, candidate1, big.NewInt(300))

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

		rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "candidate not found", func() {
			rt.Call(actor.OnCandidateBlocked, &candidate2)
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
		rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
		rt.Call(actor.OnCandidateBlocked, &candidate1)

		sum = actor.checkState(rt)
		require.True(t, sum.TotalNonBlockedVotes.Equals(big.NewInt(500)))
		require.True(t, sum.TotalBlockedVotes.Equals(big.NewInt(1000)))
		require.True(t, sum.BlockedAt[candidate1] == 99)
		rt.GetState(&st)
		require.True(t, st.TotalVotes.Equals(big.NewInt(500)))

		// block candidate2
		rt.SetEpoch(100)
		rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
		rt.Call(actor.OnCandidateBlocked, &candidate2)

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
		rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.OnExpertVotesUpdated, &builtin.OnExpertVotesUpdatedParams{Expert: candidate1, Votes: big.NewInt(999)}, big.Zero(), nil, exitcode.Ok)
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

	t.Run("withdraw after lock period over", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetBalance(big.NewInt(1000))

		rt.SetEpoch(100)
		actor.vote(rt, caller, candidate1, big.NewInt(1000))
		actor.cronTick(rt, nil)

		rt.SetEpoch(101)
		actor.rescind(rt, caller, candidate1, big.NewInt(1))
		actor.cronTick(rt, nil)

		var st vote.State
		rt.GetState(&st)
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
		require.True(t, st.TotalVotes.Equals(big.NewInt(999)))
		require.True(t, st.CumEarningsPerVote.IsZero())

		// last epoch that not withdrawable
		rt.SetEpoch(101 + vote.RescindingUnlockDelay)
		actor.withdraw(rt, caller, caller, big.Zero())
		require.True(t, rt.Balance().Equals(big.NewInt(1000)))

		rt.SetEpoch(102 + vote.RescindingUnlockDelay)
		actor.withdraw(rt, caller, caller, big.NewInt(1))
		require.True(t, rt.Balance().Equals(big.NewInt(999)))
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

	t.Run("fail when voter not found", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "voter not found", func() {
			rt.Call(actor.Withdraw, nil)
		})
	})

	t.Run("no votes and rewards", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.vote(rt, caller, candidate1, big.NewInt(10))

		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		v := rt.Call(actor.Withdraw, nil)
		amt := v.(*abi.TokenAmount)
		require.True(t, amt.IsZero())
	})
}

func TestOnEpochTickEnd(t *testing.T) {
	caller := tutil.NewIDAddr(t, 100)
	caller2 := tutil.NewIDAddr(t, 101)
	fallback := tutil.NewIDAddr(t, 102)
	candidate1 := tutil.NewIDAddr(t, 103)
	candidate2 := tutil.NewIDAddr(t, 104)
	candidate3 := tutil.NewIDAddr(t, 105)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), builtin.VoteFundActorAddr).
			WithActorType(fallback, builtin.AccountActorCodeID)
		rt := builder.Build(t)

		actor := newHarness(t, fallback)
		actor.constructAndVerify(rt)

		return rt, actor
	}

	t.Run("send funds to fallback when no votes", func(t *testing.T) {
		rt, actor := setupFunc()

		// zero funds, no send
		rt.SetEpoch(100)
		actor.applyRewards(rt, abi.NewTokenAmount(0))
		var st vote.State
		rt.GetState(&st)
		require.True(t, st.UnownedFunds.Equals(big.NewInt(0)))

		rt.SetCaller(builtin.CronActorAddr, builtin.CronActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.CronActorAddr)
		rt.Call(actor.OnEpochTickEnd, nil)
		rt.Verify()

		// non-zero funds, send to fallback
		rt.SetEpoch(101)
		funds := big.NewInt(13)
		rt.SetBalance(funds)
		actor.applyRewards(rt, funds)
		rt.GetState(&st)
		require.True(t, st.UnownedFunds.Equals(funds))

		rt.SetCaller(builtin.CronActorAddr, builtin.CronActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.CronActorAddr)
		rt.ExpectSend(fallback, builtin.MethodSend, nil, funds, nil, exitcode.Ok)
		rt.Call(actor.OnEpochTickEnd, nil)
		rt.Verify()

		rt.GetState(&st)
		require.True(t, st.UnownedFunds.IsZero() && rt.Balance().Sign() == 0)
	})

	t.Run("vote, withdraw and rescind", func(t *testing.T) {
		rt, actor := setupFunc()

		// vote at 100
		rt.SetEpoch(100)
		actor.vote(rt, caller, candidate1, abi.NewTokenAmount(99))
		actor.cronTick(rt, nil)

		// apply rewards at 101
		rt.SetEpoch(101)
		actor.applyRewards(rt, abi.NewTokenAmount(100))
		actor.cronTick(rt, nil)
		rt.SetBalance(abi.NewTokenAmount(199)) // 100 rewards + 99 votes

		var st vote.State
		rt.GetState(&st)
		cumPerVote := st.CumEarningsPerVote
		require.True(t, cumPerVote.Equals(big.NewInt(1010101010101))) // ⌊100 * 1e12 / 99⌋
		require.True(t, st.UnownedFunds.Equals(big.NewInt(1)))        // 100 - ⌊1010101010101 * 99 / 1e12⌋

		// actual states
		voter, _ := st.GetVoter(adt.AsStore(rt), caller)
		require.True(t, voter.SettleEpoch == 100)
		require.True(t, voter.Withdrawable.Equals(big.Zero()))
		require.True(t, voter.SettleCumEarningsPerVote.Equals(big.Zero()))

		// estimate settle states
		voter, _ = st.EstimateSettle(adt.AsStore(rt), caller, 101)
		require.True(t, voter.Withdrawable.Equals(abi.NewTokenAmount(99)))
		require.True(t, voter.SettleCumEarningsPerVote.Equals(st.CumEarningsPerVote))

		// withdaw at 102
		rt.SetEpoch(102)
		actor.withdraw(rt, caller, caller, abi.NewTokenAmount(99))
		require.True(t, rt.Balance().Equals(abi.NewTokenAmount(100)))
		actor.cronTick(rt, nil)

		rt.GetState(&st)
		require.True(t, st.UnownedFunds.Equals(big.NewInt(1)))
		one99Delta := abi.NewTokenAmount(10101010101)                                  // ⌊1 * 1e12 / 99⌋
		require.True(t, st.CumEarningsPerVote.Equals(big.Add(cumPerVote, one99Delta))) // 1020202020202

		// resciding at 103
		rt.SetEpoch(103)
		actor.rescind(rt, caller, candidate1, abi.NewTokenAmount(88))
		actor.cronTick(rt, nil)
		rt.GetState(&st)
		deltaltaAfterRescind := abi.NewTokenAmount(90909090909)                                                         // ⌊1 * 1e12 / 11⌋ = 90909090909
		require.True(t, st.CumEarningsPerVote.Equals(big.Add(abi.NewTokenAmount(1020202020202), deltaltaAfterRescind))) // 1111111111111
		voter, _ = st.EstimateSettle(adt.AsStore(rt), caller, 103)
		require.True(t, voter.Withdrawable.IsZero()) //  st.UnownedFunds = 1
		cumPerVote = st.CumEarningsPerVote

		// cron to 104
		rt.SetEpoch(104)
		list, err := st.ListVotesInfo(adt.AsStore(rt), caller)
		require.NoError(t, err)
		require.True(t, len(list) == 1 && st.TotalVotes.Equals(abi.NewTokenAmount(11)))
		require.True(t, list[candidate1].RescindingVotes.Equals(abi.NewTokenAmount(88))) // caller 1 rescinding 88
		require.True(t, list[candidate1].Votes.Equals(abi.NewTokenAmount(11)))
		require.True(t, list[candidate1].LastRescindEpoch == 103)

		actor.vote(rt, caller2, candidate2, abi.NewTokenAmount(80)) // call1 - 11, caller2 - 80
		actor.applyRewards(rt, abi.NewTokenAmount(90))              // UnownedFunds = 91
		actor.cronTick(rt, nil)
		rt.SetBalance(abi.NewTokenAmount(270)) // call1 99(11+88) + call2 80 + UnownedFunds 91

		rt.GetState(&st)
		deltaPerVote := calcExpectDeltaPerVote(abi.NewTokenAmount(91), abi.NewTokenAmount(91)) // 1000000000000
		require.True(t, big.Sub(st.CumEarningsPerVote, cumPerVote).Equals(deltaPerVote))

		voter1, _ := st.EstimateSettle(adt.AsStore(rt), caller, 104)
		require.True(t, voter1.Withdrawable.Equals(abi.NewTokenAmount(11)))
		voter2, _ := st.EstimateSettle(adt.AsStore(rt), caller2, 104)
		require.True(t, voter2.Withdrawable.Equals(abi.NewTokenAmount(80)))
		require.True(t, st.UnownedFunds.IsZero())
	})

	t.Run("block some candidates", func(t *testing.T) {
		rt, actor := setupFunc()

		expW := func(w1, w2 int64) {
			var st vote.State
			rt.GetState(&st)

			voter1, _ := st.EstimateSettle(adt.AsStore(rt), caller, 100)
			require.True(t, voter1.Withdrawable.Equals(abi.NewTokenAmount(w1)))
			voter2, _ := st.EstimateSettle(adt.AsStore(rt), caller2, 100)
			require.True(t, voter2.Withdrawable.Equals(abi.NewTokenAmount(w2)))
		}

		// award 120 attoEPK per epoch
		rt.SetEpoch(100)
		actor.vote(rt, caller, candidate1, abi.NewTokenAmount(10))
		actor.vote(rt, caller, candidate2, abi.NewTokenAmount(10))
		actor.vote(rt, caller, candidate3, abi.NewTokenAmount(10))
		actor.vote(rt, caller2, candidate3, abi.NewTokenAmount(10))
		actor.applyRewards(rt, abi.NewTokenAmount(120)) // 30:10
		actor.cronTick(rt, nil)

		rt.SetEpoch(102)
		actor.applyRewards(rt, abi.NewTokenAmount(240)) // 120*2, 30:10
		actor.cronTick(rt, nil)
		expW(270, 90)

		// block candidate1
		rt.SetEpoch(103)
		actor.blockCandidate(rt, candidate1)
		actor.applyRewards(rt, abi.NewTokenAmount(120)) // 20:10
		actor.cronTick(rt, nil)
		expW(350, 130)

		// caller2 vote for candidate3
		rt.SetEpoch(104)
		actor.vote(rt, caller2, candidate3, abi.NewTokenAmount(10))
		actor.applyRewards(rt, abi.NewTokenAmount(120)) // 20:20
		actor.cronTick(rt, nil)
		expW(410, 190)

		// caller1 vote for candidate3
		rt.SetEpoch(105)
		actor.vote(rt, caller, candidate3, abi.NewTokenAmount(10))
		actor.applyRewards(rt, abi.NewTokenAmount(120)) // 30:20
		actor.cronTick(rt, nil)
		expW(482, 238)

		// block candidate2
		rt.SetEpoch(106)
		actor.blockCandidate(rt, candidate2)
		actor.applyRewards(rt, abi.NewTokenAmount(120)) // 20:20
		actor.cronTick(rt, nil)
		expW(542, 298)

		var st vote.State
		rt.GetState(&st)
		require.True(t, st.TotalVotes.Equals(abi.NewTokenAmount(40)))
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

func (h *actorHarness) blockCandidate(rt *mock.Runtime, candidate address.Address) {
	rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
	ret := rt.Call(h.OnCandidateBlocked, &candidate)
	assert.Nil(h.t, ret)
	rt.Verify()
}

type cronTickConf struct {
	totalVotes   abi.TokenAmount
	unownedFunds abi.TokenAmount
}

func (h *actorHarness) cronTick(rt *mock.Runtime, conf *cronTickConf) {
	rt.ExpectValidateCallerAddr(builtin.CronActorAddr)
	rt.SetCaller(builtin.CronActorAddr, builtin.CronActorCodeID)
	if conf != nil && !conf.unownedFunds.IsZero() && conf.totalVotes.IsZero() {
		rt.ExpectSend(h.fallback, builtin.MethodSend, nil, conf.unownedFunds, nil, exitcode.Ok)
	}
	rt.Call(h.OnEpochTickEnd, nil)
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

func (h *actorHarness) withdraw(rt *mock.Runtime, voter, recipient address.Address, expectAmount abi.TokenAmount) {
	rt.SetCaller(voter, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
	if !expectAmount.IsZero() {
		rt.ExpectSend(recipient, builtin.MethodSend, nil, expectAmount, nil, exitcode.Ok)
	}
	v := rt.Call(h.Withdraw, nil)
	rt.Verify()
	require.True(h.t, v.(*abi.TokenAmount).Equals(expectAmount))
}
