package govern

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/ipfs/go-cid"
)

// Governed methods of each actor code
var GovernedActors = map[cid.Cid]map[abi.MethodNum]struct{}{
	builtin.ExpertActorCodeID: {
		// TODO:
	},
	builtin.ExpertFundActorCodeID: {
		// TODO:
	},
	builtin.KnowledgeActorCodeID: {
		builtin.MethodsKnowledge.ChangePayee: struct{}{},
	},
	builtin.StorageMarketActorCodeID: {
		builtin.MethodsMarket.ResetQuotas:     struct{}{},
		builtin.MethodsMarket.SetInitialQuota: struct{}{},
	},
}
