package govern

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
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

func ConstructState(emptyMapCid cid.Cid, supervisor address.Address) *State {

	return &State{
		Supervisor: supervisor,
		Governors:  emptyMapCid,
	}
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

	mp, err := adt.AsMap(store, out.CodeMethods)
	if err != nil {
		return false, errors.Wrapf(err, "failed to load CodeMethods")
	}
	var bf bitfield.BitField
	found, err = mp.Get(abi.CidKey(codeID), &bf)
	if err != nil {
		return false, errors.Wrapf(err, "failed to get priviledges")
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
		mp = adt.MakeEmptyMap(store)
	} else {
		mp, err = adt.AsMap(store, out.CodeMethods)
		if err != nil {
			return errors.Wrapf(err, "failed to load CodeMethods")
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
			return errors.Wrapf(err, "failed to get priviledges")
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
					return errors.Wrapf(err, "failed to subtract bitfields")
				}
			} else {
				bf, err = bitfield.MergeBitFields(bf, bitfield.NewFromSet(setBits))
				if err != nil {
					return errors.Wrapf(err, "failed to merge bitfields")
				}
			}
		}
		err = mp.Put(abi.CidKey(codeID), bf)
		if err != nil {
			return errors.Wrapf(err, "failed to put priviledges")
		}
	}

	out.CodeMethods, err = mp.Root()
	if err != nil {
		return errors.Wrapf(err, "failed to flush CodeMethods")
	}

	return governors.Put(abi.AddrKey(governor), &out)
}
