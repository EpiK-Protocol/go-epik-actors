package market

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-hamt-ipld/v2"
	"github.com/filecoin-project/go-state-types/abi"
	cid "github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type IndexMultimap struct {
	mp    *adt.Map
	store adt.Store
}

// Interprets a store as a HAMT-based map of HAMT-based maps with root `r`.
func AsIndexMultimap(s adt.Store, r cid.Cid) (*IndexMultimap, error) {
	m, err := adt.AsMap(s, r)
	if err != nil {
		return nil, err
	}
	return &IndexMultimap{mp: m, store: s}, nil
}

// Creates a new map backed by an empty HAMT and flushes it to the store.
func MakeEmptyIndexMultimap(s adt.Store) *IndexMultimap {
	m := adt.MakeEmptyMap(s)
	return &IndexMultimap{m, s}
}

// Returns the root cid of the underlying HAMT.
func (mm *IndexMultimap) Root() (cid.Cid, error) {
	return mm.mp.Root()
}

// Assumes index is non-duplicate
func (mm *IndexMultimap) Put(epoch abi.ChainEpoch, providerIndexes map[address.Address][]DataIndex) error {
	if len(providerIndexes) == 0 {
		return nil
	}

	epochKey := abi.UIntKey(uint64(epoch))
	pimap, found, err := mm.get(epochKey)
	if err != nil {
		return err
	}
	if !found {
		pimap = adt.MakeEmptyMap(mm.store)
	}

	for provider, indexes := range providerIndexes {
		if len(indexes) == 0 {
			continue
		}
		k := abi.AddrKey(provider)
		var arrRoot cbg.CborCid
		found, err := pimap.Get(k, &arrRoot)
		if err != nil {
			return err
		}
		var arr *adt.Array
		if found {
			arr, err = adt.AsArray(mm.store, cid.Cid(arrRoot))
			if err != nil {
				return err
			}
		} else {
			arr = adt.MakeEmptyArray(mm.store)
		}

		for _, index := range indexes {
			cp := index
			err := arr.AppendContinuous(&cp)
			if err != nil {
				return err
			}
		}

		nr, err := arr.Root()
		if err != nil {
			return xerrors.Errorf("failed to flush array root: %w", err)
		}
		nid := cbg.CborCid(nr)
		err = pimap.Put(k, &nid)
		if err != nil {
			return err
		}
	}

	nr, err := pimap.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush map root for epoch %d: %w", epoch, err)
	}
	newMapRoot := cbg.CborCid(nr)
	err = mm.mp.Put(epochKey, &newMapRoot)
	if err != nil {
		return errors.Wrapf(err, "failed to store index")
	}
	return nil
}

// Removes all values for a piece.
func (mm *IndexMultimap) RemoveAll(key abi.ChainEpoch) error {
	k := abi.UIntKey(uint64(key))
	found, err := mm.mp.Has(k)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	err = mm.mp.Delete(k)
	if err != nil && !xerrors.Is(err, hamt.ErrNotFound) {
		return xerrors.Errorf("failed to delete map key %v: %w", key, err)
	}
	return nil
}

// Iterates all entries for a key, iteration halts if the function returns an error.
func (mm *IndexMultimap) ForEach(epoch abi.ChainEpoch, fn func(provder address.Address, index DataIndex) error) error {
	mp, found, err := mm.get(abi.UIntKey(uint64(epoch)))
	if err != nil {
		return err
	}
	if found {
		var arrRoot cbg.CborCid
		return mp.ForEach(&arrRoot, func(k string) error {
			provider, err := address.NewFromBytes([]byte(k))
			if err != nil {
				return err
			}

			arr, err := adt.AsArray(mm.store, cid.Cid(arrRoot))
			if err != nil {
				return err
			}

			var index DataIndex
			return arr.ForEach(&index, func(i int64) error {
				return fn(provider, index)
			})
		})
	}
	return nil
}

func (mm *IndexMultimap) get(key abi.Keyer) (*adt.Map, bool, error) {
	var imRoot cbg.CborCid
	found, err := mm.mp.Get(key, &imRoot)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to load map key %v", key)
	}
	var m *adt.Map
	if found {
		m, err = adt.AsMap(mm.store, cid.Cid(imRoot))
		if err != nil {
			return nil, false, err
		}
	}
	return m, found, nil
}
