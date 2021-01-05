package reward

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type StateSummary struct{}

var EPK = big.NewInt(1e18)
var StorageMiningAllocationCheck = big.Mul(big.NewInt(700_000_000), EPK)

func CheckStateInvariants(st *State, store adt.Store, priorEpoch abi.ChainEpoch, balance abi.TokenAmount) (*StateSummary, *builtin.MessageAccumulator, error) {
	acc := &builtin.MessageAccumulator{}

	totalReward := big.Sum(st.TotalExpertReward, st.TotalStoragePowerReward, st.TotalVoteReward, st.TotalKnowledgeReward, st.TotalRetrievalReward, st.TotalSendFailed)
	// Can't assert equality because anyone can send funds to reward actor (and already have on mainnet)
	acc.Require(big.Add(totalReward, balance).GreaterThanEqual(StorageMiningAllocationCheck), "reward given %v + reward left %v < storage mining allocation %v", st.TotalStoragePowerReward, balance, StorageMiningAllocationCheck)

	acc.Require(st.Epoch <= priorEpoch+1, "reward state epoch %d does not match priorEpoch+1 %d", st.Epoch, priorEpoch+1)

	return &StateSummary{}, acc, nil
}
