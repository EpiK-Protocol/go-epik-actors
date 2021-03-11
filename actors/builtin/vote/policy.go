package vote

import (
	abi "github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
)

const RescindingUnlockDelay = 3 * builtin.EpochsInDay

var Multiplier1E12 = abi.NewTokenAmount(1e12)
