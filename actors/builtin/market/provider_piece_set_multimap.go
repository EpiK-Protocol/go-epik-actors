package market

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	cid "github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type ProviderPieceSetMultimap struct {
	mp            *adt.Map
	store         adt.Store
	innerBitwidth int
}

func AsProviderPieceSetMultimap(s adt.Store, r cid.Cid, outerBitwidth, innerBitwidth int) (*ProviderPieceSetMultimap, error) {
	m, err := adt.AsMap(s, r, outerBitwidth)
	if err != nil {
		return nil, err
	}
	return &ProviderPieceSetMultimap{mp: m, store: s, innerBitwidth: innerBitwidth}, nil
}

func MakeEmptyProviderPieceSetMultimap(s adt.Store, bitwidth int) (*ProviderPieceSetMultimap, error) {
	m, err := adt.MakeEmptyMap(s, bitwidth)
	if err != nil {
		return nil, err
	}
	return &ProviderPieceSetMultimap{mp: m, store: s, innerBitwidth: bitwidth}, nil
}

func StoreEmptyProviderPieceSetMultimap(s adt.Store, bitwidth int) (cid.Cid, error) {
	mm, err := MakeEmptyProviderPieceSetMultimap(s, bitwidth)
	if err != nil {
		return cid.Undef, err
	}
	return mm.Root()
}

func (mm *ProviderPieceSetMultimap) Root() (cid.Cid, error) {
	return mm.mp.Root()
}

func (mm *ProviderPieceSetMultimap) PutMany(provider address.Address, values map[cid.Cid]struct{}) error {
	if provider == address.Undef {
		return xerrors.New("undefined provider")
	}

	if len(values) == 0 {
		return xerrors.Errorf("empty values")
	}

	k := abi.AddrKey(provider)
	set, found, err := mm.get(k)
	if err != nil {
		return err
	}
	if !found {
		set, err = adt.MakeEmptySet(mm.store, mm.innerBitwidth)
		if err != nil {
			return err
		}
	}

	// Add to the set.
	for v := range values {
		if err = set.Put(abi.CidKey(v)); err != nil {
			return xerrors.Errorf("failed to add value to set %v: %w", provider, err)
		}
	}

	src, err := set.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush set root: %w", err)
	}
	// Store the new set root under key.
	newSetRoot := cbg.CborCid(src)
	return mm.mp.Put(k, &newSetRoot)
}

// Removes all values for a key.
func (mm *ProviderPieceSetMultimap) RemoveAll(provider address.Address) error {
	if _, err := mm.mp.TryDelete(abi.AddrKey(provider)); err != nil {
		return xerrors.Errorf("failed to delete set key %v: %w", provider, err)
	}
	return nil
}

func (mm *ProviderPieceSetMultimap) Remove(provider address.Address, v cid.Cid) error {
	k := abi.AddrKey(provider)
	set, found, err := mm.get(k)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if present, err := set.TryDelete(abi.CidKey(v)); err != nil {
		return xerrors.Errorf("failed to delete set value %v: %w", provider, err)
	} else if present {
		src, err := set.Root()
		if err != nil {
			return xerrors.Errorf("failed to flush set root: %w", err)
		}
		newSetRoot := cbg.CborCid(src)
		return mm.mp.Put(k, &newSetRoot)
	}
	return nil
}

func (mm *ProviderPieceSetMultimap) Has(provider address.Address, v cid.Cid) (bool, error) {
	k := abi.AddrKey(provider)
	set, found, err := mm.get(k)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	return set.Has(abi.CidKey(v))
}

func (mm *ProviderPieceSetMultimap) get(key abi.Keyer) (*adt.Set, bool, error) {
	var setRoot cbg.CborCid
	found, err := mm.mp.Get(key, &setRoot)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to load set key %v: %w", key, err)
	}
	var set *adt.Set
	if found {
		set, err = adt.AsSet(mm.store, cid.Cid(setRoot), mm.innerBitwidth)
		if err != nil {
			return nil, false, err
		}
	}
	return set, found, nil
}
