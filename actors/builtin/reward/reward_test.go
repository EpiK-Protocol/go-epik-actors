package reward_test

import (
	"context"
	"fmt"
	"testing"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/reward"
	"github.com/filecoin-project/specs-actors/v2/support/mock"
	tutil "github.com/filecoin-project/specs-actors/v2/support/testing"
)

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, reward.Actor{})
}

// const EpochZeroReward = "36266264293777134739"

func TestConstructor(t *testing.T) {
	actor := rewardHarness{reward.Actor{}, t}

	t.Run("construct with 0 power", func(t *testing.T) {
		rt := mock.NewBuilder(context.Background(), builtin.RewardActorAddr).
			WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID).
			Build(t)
		startRealizedPower := abi.NewStoragePower(0)
		actor.constructAndVerify(rt, &startRealizedPower)
		st := getState(rt)
		assert.Equal(t, abi.ChainEpoch(0), st.Epoch)
		/* assert.Equal(t, reward.DefaultSimpleTotal, st.SimpleTotal) */
		assert.Equal(t, reward.EpochZeroReward, st.ThisEpochReward)
		assert.Equal(t, big.Zero(), st.TotalStoragePowerReward)
		// assert.Equal(t, abi.NewStoragePower(0), st.CumsumRealized)
		// epochZeroBaseline := big.Sub(reward.BaselineInitialValue, big.NewInt(1)) // account for rounding error of one byte during construction
		// assert.Equal(t, epochZeroBaseline, st.ThisEpochBaselinePower)
		// assert.Equal(t, reward.BaselineInitialValue, st.EffectiveBaselinePower)
	})
	/* t.Run("construct with less power than baseline", func(t *testing.T) {
		rt := mock.NewBuilder(context.Background(), builtin.RewardActorAddr).
			WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID).
			Build(t)
		startRealizedPower := big.Lsh(abi.NewStoragePower(1), 39)
		actor.constructAndVerify(rt, &startRealizedPower)
		st := getState(rt)
		assert.Equal(t, abi.ChainEpoch(0), st.Epoch)
		assert.Equal(t, startRealizedPower, st.CumsumRealized)

		assert.NotEqual(t, big.Zero(), st.ThisEpochReward)
	})
	t.Run("construct with more power than baseline", func(t *testing.T) {
		rt := mock.NewBuilder(context.Background(), builtin.RewardActorAddr).
			WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID).
			Build(t)
		startRealizedPower := reward.BaselineInitialValue
		actor.constructAndVerify(rt, &startRealizedPower)
		st := getState(rt)
		rwrd := st.ThisEpochReward

		// start with 2x power
		rt = mock.NewBuilder(context.Background(), builtin.RewardActorAddr).
			WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID).
			Build(t)
		startRealizedPower = big.Mul(reward.BaselineInitialValue, big.NewInt(2))
		actor.constructAndVerify(rt, &startRealizedPower)
		newSt := getState(rt)
		// Reward value is the same; realized power impact on reward is capped at baseline
		assert.Equal(t, rwrd, newSt.ThisEpochReward)
	}) */

}

// TODO: error
func TestAwardBlockReward(t *testing.T) {
	actor := rewardHarness{reward.Actor{}, t}
	winner := tutil.NewIDAddr(t, 1000)
	builder := mock.NewBuilder(context.Background(), builtin.RewardActorAddr).
		WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("rejects gas reward exceeding balance", func(t *testing.T) {
		rt := builder.Build(t)
		startRealizedPower := abi.NewStoragePower(0)
		actor.constructAndVerify(rt, &startRealizedPower)

		rt.SetBalance(abi.NewTokenAmount(9))
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		rt.ExpectAbort(exitcode.ErrIllegalState, func() {
			gasReward := big.NewInt(10)
			actor.awardBlockReward(rt, winner, big.Zero(), gasReward, 1, newEmptyDetail(), big.Zero())
		})
	})

	t.Run("rejects negative penalty or reward", func(t *testing.T) {
		rt := builder.Build(t)
		startRealizedPower := abi.NewStoragePower(0)
		actor.constructAndVerify(rt, &startRealizedPower)

		rt.SetBalance(abi.NewTokenAmount(1e18))
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			penalty := big.NewInt(-1)
			actor.awardBlockReward(rt, winner, penalty, big.Zero(), 1, newEmptyDetail(), big.Zero())
		})
		rt.Reset()
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			gasReward := big.NewInt(-1)
			actor.awardBlockReward(rt, winner, big.Zero(), gasReward, 1, newEmptyDetail(), big.Zero())
		})
	})

	t.Run("rejects non-positive sharecount", func(t *testing.T) {
		rt := builder.Build(t)
		startRealizedPower := abi.NewStoragePower(0)
		actor.constructAndVerify(rt, &startRealizedPower)

		rt.SetBalance(abi.NewTokenAmount(1e18))
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.awardBlockReward(rt, winner, big.Zero(), big.Zero(), 0, newEmptyDetail(), big.Zero())
		})
		rt.Reset()
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			actor.awardBlockReward(rt, winner, big.Zero(), big.Zero(), -1, newEmptyDetail(), big.Zero())
		})
	})

	t.Run("pays zero penalty and gas with no retrieval pledge", func(t *testing.T) {
		rt := builder.Build(t)
		startRealizedPower := abi.NewStoragePower(0)
		actor.constructAndVerify(rt, &startRealizedPower)

		oneEpk := abi.NewTokenAmount(1e18)

		rt.SetBalance(oneEpk)
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		detail := getMinerRewardDetail(oneEpk, big.Zero(), big.Zero(), 1)
		actor.awardBlockReward(rt, winner, big.Zero(), big.Zero(), 1, detail, big.Zero())
		rt.Reset()
	})

	t.Run("pays reward and tracks penalty", func(t *testing.T) {
		rt := builder.Build(t)
		startRealizedPower := abi.NewStoragePower(0)
		actor.constructAndVerify(rt, &startRealizedPower)

		rt.SetBalance(big.Mul(big.NewInt(1e9), abi.NewTokenAmount(1e18)))
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		penalty := big.NewInt(100)
		gasReward := big.NewInt(200)
		shareCount := int64(1)

		detail := getMinerRewardDetail(big.Div(reward.EpochZeroReward, big.NewInt(shareCount)), big.Zero(), big.Zero(), shareCount)
		detail.gasReward = gasReward
		actor.awardBlockReward(rt, winner, penalty, gasReward, shareCount, detail, big.Zero())
		rt.Reset()
	})

	t.Run("pays out current balance when reward exceeds total balance", func(t *testing.T) {
		rt := builder.Build(t)
		startRealizedPower := abi.NewStoragePower(1)
		actor.constructAndVerify(rt, &startRealizedPower)

		// Total reward is a huge number, upon writing ~1e18, so 300 should be way less
		smallReward := abi.NewTokenAmount(311)
		penalty := abi.NewTokenAmount(100)
		rt.SetBalance(smallReward)
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)

		minerPenalty := big.Mul(big.NewInt(reward.PenaltyMultiplier), penalty)
		expectedParams := builtin.ApplyRewardParams{Reward: big.NewInt(235), Penalty: minerPenalty}

		rt.ExpectSend(winner, builtin.MethodsMiner.ApplyRewards, &expectedParams, big.NewInt(235), nil, 0)
		rt.ExpectSend(builtin.VoteFundActorAddr, builtin.MethodsVote.ApplyRewards, nil, big.NewInt(3), &builtin.Discard{}, 0)
		rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.ApplyRewards, nil, big.NewInt(27), &builtin.Discard{}, 0)
		rt.ExpectSend(builtin.KnowledgeFundActorAddr, builtin.MethodsKnowledge.ApplyRewards, nil, big.NewInt(46), &builtin.Discard{}, 0)

		rt.Call(actor.AwardBlockReward, &reward.AwardBlockRewardParams{
			Miner:            winner,
			Penalty:          penalty,
			GasReward:        big.Zero(),
			ShareCount:       1,
			RetrievalPledged: big.Zero(),
		})
		rt.Verify()
	})

	t.Run("pays 2.26% bandwidth reward", func(t *testing.T) {
		rt := builder.Build(t)
		startRealizedPower := abi.NewStoragePower(1)
		actor.constructAndVerify(rt, &startRealizedPower)

		// Total reward is a huge number, upon writing ~1e18, so 300 should be way less
		smallReward := abi.NewTokenAmount(311)
		penalty := abi.NewTokenAmount(100)
		rt.SetBalance(smallReward)
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		rt.SetCirculatingSupply(big.NewInt(1000))

		minerPenalty := big.Mul(big.NewInt(reward.PenaltyMultiplier), penalty)
		expectedParams := builtin.ApplyRewardParams{Reward: big.NewInt(235), Penalty: minerPenalty}

		rt.ExpectSend(winner, builtin.MethodsMiner.ApplyRewards, &expectedParams, big.NewInt(235), nil, 0)
		rt.ExpectSend(builtin.VoteFundActorAddr, builtin.MethodsVote.ApplyRewards, nil, big.NewInt(3), &builtin.Discard{}, 0)
		rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.ApplyRewards, nil, big.NewInt(27), &builtin.Discard{}, 0)
		rt.ExpectSend(builtin.KnowledgeFundActorAddr, builtin.MethodsKnowledge.ApplyRewards, nil, big.NewInt(39), &builtin.Discard{}, 0)
		rt.ExpectSend(builtin.RetrievalFundActorAddr, builtin.MethodsRetrieval.ApplyRewards, nil, big.NewInt(7), &builtin.Discard{}, 0)

		rt.Call(actor.AwardBlockReward, &reward.AwardBlockRewardParams{
			Miner:            winner,
			Penalty:          penalty,
			GasReward:        big.Zero(),
			ShareCount:       1,
			RetrievalPledged: big.NewInt(113),
		})
		rt.Verify()
	})

	t.Run("TotalStoragePowerReward tracks correctly", func(t *testing.T) {
		rt := builder.Build(t)
		startRealizedPower := abi.NewStoragePower(1)
		actor.constructAndVerify(rt, &startRealizedPower)
		miner := tutil.NewIDAddr(t, 1000)

		st := getState(rt)
		assert.Equal(t, big.Zero(), st.TotalStoragePowerReward)
		st.ThisEpochReward = abi.NewTokenAmount(1000)
		rt.ReplaceState(st)
		// enough balance to pay 3 full rewards and one partial
		totalPayout := abi.NewTokenAmount(3500)
		rt.SetBalance(totalPayout)

		detail1000 := getMinerRewardDetail(big.NewInt(1000), big.Zero(), big.Zero(), 1)
		detail500 := getMinerRewardDetail(big.NewInt(500), big.Zero(), big.Zero(), 1)

		// award normalized by expected leaders is 1000
		actor.awardBlockReward(rt, miner, big.Zero(), big.Zero(), 1, detail1000, big.Zero())
		actor.awardBlockReward(rt, miner, big.Zero(), big.Zero(), 1, detail1000, big.Zero())
		actor.awardBlockReward(rt, miner, big.Zero(), big.Zero(), 1, detail1000, big.Zero())
		actor.awardBlockReward(rt, miner, big.Zero(), big.Zero(), 1, detail500, big.Zero()) // partial payout when balance below reward

		newState := getState(rt)
		assert.Equal(t, big.Add(detail500.powerReward, big.Mul(detail1000.powerReward, big.NewInt(3))), newState.TotalStoragePowerReward)
		assert.True(t, rt.Balance().Equals(abi.NewTokenAmount(0)))
	})

	t.Run("funds are sent to the burnt funds actor if sending locked funds to miner fails", func(t *testing.T) {
		rt := builder.Build(t)
		startRealizedPower := abi.NewStoragePower(1)
		actor.constructAndVerify(rt, &startRealizedPower)
		miner := tutil.NewIDAddr(t, 1000)
		st := getState(rt)
		assert.Equal(t, big.Zero(), st.TotalStoragePowerReward)
		st.ThisEpochReward = abi.NewTokenAmount(1000)
		rt.ReplaceState(st)
		// enough balance to pay 3 full rewards and one partial
		totalPayout := abi.NewTokenAmount(3500)
		rt.SetBalance(totalPayout)

		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		expectedReward := big.NewInt(750)
		penalty := big.Zero()
		expectedParams := builtin.ApplyRewardParams{Reward: expectedReward, Penalty: penalty}
		rt.ExpectSend(miner, builtin.MethodsMiner.ApplyRewards, &expectedParams, expectedReward, nil, exitcode.ErrForbidden)
		rt.ExpectSend(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, expectedReward, nil, exitcode.Ok)
		rt.ExpectSend(builtin.VoteFundActorAddr, builtin.MethodsVote.ApplyRewards, nil, big.NewInt(10), &builtin.Discard{}, 0)
		rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.ApplyRewards, nil, big.NewInt(90), &builtin.Discard{}, 0)
		rt.ExpectSend(builtin.KnowledgeFundActorAddr, builtin.MethodsKnowledge.ApplyRewards, nil, big.NewInt(150), &builtin.Discard{}, 0)

		rt.Call(actor.AwardBlockReward, &reward.AwardBlockRewardParams{
			Miner:            miner,
			Penalty:          big.Zero(),
			GasReward:        big.Zero(),
			ShareCount:       1,
			RetrievalPledged: big.Zero(),
		})
		assert.True(t, rt.Balance().Equals(abi.NewTokenAmount(2500)), rt.Balance())
		rt.Verify()
	})
}

func TestThisEpochReward(t *testing.T) {
	t.Run("successfully fetch reward for this epoch", func(t *testing.T) {
		actor := rewardHarness{reward.Actor{}, t}
		builder := mock.NewBuilder(context.Background(), builtin.RewardActorAddr).
			WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
		rt := builder.Build(t)
		power := abi.NewStoragePower(1 << 50)
		actor.constructAndVerify(rt, &power)

		resp := actor.thisEpochReward(rt)
		st := getState(rt)

		require.EqualValues(t, st.Epoch, resp.Epoch)
		require.EqualValues(t, st.ThisEpochReward, resp.ThisEpochReward)
		require.EqualValues(t, st.TotalStoragePowerReward, resp.TotalStoragePowerReward)
	})
}

func TestSuccessiveKPIUpdates(t *testing.T) {
	actor := rewardHarness{reward.Actor{}, t}
	builder := mock.NewBuilder(context.Background(), builtin.RewardActorAddr).
		WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
	rt := builder.Build(t)
	power := abi.NewStoragePower(1 << 50)
	actor.constructAndVerify(rt, &power)

	rt.SetEpoch(abi.ChainEpoch(0))
	actor.updateNetworkKPI(rt, &power)
	st := getState(rt)
	require.EqualValues(t, st.Epoch, 1)
	require.EqualValues(t, st.ThisEpochReward, reward.EpochZeroReward)
	require.EqualValues(t, st.TotalStoragePowerReward, abi.NewTokenAmount(0))

	rt.SetEpoch(abi.ChainEpoch(1))
	actor.updateNetworkKPI(rt, &power)
	st = getState(rt)
	require.EqualValues(t, st.Epoch, 2)
	require.EqualValues(t, st.ThisEpochReward, reward.EpochZeroReward)
	require.EqualValues(t, st.TotalStoragePowerReward, abi.NewTokenAmount(0))

	rt.SetEpoch(90*builtin.EpochsInDay - 2)
	actor.updateNetworkKPI(rt, &power)
	st = getState(rt)
	require.EqualValues(t, st.Epoch, 90*builtin.EpochsInDay-1)
	require.EqualValues(t, st.ThisEpochReward, reward.EpochZeroReward)
	require.EqualValues(t, st.TotalStoragePowerReward, abi.NewTokenAmount(0))

	rt.SetEpoch(90*builtin.EpochsInDay - 1)
	actor.updateNetworkKPI(rt, &power)
	st = getState(rt)
	require.EqualValues(t, st.Epoch, 90*builtin.EpochsInDay)
	require.NotEqualValues(t, st.ThisEpochReward, reward.EpochZeroReward, st.ThisEpochReward)
	require.EqualValues(t, st.TotalStoragePowerReward, abi.NewTokenAmount(0))
}

type rewardHarness struct {
	reward.Actor
	t testing.TB
}

func (h *rewardHarness) constructAndVerify(rt *mock.Runtime, currRawPower *abi.StoragePower) {
	rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	ret := rt.Call(h.Constructor, currRawPower)
	assert.Nil(h.t, ret)
	rt.Verify()

}

func (h *rewardHarness) updateNetworkKPI(rt *mock.Runtime, currRawPower *abi.StoragePower) {
	rt.SetCaller(builtin.StoragePowerActorAddr, builtin.StoragePowerActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.StoragePowerActorAddr)
	ret := rt.Call(h.UpdateNetworkKPI, currRawPower)
	assert.Nil(h.t, ret)
	rt.Verify()
}

func (h *rewardHarness) awardBlockReward(
	rt *mock.Runtime, miner address.Address,
	penalty, gasReward abi.TokenAmount,
	shareCount int64,
	detail *rewardDetail, retrievalPledge abi.TokenAmount,
) {
	rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	// expect penalty multiplier
	minerPenalty := big.Mul(big.NewInt(reward.PenaltyMultiplier), penalty)
	expectedParams := builtin.ApplyRewardParams{Reward: big.Add(detail.powerReward, detail.gasReward), Penalty: minerPenalty}
	rt.ExpectSend(miner, builtin.MethodsMiner.ApplyRewards, &expectedParams, expectedParams.Reward, nil, 0)
	if !detail.voteReward.IsZero() {
		rt.ExpectSend(builtin.VoteFundActorAddr, builtin.MethodsVote.ApplyRewards, nil, detail.voteReward, &builtin.Discard{}, 0)
	}
	if !detail.expertReward.IsZero() {
		rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.ApplyRewards, nil, detail.expertReward, &builtin.Discard{}, 0)
	}
	if !detail.knowledgeReward.IsZero() {
		rt.ExpectSend(builtin.KnowledgeFundActorAddr, builtin.MethodsKnowledge.ApplyRewards, nil, detail.knowledgeReward, &builtin.Discard{}, 0)
	}
	if !detail.bandwidthReward.IsZero() {
		rt.ExpectSend(builtin.RetrievalFundActorAddr, builtin.MethodsRetrieval.ApplyRewards, nil, detail.bandwidthReward, &builtin.Discard{}, 0)
	}

	rt.Call(h.AwardBlockReward, &reward.AwardBlockRewardParams{
		Miner:            miner,
		Penalty:          penalty,
		GasReward:        gasReward,
		ShareCount:       shareCount,
		RetrievalPledged: retrievalPledge,
	})
	rt.Verify()
}

func (h *rewardHarness) thisEpochReward(rt *mock.Runtime) *reward.ThisEpochRewardReturn {
	rt.ExpectValidateCallerAny()

	ret := rt.Call(h.ThisEpochReward, nil)
	rt.Verify()

	resp, ok := ret.(*reward.ThisEpochRewardReturn)
	require.True(h.t, ok)
	return resp
}

func getState(rt *mock.Runtime) *reward.State {
	var st reward.State
	rt.GetState(&st)
	return &st
}

type rewardDetail struct {
	voteReward      abi.TokenAmount
	expertReward    abi.TokenAmount
	knowledgeReward abi.TokenAmount
	bandwidthReward abi.TokenAmount
	powerReward     abi.TokenAmount
	gasReward       abi.TokenAmount
}

func newEmptyDetail() *rewardDetail {
	return &rewardDetail{
		voteReward:      big.Zero(),
		expertReward:    big.Zero(),
		knowledgeReward: big.Zero(),
		bandwidthReward: big.Zero(),
		powerReward:     big.Zero(),
		gasReward:       big.Zero(),
	}
}

func (r *rewardDetail) Total() big.Int {
	return big.Add(
		r.voteReward, big.Add(
			r.expertReward, big.Add(
				r.knowledgeReward, big.Add(
					r.bandwidthReward, big.Add(r.powerReward, r.gasReward)))))
}

func (r *rewardDetail) String() string {
	return fmt.Sprintf("vote: %s, expert: %s, knowledge: %s, bandwidth: %s, power: %s, gas: %s, total: %s\n",
		r.voteReward, r.expertReward, r.knowledgeReward, r.bandwidthReward, r.powerReward, r.gasReward, r.Total())
}

func getMinerRewardDetail(block, retrievalPledged, circulating abi.TokenAmount, shareCount int64) *rewardDetail {

	detail := &rewardDetail{
		bandwidthReward: big.Zero(),
		gasReward:       big.Zero(),
	}
	detail.voteReward = big.Div(block, big.NewInt(100))                           // 1% to vote
	detail.expertReward = big.Div(big.Mul(block, big.NewInt(9)), big.NewInt(100)) // 9% to expert

	kb := big.Div(big.Mul(block, big.NewInt(15)), big.NewInt(100))                                    // 15% to knowledge and bandwidth
	detail.powerReward = big.Sub(big.Sub(big.Sub(block, detail.voteReward), detail.expertReward), kb) // 75% to miner

	a := big.Mul(retrievalPledged, big.NewInt(100))
	b := big.Mul(circulating, big.NewInt(75))
	c := big.Mul(circulating, big.NewInt(500))
	if !c.IsZero() {
		detail.bandwidthReward = big.Div(big.Mul(big.Min(a, b), block), c) // min(pledge/circulating/5, 15%) to bandwidth
	}
	detail.knowledgeReward = big.Sub(kb, detail.bandwidthReward) // 15% - min(pledge/circulating/5, 15%) to knowledge
	return detail
}
