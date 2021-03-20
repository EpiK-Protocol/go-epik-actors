package expert

import (
	abi "github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
)

/*

								 + ----------------------------------------------------------------------------------- +
								 |																 					   |
								 |																 				  	   |
								 |		  +	---------------------> blocked	<-------------------------------------- +  |
								 |		  |						      ↑			  	  	 				   	  	    |  |
					    		 |	      |                           |             						    	|  |
				    			 |	      |                	          |            									|  |
		            			 ↓     	  |	 (enough votes)			  |             (no enough vote for 3 days)     |  |
	registered ------------> nominated -- +	----------------> normal(qualified) -------------------------------> disqualified
										  |																			 ↑
										  |																			 |
										  |	                    (no enough vote for 3 days)         				 |
										  + ------------------------------------------------------------------------ +
*/

// ExpertState is the state of expert.
type ExpertState uint64

func (es ExpertState) AllowVote() bool {
	return es == ExpertStateNormal || es == ExpertStateNominated
}

// Qualified true means expert can:
//	1. import new data
//	2. nominate new expert
//	3. accept new miner to store his data
func (es ExpertState) Qualified() bool {
	return es == ExpertStateNormal
}

const (
	// ExpertStateRegistered new registered expert.
	ExpertStateRegistered ExpertState = iota

	// Expert was nominated by qualified one.
	ExpertStateNominated

	// Expert can import data and nominate new expert.
	ExpertStateNormal

	// ExpertStateBlocked blocked expert
	ExpertStateBlocked

	// Expert was disqualified, cause he has no enough votes for at least 3 days.
	ExpertStateDisqualified
)

// ExpertApplyCost expert apply cost
var ExpertApplyCost = big.Mul(big.NewInt(99), builtin.TokenPrecision)

// ExpertVoteThreshold threshold of expert vote amount
var ExpertVoteThreshold = big.Mul(big.NewInt(100000), builtin.TokenPrecision)

// ExpertVoteThresholdAddition addition threshold of expert vote amount
var ExpertVoteThresholdAddition = big.Mul(big.NewInt(25000), builtin.TokenPrecision)

// ExpertVoteCheckPeriod period of expert vote check duration
var ExpertVoteCheckPeriod = abi.ChainEpoch(3 * builtin.EpochsInDay) // 3 * 24 hours PARAM_SPEC

// Only used for owner change by governor
var NewOwnerActivateDelay = abi.ChainEpoch(3 * builtin.EpochsInDay) // 3 * 24 hours PARAM_SPEC

const NoLostEpoch = abi.ChainEpoch(-1)
