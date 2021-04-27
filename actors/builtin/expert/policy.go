package expert

import (
	abi "github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
)

// ExpertState is the state of expert.
type ExpertState uint64

func (es ExpertState) AllowVote() bool {
	return es == ExpertStateQualified || es == ExpertStateUnqualified
}

// Qualified true means expert can:
//	1. import new data
//	2. nominate new expert
func (es ExpertState) Qualified() bool {
	return es == ExpertStateQualified
}

const (
	// ExpertStateRegistered new registered expert.
	ExpertStateRegistered ExpertState = iota

	// Nominated and has enough votes
	ExpertStateQualified

	// Nominated but no enough votes
	ExpertStateUnqualified

	// Blocked by governor
	ExpertStateBlocked
)

// ExpertApplyCost expert apply cost
var ExpertApplyCost = big.Mul(big.NewInt(99), builtin.TokenPrecision)

// ExpertVoteThreshold threshold of expert vote amount
var ExpertVoteThreshold = big.Mul(big.NewInt(100000), builtin.TokenPrecision)

// ExpertVoteThresholdAddition addition threshold of expert vote amount
var ExpertVoteThresholdAddition = big.Mul(big.NewInt(25000), builtin.TokenPrecision)

// Only used for owner change by governor
var ActivateNewOwnerDelay = abi.ChainEpoch(3 * builtin.EpochsInDay) // 3 * 24 hours PARAM_SPEC
