package retrieval

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
)

const RetrievalSizePerEPK = 10 << 20

// RetrievalLockPeriod retrieval lock periods
const RetrievalLockPeriod = abi.ChainEpoch(3 * builtin.EpochsInDay) // 3 * 24 hours PARAM_SPEC

// RetrievalStateDuration retrieval state refresh duration
const RetrievalStateDuration = abi.ChainEpoch(builtin.EpochsInDay)
