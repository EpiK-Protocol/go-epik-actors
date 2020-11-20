package expertfund

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	cid "github.com/ipfs/go-cid"
)

// State state of expert fund.
type State struct {
	// Information for all submit rdf data experts.
	Experts cid.Cid // Map, AMT[key]ExpertInfo

	// TotalExpertDataSize total expert registered data size
	TotalExpertDataSize abi.PaddedPieceSize

	// TotalExpertReward total expert fund receive rewards
	TotalExpertReward abi.TokenAmount
}

// ExpertInfo info of expert registered data
type ExpertInfo struct {
	// DataSize total of expert data size
	DataSize abi.PaddedPieceSize

	// RewardDebt reward debt
	RewardDebt abi.TokenAmount
}

// ConstructState expert fund construct
func ConstructState(emptyMapCid cid.Cid) *State {
	return &State{
		Experts: emptyMapCid,

		TotalExpertDataSize: abi.PaddedPieceSize(0),
		TotalExpertReward:   abi.NewTokenAmount(0),
	}
}

// Deposit deposit expert data to fund.
func (st *State) Deposit(rt Runtime, fromAddr address.Address, size abi.PaddedPieceSize) error {
	experts, err := adt.AsMap(adt.AsStore(rt), st.Experts)
	if err != nil {
		return err
	}
	var out ExpertInfo
	_, err = experts.Get(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return err
	}
	out.DataSize += size
	err = experts.Put(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return err
	}
	if st.Experts, err = experts.Root(); err != nil {
		return err
	}
	st.TotalExpertDataSize += size
	return nil
}

// Claim claim expert fund.
func (st *State) Claim(rt Runtime, fromAddr address.Address, amount abi.TokenAmount) error {
	experts, err := adt.AsMap(adt.AsStore(rt), st.Experts)
	if err != nil {
		return err
	}
	var out ExpertInfo
	_, err = experts.Get(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return err
	}
	err = experts.Put(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return err
	}
	if st.Experts, err = experts.Root(); err != nil {
		return err
	}
	return nil
}

// Reset reset expert data.
func (st *State) Reset(rt Runtime, fromAddr address.Address) error {
	experts, err := adt.AsMap(adt.AsStore(rt), st.Experts)
	if err != nil {
		return err
	}
	var out ExpertInfo
	_, err = experts.Get(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return err
	}
	out.DataSize = 0
	err = experts.Put(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return err
	}
	if st.Experts, err = experts.Root(); err != nil {
		return err
	}
	return nil
}
