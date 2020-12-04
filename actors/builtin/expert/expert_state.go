package expert

import (
	addr "github.com/filecoin-project/go-address"
	cid "github.com/ipfs/go-cid"
	xerrors "golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

// State of expert
type State struct {
	// Information not related to sectors.
	Info cid.Cid

	// Information for all submit rdf data.
	Datas cid.Cid // Map, AMT[key]DataOnChainInfo (sparse)

	// VoteAmount expert vote amount
	VoteAmount abi.TokenAmount

	// LostEpoch record expert votes <  epoch
	LostEpoch abi.ChainEpoch

	// Status of expert
	Status ExpertState

	// OwnerChange owner change info
	OwnerChange cid.Cid
}

// PendingOwnerChange pending owner change
type PendingOwnerChange struct {
	ApplyEpoch abi.ChainEpoch
	ApplyOwner addr.Address
}

// ExpertInfo expert info
type ExpertInfo struct {

	// Account that owns this expert.
	// - Income and returned collateral are paid to this address.
	Owner addr.Address // Must be an ID-address.

	// Type expert type
	Type ExpertType

	// ApplicationHash expert application hash
	ApplicationHash string

	// Proposer of expert
	Proposer addr.Address
}

type DataOnChainInfo struct {
	RootID     string
	PieceID    string
	PieceSize  abi.PaddedPieceSize
	Redundancy uint64
}

func ConstructExpertInfo(owner addr.Address, eType ExpertType, aHash string) (*ExpertInfo, error) {
	return &ExpertInfo{
		Owner:           owner,
		Type:            eType,
		ApplicationHash: aHash,
		Proposer:        owner,
	}, nil
}

func ConstructState(store adt.Store, info cid.Cid, state ExpertState, emptyChange cid.Cid) (*State, error) {
	emptyMapCid, err := adt.MakeEmptyMap(store, builtin.DefaultHamtBitwidth).Root()
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty map: %w", err)
	}

	return &State{
		Info:        info,
		Datas:       emptyMapCid,
		VoteAmount:  abi.NewTokenAmount(0),
		LostEpoch:   abi.ChainEpoch(-1),
		Status:      state,
		OwnerChange: emptyChange,
	}, nil
}

func (st *State) GetInfo(store adt.Store) (*ExpertInfo, error) {
	var info ExpertInfo
	if err := store.Get(store.Context(), st.Info, &info); err != nil {
		return nil, xerrors.Errorf("failed to get expert info %w", err)
	}
	return &info, nil
}

func (st *State) SaveInfo(store adt.Store, info *ExpertInfo) error {
	c, err := store.Put(store.Context(), info)
	if err != nil {
		return err
	}
	st.Info = c
	return nil
}

func (st *State) HasDataID(store adt.Store, pieceID string) (bool, error) {
	pieces, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return false, err
	}

	var info DataOnChainInfo
	found, err := pieces.Get(adt.StringKey(pieceID), &info)
	if err != nil {
		return false, xerrors.Errorf("failed to get data %v: %w", pieceID, err)
	}
	return found, nil
}

func (st *State) PutData(store adt.Store, data *DataOnChainInfo) error {
	datas, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}

	if err := datas.Put(adt.StringKey(data.PieceID), data); err != nil {
		return xerrors.Errorf("failed to put data %v: %w", data, err)
	}
	st.Datas, err = datas.Root()
	return err
}

func (st *State) GetData(store adt.Store, pieceID string) (*DataOnChainInfo, bool, error) {
	datas, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, false, err
	}

	var info DataOnChainInfo
	found, err := datas.Get(adt.StringKey(pieceID), &info)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to get data %v: %w", pieceID, err)
	}
	return &info, found, nil
}

func (st *State) DeleteData(store adt.Store, pieceID string) error {
	datas, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	err = datas.Delete(adt.StringKey(pieceID))
	if err != nil {
		return xerrors.Errorf("failed to delete data for %v: %w", pieceID, err)
	}

	st.Datas, err = datas.Root()
	return err
}

func (st *State) ForEachData(store adt.Store, f func(*DataOnChainInfo)) error {
	datas, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	var info DataOnChainInfo
	return datas.ForEach(&info, func(key string) error {
		f(&info)
		return nil
	})
}

func (st *State) GetOwnerChange(store adt.Store) (*PendingOwnerChange, error) {

	var change PendingOwnerChange
	if err := store.Get(store.Context(), st.OwnerChange, &change); err != nil {
		return nil, xerrors.Errorf("failed to get owner change %w", err)
	}
	return &change, nil
}

func (st *State) ApplyOwnerChange(store adt.Store, currEpoch abi.ChainEpoch, applyOwner addr.Address) error {
	change := &PendingOwnerChange{
		ApplyEpoch: currEpoch,
		ApplyOwner: applyOwner,
	}
	c, err := store.Put(store.Context(), change)
	if err != nil {
		return err
	}
	st.OwnerChange = c
	return nil
}

func (st *State) AutoUpdateOwnerChange(store adt.Store, currEpoch abi.ChainEpoch) error {
	info, err := st.GetInfo(store)
	if err != nil {
		return err
	}

	change, err := st.GetOwnerChange(store)
	if err != nil {
		return err
	}
	if info.Owner != change.ApplyOwner &&
		change.ApplyEpoch > 0 &&
		(currEpoch-change.ApplyEpoch) >= ExpertVoteCheckPeriod {
		info.Owner = change.ApplyOwner
		if err := st.SaveInfo(store, info); err != nil {
			return err
		}
	}
	return nil
}

func (st *State) Validate(strore adt.Store, currEpoch abi.ChainEpoch) error {
	switch st.Status {
	case ExpertStateNormal:
		info, err := st.GetInfo(strore)
		if err != nil {
			return err
		}
		if info.Type != ExpertFoundation {
			if st.VoteAmount.LessThan(ExpertVoteThreshold) {
				if st.LostEpoch < 0 {
					return xerrors.Errorf("failed to vaildate expert with below vote:%w", st.VoteAmount)
				} else if (st.LostEpoch + ExpertVoteCheckPeriod) < currEpoch {
					return xerrors.Errorf("failed to vaildate expert with lost vote:%w", st.VoteAmount)
				}
			}
		}
	case ExpertStateImplicated:
		if st.VoteAmount.LessThan(ExpertVoteThresholdAddition) {
			if st.LostEpoch < 0 {
				return xerrors.Errorf("failed to vaildate expert with below vote:%w", st.VoteAmount)
			} else if (st.LostEpoch + ExpertVoteCheckPeriod) < currEpoch {
				return xerrors.Errorf("failed to vaildate expert with lost vote:%w", st.VoteAmount)
			}
		}
	default:
		return xerrors.Errorf("failed to validate expert status: %d", st.Status)
	}

	return nil
}
