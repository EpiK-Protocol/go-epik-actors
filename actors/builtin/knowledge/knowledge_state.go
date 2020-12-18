package knowledge

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
)

type State struct {
	// Current funds payee
	Payee addr.Address // ID-Address

	Tally cid.Cid // Map, HAMT [Payee]TokenAmount
}

func ConstructState(emptyMapCid cid.Cid, payee addr.Address) *State {
	return &State{
		Tally: emptyMapCid,
		Payee: payee,
	}
}
