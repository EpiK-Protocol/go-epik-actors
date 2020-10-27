package expert

import (
	addr "github.com/filecoin-project/go-address"
	cid "github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
)

type AddrKey = adt.AddrKey

// State of expert
type State struct {
	Info ExpertInfo

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
	PieceID cid.Cid
}

func ConstructState(emptyMapCid cid.Cid, ownerAddr addr.Address,
	peerId abi.PeerID, multiaddrs []abi.Multiaddrs) *State {
	return &State{
		Info: ExpertInfo{
			Owner:      ownerAddr,
			PeerId:     peerId,
			Multiaddrs: multiaddrs,
		},
		Datas: emptyMapCid,
	}
}

func (st *State) HasDataID(store adt.Store, pieceID cid.Cid) (bool, error) {
	pieces, err := adt.AsMap(store, st.Datas)
	if err != nil {
		return false, err
	}

	var info DataOnChainInfo
	found, err := pieces.Get(adt.CidKey(pieceID), &info)
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

	if err := datas.Put(adt.CidKey(data.PieceID), data); err != nil {
		return errors.Wrapf(err, "failed to put data %v", data)
	}
	st.Datas, err = datas.Root()
	return err
}

func (st *State) GetData(store adt.Store, pieceID cid.Cid) (*DataOnChainInfo, bool, error) {
	datas, err := adt.AsMap(store, st.Datas)
	if err != nil {
		return nil, false, err
	}

	var info DataOnChainInfo
	found, err := datas.Get(adt.CidKey(pieceID), &info)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to get data %v", pieceID)
	}
	return &info, found, nil
}

func (st *State) DeleteData(store adt.Store, pieceID cid.Cid) error {
	datas, err := adt.AsMap(store, st.Datas)
	if err != nil {
		return err
	}
	err = datas.Delete(adt.CidKey(pieceID))
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
}
