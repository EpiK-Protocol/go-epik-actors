package reward

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	rtt "github.com/filecoin-project/go-state-types/rt"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
	. "github.com/filecoin-project/specs-actors/v2/actors/util"
)

// PenaltyMultiplier is the factor miner penaltys are scaled up by
const PenaltyMultiplier = 3

type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.AwardBlockReward,
		3:                         a.ThisEpochReward,
		4:                         a.UpdateNetworkKPI,
	}
}

func (a Actor) Code() cid.Cid {
	return builtin.RewardActorCodeID
}

func (a Actor) IsSingleton() bool {
	return true
}

func (a Actor) State() cbor.Er {
	return new(State)
}

var _ runtime.VMActor = Actor{}

func (a Actor) Constructor(rt runtime.Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	st := ConstructState()
	rt.StateCreate(st)
	return nil
}

type AwardBlockRewardParams struct {
	Miner     address.Address
	Penalty   abi.TokenAmount // penalty for including bad messages in a block, >= 0
	GasReward abi.TokenAmount // gas reward from all gas fees in a block, >= 0
	WinCount  int64           // number of reward units won, > 0

	ShareCount int64 // number of blocks in current tipset, sharing ThisEpochReward

	RetrievalPledged abi.TokenAmount // total retrieval pledged epik
}

// Awards a reward to a block producer.
// This method is called only by the system actor, implicitly, as the last message in the evaluation of a block.
// The system actor thus computes the parameters and attached value.
//
// The reward includes two components:
// - the epoch block reward, computed and paid from the reward actor's balance,
// - the block gas reward, expected to be transferred to the reward actor with this invocation.
//
// The reward is reduced before the residual is credited to the block producer, by:
// - a penalty amount, provided as a parameter, which is burnt,
func (a Actor) AwardBlockReward(rt runtime.Runtime, params *AwardBlockRewardParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)
	priorBalance := rt.CurrentBalance()
	if params.Penalty.LessThan(big.Zero()) {
		rt.Abortf(exitcode.ErrIllegalArgument, "negative penalty %v", params.Penalty)
	}
	if params.GasReward.LessThan(big.Zero()) {
		rt.Abortf(exitcode.ErrIllegalArgument, "negative gas reward %v", params.GasReward)
	}
	if priorBalance.LessThan(params.GasReward) {
		rt.Abortf(exitcode.ErrIllegalState, "actor current balance %v insufficient to pay gas reward %v",
			priorBalance, params.GasReward)
	}
	if params.ShareCount <= 0 {
		rt.Abortf(exitcode.ErrIllegalArgument, "non-positive share count %d", params.ShareCount)
	}

	if params.WinCount <= 0 {
		rt.Abortf(exitcode.ErrIllegalArgument, "non-positive win count %d", params.WinCount)
	}

	minerAddr, ok := rt.ResolveAddress(params.Miner)
	if !ok {
		rt.Abortf(exitcode.ErrNotFound, "failed to resolve given owner address")
	}
	// The miner penalty is scaled up by a factor of PenaltyMultiplier
	penalty := big.Mul(big.NewInt(PenaltyMultiplier), params.Penalty)
	expertReward := big.Zero()
	minerReward := big.Zero()
	voteReward := big.Zero()
	knowledgeReward := big.Zero()
	bandwidthReward := big.Zero()

	var st State
	rt.StateTransaction(&st, func() {
		// blockReward := big.Mul(st.ThisEpochReward, big.NewInt(params.WinCount))
		// blockReward = big.Div(blockReward, big.NewInt(builtin.ExpectedLeadersPerEpoch))
		blockReward := big.Div(st.ThisEpochReward, big.NewInt(params.ShareCount))
		totalReward := big.Add(blockReward, params.GasReward)
		currBalance := rt.CurrentBalance()
		if totalReward.GreaterThan(currBalance) {
			rt.Log(rtt.WARN, "reward actor balance %d below totalReward expected %d, paying out rest of balance", currBalance, totalReward)
			totalReward = currBalance

			blockReward = big.Sub(totalReward, params.GasReward)
			// // Since we have already asserted the balance is greater than gas reward blockReward is >= 0
			// AssertMsg(blockReward.GreaterThanEqual(big.Zero()), "programming error, block reward is %v below zero", blockReward)
		}
		AssertMsg(totalReward.LessThanEqual(priorBalance), "total reward %v exceeds balance %v", totalReward, priorBalance)

		var powerReward big.Int
		voteReward, expertReward, knowledgeReward, bandwidthReward, powerReward =
			distributeBlockRewards(blockReward, params.RetrievalPledged, rt.TotalFilCircSupply())

		st.TotalStoragePowerReward = big.Add(st.TotalStoragePowerReward, powerReward)
		minerReward = big.Add(powerReward, params.GasReward)
	})

	// if this fails, we can assume the miner is responsible and avoid failing here.
	rewardParams := builtin.ApplyRewardParams{
		Reward:  minerReward,
		Penalty: penalty,
	}
	code := rt.Send(minerAddr, builtin.MethodsMiner.ApplyRewards, &rewardParams, minerReward, &builtin.Discard{})
	if !code.IsSuccess() {
		rt.Log(rtt.ERROR, "failed to send ApplyRewards call to the miner actor with funds: %v, code: %v", minerReward, code)
		code := rt.Send(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, minerReward, &builtin.Discard{})
		if !code.IsSuccess() {
			rt.Log(rtt.ERROR, "failed to send unsent reward to the burnt funds actor, code: %v", code)
		}
	}

	if !voteReward.IsZero() {
		code = rt.Send(builtin.VoteFundActorAddr, builtin.MethodsVote.ApplyRewards, nil, voteReward, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds to vote")
	}

	if !expertReward.IsZero() {
		code = rt.Send(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.ApplyRewards, nil, expertReward, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds to expert")
	}

	if !knowledgeReward.IsZero() {
		code = rt.Send(builtin.KnowledgeFundActorAddr, builtin.MethodsKnowledge.ApplyRewards, nil, knowledgeReward, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds to knowledge")
	}

	if !bandwidthReward.IsZero() {
		code = rt.Send(builtin.RetrievalFundActorAddr, builtin.MethodsRetrieval.ApplyRewards, nil, bandwidthReward, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds to retrieval")
	}

	return nil
}

type ThisEpochRewardReturn struct {
	Epoch                   abi.ChainEpoch
	ThisEpochReward         abi.TokenAmount
	TotalStoragePowerReward abi.TokenAmount
}

// The award value used for the current epoch, updated at the end of an epoch
// through cron tick.  In the case previous epochs were null blocks this
// is the reward value as calculated at the last non-null epoch.
func (a Actor) ThisEpochReward(rt runtime.Runtime, _ *abi.EmptyValue) *ThisEpochRewardReturn {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)
	return &ThisEpochRewardReturn{
		Epoch:                   st.Epoch,
		ThisEpochReward:         st.ThisEpochReward,
		TotalStoragePowerReward: st.TotalStoragePowerReward,
	}
}

// Called at the end of each epoch by the power actor (in turn by its cron hook).
// This is only invoked for non-empty tipsets, but catches up any number of null
// epochs to compute the next epoch reward.
func (a Actor) UpdateNetworkKPI(rt runtime.Runtime, _ *abi.EmptyValue /* currRealizedPower *abi.StoragePower */) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.StoragePowerActorAddr)
	/* if currRealizedPower == nil {
		rt.Abortf(exitcode.ErrIllegalArgument, "arugment should not be nil")
	} */

	var st State
	rt.StateTransaction(&st, func() {
		if rt.CurrEpoch()+1 >= st.Epoch+RewardDecayPeriod {
			st.ThisEpochReward = big.Div(big.Mul(st.ThisEpochReward, DecayTarget.Numerator), DecayTarget.Denominator)
			st.Epoch = rt.CurrEpoch() + 1
		}
		/* prev := st.Epoch
		// if there were null runs catch up the computation until
		// st.Epoch == rt.CurrEpoch()
		for st.Epoch < rt.CurrEpoch() {
			// Update to next epoch to process null rounds
			st.updateToNextEpoch(*currRealizedPower)
		}

		st.updateToNextEpochWithReward(*currRealizedPower)
		// only update smoothed estimates after updating reward and epoch
		st.updateSmoothedEstimates(st.Epoch - prev) */
	})
	return nil
}
