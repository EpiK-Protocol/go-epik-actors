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
	builtin.KnowledgeFundsActorCodeID: {
		builtin.MethodsKnowledge.ChangePayee: struct{}{},
	},
	builtin.StorageMarketActorCodeID: {
		builtin.MethodsMarket.ResetQuotas:     struct{}{},
		builtin.MethodsMarket.SetInitialQuota: struct{}{},
	},
}

var GovernedCallerTypes = func() []cid.Cid {
	ret := make([]cid.Cid, 0, len(GovernedActors))
	for code := range GovernedActors {
		ret = append(ret, code)
	}
	return ret
}()
