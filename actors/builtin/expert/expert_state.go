package expert

import (
	addr "github.com/filecoin-project/go-address"
	cid "github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	xerrors "golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

// State of expert
type State struct {
	// Information not related to sectors.
	Info cid.Cid

	// Information for all submit rdf data.
	Datas cid.Cid // Map, AMT[key]DataOnChainInfo (sparse)
}

// ExpertInfo expert info
type ExpertInfo struct {

	// Account that owns this expert.
	// - Income and returned collateral are paid to this address.
	Owner addr.Address // Must be an ID-address.

	// Byte array representing a Libp2p identity that should be used when connecting to this miner.
	PeerId abi.PeerID

	// Slice of byte arrays representing Libp2p multi-addresses used for establishing a connection with this miner.
	Multiaddrs []abi.Multiaddrs
}

type DataOnChainInfo struct {
	PieceID    string
	PieceSize  abi.PaddedPieceSize
	Redundancy uint64
	Bounty     string
}

func ConstructExpertInfo(owner addr.Address, pid []byte, multiAddrs [][]byte) (*ExpertInfo, error) {
	return &ExpertInfo{
		Owner:      owner,
		PeerId:     pid,
		Multiaddrs: multiAddrs,
	}, nil
}

func ConstructState(info cid.Cid, emptyMapCid cid.Cid) *State {
	return &State{
		Info:  info,
		Datas: emptyMapCid,
	}
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
	pieces, err := adt.AsMap(store, st.Datas)
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
	datas, err := adt.AsMap(store, st.Datas)
	if err != nil {
		return err
	}

	if err := datas.Put(adt.StringKey(data.PieceID), data); err != nil {
		return errors.Wrapf(err, "failed to put data %v", data)
	}
	st.Datas, err = datas.Root()
	return err
}

func (st *State) GetData(store adt.Store, pieceID string) (*DataOnChainInfo, bool, error) {
	datas, err := adt.AsMap(store, st.Datas)
	if err != nil {
		return nil, false, err
	}

	var info DataOnChainInfo
	found, err := datas.Get(adt.StringKey(pieceID), &info)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to get data %v", pieceID)
	}
	return &info, found, nil
}

func (st *State) DeleteData(store adt.Store, pieceID string) error {
	datas, err := adt.AsMap(store, st.Datas)
	if err != nil {
		return err
	}
	err = datas.Delete(adt.StringKey(pieceID))
	if err != nil {
		return errors.Wrapf(err, "failed to delete data for %v", pieceID)
	}

	st.Datas, err = datas.Root()
	return err
}

func (st *State) ForEachData(store adt.Store, f func(*DataOnChainInfo)) error {
	datas, err := adt.AsMap(store, st.Datas)
	if err != nil {
		return err
	}
	var info DataOnChainInfo
	return datas.ForEach(&info, func(key string) error {
		f(&info)
		return nil
	})
	return nil
}
