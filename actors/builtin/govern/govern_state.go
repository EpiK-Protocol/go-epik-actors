package govern

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/ipfs/go-cid"
	xerrors "golang.org/x/xerrors"
)

type State struct {

	// Supervisor is in charge of the management of authorization policies.
	Supervisor address.Address

	// Granted authorities of each governor by Supervisor
	Governors cid.Cid // Map, HAMT[address]GrantedAuthorities, ID-Address
}

type GrantedAuthorities struct {
	// Granted methods of actor code
	CodeMethods cid.Cid // Map, HAMT[actor codeID]BitField
}

func ConstructState(store adt.Store, supervisor address.Address) (*State, error) {

	if supervisor.Protocol() != address.ID {
		return nil, xerrors.New("supervisor address must be an ID address")
	}

	emptyMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty map: %w", err)
	}

	return &State{
		Supervisor: supervisor,
		Governors:  emptyMapCid,
	}, nil
}

func (st *State) IsGranted(store adt.Store, governors *adt.Map, governor address.Address, codeID cid.Cid, method abi.MethodNum) (bool, error) {
	var out GrantedAuthorities
	found, err := governors.Get(abi.AddrKey(governor), &out)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	mp, err := adt.AsMap(store, out.CodeMethods, builtin.DefaultHamtBitwidth)
	if err != nil {
		return false, xerrors.Errorf("failed to load CodeMethods: %w", err)
	}
	var bf bitfield.BitField
	found, err = mp.Get(abi.CidKey(codeID), &bf)
	if err != nil {
		return false, xerrors.Errorf("failed to get priviledges: %w", err)
	}
	if !found {
		return false, nil
	}

	return util.BitFieldContainsAll(bf, bitfield.NewFromSet([]uint64{uint64(method)}))
}

func (st *State) grantOrRevoke(store adt.Store, governors *adt.Map, governor address.Address,
	targetCodeMethods map[cid.Cid][]abi.MethodNum, grant bool) error {

	if len(targetCodeMethods) == 0 {
		return nil
	}

	var out GrantedAuthorities
	found, err := governors.Get(abi.AddrKey(governor), &out)
	if err != nil {
		return err
	}
	var mp *adt.Map
	if !found {
		if !grant { // do nothing for revoke
			return nil
		}
		mp, err = adt.MakeEmptyMap(store, builtin.DefaultHamtBitwidth)
		if err != nil {
			return xerrors.Errorf("failed to create empty map: %w", err)
		}
	} else {
		mp, err = adt.AsMap(store, out.CodeMethods, builtin.DefaultHamtBitwidth)
		if err != nil {
			return xerrors.Errorf("failed to load CodeMethods: %w", err)
		}
	}

	for codeID, methods := range targetCodeMethods {
		if len(methods) == 0 {
			continue
		}

		setBits := make([]uint64, 0, len(methods))
		for _, method := range methods {
			setBits = append(setBits, uint64(method))
		}

		var bf bitfield.BitField
		found, err = mp.Get(abi.CidKey(codeID), &bf)
		if err != nil {
			return xerrors.Errorf("failed to get priviledges: %w", err)
		}
		if !found {
			if !grant { // do nothing for revoke
				continue
			}
			bf = bitfield.NewFromSet(setBits)
		} else {
			if !grant {
				bf, err = bitfield.SubtractBitField(bf, bitfield.NewFromSet(setBits))
				if err != nil {
					return xerrors.Errorf("failed to subtract bitfields: %w", err)
				}
				empty, err := bf.IsEmpty()
				if err != nil {
					return xerrors.Errorf("failed to check bitfield empty(revoke): %w", err)
				}
				if empty {
					err = mp.Delete(abi.CidKey(codeID))
					if err != nil {
						return xerrors.Errorf("failed to delete empty bitfield(revoke): %w", err)
					}
					continue
				}
			} else {
				bf, err = bitfield.MergeBitFields(bf, bitfield.NewFromSet(setBits))
				if err != nil {
					return xerrors.Errorf("failed to merge bitfields: %w", err)
				}
			}
		}
		err = mp.Put(abi.CidKey(codeID), bf)
		if err != nil {
			return xerrors.Errorf("failed to put priviledges: %w", err)
		}
	}
	keys, err := mp.CollectKeys()
	if err != nil {
		return xerrors.Errorf("failed to collect keys: %w", err)
	}
	if len(keys) == 0 {
		return governors.Delete(abi.AddrKey(governor))
	} else {
		out.CodeMethods, err = mp.Root()
		if err != nil {
			return xerrors.Errorf("failed to flush CodeMethods: %w", err)
		}
		return governors.Put(abi.AddrKey(governor), &out)
	}
}
