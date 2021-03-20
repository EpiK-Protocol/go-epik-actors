package expert

import (
	addr "github.com/filecoin-project/go-address"
	cid "github.com/ipfs/go-cid"
	xerrors "golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

// State of expert
type State struct {
	// Information not related to sectors.
	Info cid.Cid

	// Information for all submit rdf data.
	Datas cid.Cid // Map, HAMT[pieceCid]DataOnChainInfo (sparse)

	DataCount uint64

	// LostEpoch record expert votes <  epoch or blocked epoch
	LostEpoch abi.ChainEpoch

	// Status of expert
	Status ExpertState

	ImplicatedTimes uint64
}

// ExpertInfo expert info
type ExpertInfo struct {

	// Account that owns this expert.
	// - Income and returned collateral are paid to this address.
	Owner addr.Address // Must be an ID-address.

	// Type expert type
	Type builtin.ExpertType

	// ApplicationHash expert application hash
	ApplicationHash string

	// Proposer of expert
	Proposer addr.Address

	// Only for owner change by governor
	ApplyNewOwner      addr.Address
	ApplyNewOwnerEpoch abi.ChainEpoch
}

type DataOnChainInfo struct {
	RootID     string
	PieceID    string
	PieceSize  abi.PaddedPieceSize
	Redundancy uint64
}

func ConstructState(store adt.Store, info cid.Cid, state ExpertState) (*State, error) {
	emptyMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty map: %w", err)
	}

	return &State{
		Info:            info,
		Datas:           emptyMapCid,
		LostEpoch:       NoLostEpoch,
		Status:          state,
		ImplicatedTimes: 0,
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

// !! not used
func (st *State) DeleteData(store adt.Store, pieceID string) error {
	datas, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	err = datas.Delete(adt.StringKey(pieceID))
	if err != nil {
		return xerrors.Errorf("failed to delete data for %s: %w", pieceID, err)
	}
	st.DataCount--
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

func (st *State) VoteThreshold() abi.TokenAmount {
	return big.Add(ExpertVoteThreshold, big.Mul(big.NewIntUnsigned(st.ImplicatedTimes), ExpertVoteThresholdAddition))
}
