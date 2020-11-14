package runtime

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/rt"
)

// Concrete types associated with the runtime interface.

// Result of checking two headers for a consensus fault.
// type ConsensusFault = runtime0.ConsensusFault

type ConsensusFault struct {
	// Address of the miner at fault (always an ID address).
	Target addr.Address
	// Epoch of the fault, which is the higher epoch of the two blocks causing it.
	Epoch abi.ChainEpoch
	// Type of fault.
	Type ConsensusFaultType
}

// type ConsensusFaultType = runtime0.ConsensusFaultType
type ConsensusFaultType int64

const (
	ConsensusFaultDoubleForkMining ConsensusFaultType = 1
	ConsensusFaultParentGrinding   ConsensusFaultType = 2
	ConsensusFaultTimeOffsetMining ConsensusFaultType = 3
)

type VMActor = rt.VMActor
