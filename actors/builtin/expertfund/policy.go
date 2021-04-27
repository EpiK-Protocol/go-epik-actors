package expertfund

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
)

// DefaultDataStoreThreshold default threshold
var DefaultDataStoreThreshold = uint64(10)

// AccumulatedMultiplier accumulated
var AccumulatedMultiplier = abi.NewTokenAmount(1e12)

var RewardVestingDelay = abi.ChainEpoch(7 * builtin.EpochsInDay)

var ClearExpertContributionDelay = abi.ChainEpoch(3 * builtin.EpochsInDay) // 3 * 24 hours PARAM_SPEC
