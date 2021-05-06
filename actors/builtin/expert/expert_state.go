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

	ExpertState ExpertState

	ImplicatedTimes uint64
	CurrentVotes    abi.TokenAmount // Valid votes
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
		ExpertState:     state,
		ImplicatedTimes: 0,
		CurrentVotes:    abi.NewTokenAmount(0),
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

func (st *State) PutDatas(store adt.Store, infos ...*DataOnChainInfo) error {
	if len(infos) == 0 {
		return nil
	}

	datas, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}

	for _, info := range infos {
		if err := datas.Put(adt.StringKey(info.PieceID), info); err != nil {
			return xerrors.Errorf("failed to put data %s: %w", info.PieceID, err)
		}
	}
	st.Datas, err = datas.Root()
	return err
}

func (st *State) GetDatas(store adt.Store, mustPresent bool, pieceIDs ...cid.Cid) ([]*DataOnChainInfo, error) {
	datas, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, err
	}

	ret := make([]*DataOnChainInfo, 0, len(pieceIDs))
	for _, pieceID := range pieceIDs {
		var info DataOnChainInfo
		found, err := datas.Get(adt.StringKey(pieceID.String()), &info)
		if err != nil {
			return nil, xerrors.Errorf("failed to get data %v: %w", pieceID, err)
		}
		if mustPresent && !found {
			return nil, xerrors.Errorf("data not found %s", pieceID)
		}
		if found {
			ret = append(ret, &info)
		}
	}
	return ret, nil
}

// !! not used
func (st *State) DeleteData(store adt.Store, pieceID cid.Cid) error {
	datas, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	present, err := datas.TryDelete(adt.StringKey(pieceID.String()))
	if err != nil {
		return xerrors.Errorf("failed to delete data %s: %w", pieceID, err)
	}
	if present {
		st.DataCount--
		st.Datas, err = datas.Root()
		return err
	}
	return nil
}

func (st *State) VoteThreshold() abi.TokenAmount {
	return big.Add(ExpertVoteThreshold, big.Mul(big.NewIntUnsigned(st.ImplicatedTimes), ExpertVoteThresholdAddition))
}
