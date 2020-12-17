package expertfund

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
)

// DefaultDataStoreThreshold default threshold
var DefaultDataStoreThreshold = uint64(10)

// AccumulatedMultiplier accumulated
var AccumulatedMultiplier = abi.NewTokenAmount(1e12)

type VestSpec struct {
	InitialDelay abi.ChainEpoch // Delay before any amount starts vesting.
	VestPeriod   abi.ChainEpoch // Period over which the total should vest, after the initial delay.
	StepDuration abi.ChainEpoch // Duration between successive incremental vests (independent of vesting period).
	Quantization abi.ChainEpoch // Maximum precision of vesting table (limits cardinality of table).
}

// The vesting schedule for total rewards (block reward + gas reward) earned by a block producer.
var RewardVestingSpec = VestSpec{ // PARAM_SPEC
	InitialDelay: abi.ChainEpoch(7 * builtin.EpochsInDay),
	VestPeriod:   abi.ChainEpoch(7 * builtin.EpochsInDay),
	StepDuration: abi.ChainEpoch(1 * builtin.EpochsInDay),
	Quantization: 12 * builtin.EpochsInHour,
}
