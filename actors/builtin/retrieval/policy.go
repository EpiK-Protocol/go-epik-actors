package retrieval

import (
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
)

// RetrievalSizePerEPK retrieval size stake per epk, 10Mib
const RetrievalSizePerEPK = 10 << 20

// RetrievalRewardPerByte retrieval reward per byte, 10M = 0.0002 EPK
var RetrievalRewardPerByte = big.Div(big.Mul(big.NewInt(2), builtin.TokenPrecision), big.NewInt(10000*10*1024*1024))

// RetrievalLockPeriod retrieval lock periods
const RetrievalLockPeriod = abi.ChainEpoch(3 * builtin.EpochsInDay) // 3 * 24 hours PARAM_SPEC

// RetrievalStateDuration retrieval state refresh duration
const RetrievalStateDuration = abi.ChainEpoch(builtin.EpochsInDay)
