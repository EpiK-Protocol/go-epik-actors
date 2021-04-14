package power

import (
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
)

// The number of miners that must meet the consensus minimum miner power before that minimum power is enforced
// as a condition of leader election.
// This ensures a network still functions before any miners reach that threshold.
const ConsensusMinerMinMiners = 4 // PARAM_SPEC

// Maximum number of prove-commits each miner can submit in one epoch.
//
// This limits the number of proof partitions we may need to load in the cron call path.
// Onboarding 1EiB/year requires at least 32 prove-commits per epoch.
const MaxMinerProveCommitsPerEpoch = 200 // PARAM_SPEC

var ConsensusMinerMinPledge = big.Mul(big.NewInt(1000), builtin.TokenPrecision) // 1000 EPK

const MaxWindowPoStRatio = uint64(1000)
const MinWindowPoStRatio = uint64(0)

func IsValidWdPoStRatio(ratio uint64) bool {
	return ratio >= MinWindowPoStRatio && ratio <= MaxWindowPoStRatio
}
