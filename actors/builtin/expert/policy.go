package expert

import (
	abi "github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
)

// ExpertType is the expert type.
type ExpertType uint64

const (
	// ExpertFoundation foundation expert
	ExpertFoundation ExpertType = iota

	// ExpertNormal normal expert
	ExpertNormal
)

// ExpertState is the state of expert.
type ExpertState uint64

const (
	// ExpertStateRegistered registered expert
	ExpertStateRegistered ExpertState = iota

	// ExpertStateNormal foundation expert
	ExpertStateNormal

	// ExpertStateImplicated implicated expert
	ExpertStateImplicated

	// ExpertStateBlocked blocked expert
	ExpertStateBlocked
)

// ExpertApplyCost expert apply cost
var ExpertApplyCost = big.Mul(big.NewInt(99), builtin.TokenPrecision)

// ExpertVoteThreshold threshold of expert vote amount
var ExpertVoteThreshold = big.Mul(big.NewInt(100000), builtin.TokenPrecision)

// ExpertVoteThresholdAddition addition threshold of expert vote amount
var ExpertVoteThresholdAddition = big.Add(ExpertVoteThreshold, big.Mul(big.NewInt(25000), builtin.TokenPrecision))

// ExpertVoteCheckPeriod period of expert vote check duration
var ExpertVoteCheckPeriod = abi.ChainEpoch(3 * builtin.EpochsInDay) // 3 * 24 hours PARAM_SPEC
