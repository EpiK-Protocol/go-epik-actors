package knowledge

import (
	"github.com/filecoin-project/go-address"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/ipfs/go-cid"
	xerrors "golang.org/x/xerrors"
)

type State struct {
	// Current funds payee
	Payee addr.Address // ID-Address

	Tally cid.Cid // Map, HAMT [Payee]TokenAmount
}

func ConstructState(store adt.Store, initialPayee address.Address) (*State, error) {
	if initialPayee.Protocol() != address.ID {
		return nil, xerrors.New("intial payee address must be an ID address")
	}

	emptyMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty map: %w", err)
	}

	return &State{
		Tally: emptyMapCid,
		Payee: initialPayee,
	}, nil
}
