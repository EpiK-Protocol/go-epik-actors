package expertfund

import (
	"strconv"

	"github.com/filecoin-project/go-address"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	cid "github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	xerrors "golang.org/x/xerrors"
)

// State state of expert fund.
type State struct {
	// Information for all submit rdf data experts.
	Experts cid.Cid // Map, AMT[key]ExpertInfo

	PoolInfo cid.Cid

	Datas cid.Cid // Map, AMT[key]address

	// TotalExpertDataSize total expert registered data size
	TotalExpertDataSize abi.PaddedPieceSize

	// TotalExpertReward total expert fund receive rewards
	TotalExpertReward abi.TokenAmount

	LastFundBalance abi.TokenAmount

	DataStoreThreshold uint64
}

// ExpertInfo info of expert registered data
type ExpertInfo struct {
	// DataSize total of expert data size
	DataSize abi.PaddedPieceSize

	// RewardDebt reward debt
	RewardDebt abi.TokenAmount

	LockedFunds abi.TokenAmount // Total rewards and added funds locked in vesting table

	VestingFunds cid.Cid // VestingFunds (Vesting Funds schedule for the expert).

	UnlockedFunds abi.TokenAmount
}

// PoolInfo pool info
type PoolInfo struct {
	LastRewardBlock abi.ChainEpoch
	// AccPerShare Accumulated EPK per share, times 1e12.
	AccPerShare abi.TokenAmount
}

// ConstructState expert fund construct
func ConstructState(emptyMapCid cid.Cid, pool cid.Cid) *State {
	return &State{
		Experts:  emptyMapCid,
		PoolInfo: pool,
		Datas:    emptyMapCid,

		TotalExpertDataSize: abi.PaddedPieceSize(0),
		TotalExpertReward:   abi.NewTokenAmount(0),
		LastFundBalance:     abi.NewTokenAmount(0),
		DataStoreThreshold:  DefaultDataStoreThreshold,
	}
}

func (st *State) HasDataID(store adt.Store, pieceID string) (bool, error) {
	pieces, err := adt.AsMap(store, st.Datas)
	if err != nil {
		return false, err
	}

	var expert addr.Address
	found, err := pieces.Get(adt.StringKey(pieceID), &expert)
	if err != nil {
		return false, xerrors.Errorf("failed to get data %v: %w", pieceID, err)
	}
	return found, nil
}

func (st *State) PutData(store adt.Store, pieceID string, expert addr.Address) error {
	datas, err := adt.AsMap(store, st.Datas)
	if err != nil {
		return err
	}

	if err := datas.Put(adt.StringKey(pieceID), &expert); err != nil {
		return errors.Wrapf(err, "failed to put expert %v", expert)
	}
	st.Datas, err = datas.Root()
	return err
}

func (st *State) GetData(store adt.Store, pieceID string) (addr.Address, bool, error) {
	datas, err := adt.AsMap(store, st.Datas)
	if err != nil {
		return addr.Undef, false, err
	}

	var expert addr.Address
	found, err := datas.Get(adt.StringKey(pieceID), &expert)
	if err != nil {
		return addr.Undef, false, errors.Wrapf(err, "failed to get data %v", pieceID)
	}
	return expert, found, nil
}

// Deposit deposit expert data to fund.
func (st *State) Deposit(rt Runtime, fromAddr address.Address, size abi.PaddedPieceSize) error {
	st.TotalExpertDataSize += size
	if err := st.UpdatePool(rt); err != nil {
		return err
	}

	pool, err := st.GetPoolInfo(rt)
	if err != nil {
		return err
	}

	experts, err := adt.AsMap(adt.AsStore(rt), st.Experts)
	if err != nil {
		return err
	}
	var out ExpertInfo
	found, err := experts.Get(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return err
	}
	if !found {
		emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
		if err != nil {
			return err
		}
		out = ExpertInfo{
			DataSize:      0,
			VestingFunds:  emptyMap,
			RewardDebt:    abi.NewTokenAmount(0),
			LockedFunds:   abi.NewTokenAmount(0),
			UnlockedFunds: abi.NewTokenAmount(0),
		}
	}
	if err := st.updateVestingFunds(rt, pool, &out); err != nil {
		return err
	}

	out.DataSize += size
	debt := big.Mul(abi.NewTokenAmount(int64(out.DataSize)), pool.AccPerShare)
	out.RewardDebt = big.Div(debt, AccumulatedMultiplier)
	err = experts.Put(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return err
	}
	if st.Experts, err = experts.Root(); err != nil {
		return err
	}
	return nil
}

// Claim claim expert fund.
func (st *State) Claim(rt Runtime, fromAddr address.Address, amount abi.TokenAmount) error {
	if err := st.UpdatePool(rt); err != nil {
		return err
	}

	pool, err := st.GetPoolInfo(rt)
	if err != nil {
		return err
	}

	experts, err := adt.AsMap(adt.AsStore(rt), st.Experts)
	if err != nil {
		return err
	}
	var out ExpertInfo
	found, err := experts.Get(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return err
	}
	if !found {
		return xerrors.Errorf("expert not found")
	}

	if err := st.updateVestingFunds(rt, pool, &out); err != nil {
		return err
	}

	debt := big.Mul(abi.NewTokenAmount(int64(out.DataSize)), pool.AccPerShare)
	out.RewardDebt = big.Div(debt, AccumulatedMultiplier)

	if out.UnlockedFunds.LessThan(amount) {
		return xerrors.Errorf("insufficient unlocked funds")
	}
	out.UnlockedFunds = big.Sub(out.UnlockedFunds, amount)

	err = experts.Put(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return err
	}
	if st.Experts, err = experts.Root(); err != nil {
		return err
	}
	return nil
}

// UpdateExpert update expert.
func (st *State) Reset(rt Runtime, expert addr.Address) error {
	pool, err := st.GetPoolInfo(rt)
	if err != nil {
		return err
	}

	experts, err := adt.AsMap(adt.AsStore(rt), st.Experts)
	if err != nil {
		return err
	}
	var out ExpertInfo
	found, err := experts.Get(abi.AddrKey(expert), &out)
	if err != nil {
		return err
	}

	if !found {
		return xerrors.Errorf("expert not found")
	}

	st.TotalExpertDataSize = st.TotalExpertDataSize - out.DataSize

	if err := st.UpdatePool(rt); err != nil {
		return err
	}

	out.DataSize = 0
	out.RewardDebt = abi.NewTokenAmount(0)
	if err := st.updateVestingFunds(rt, pool, &out); err != nil {
		return err
	}
	out.LockedFunds = abi.NewTokenAmount(0)
	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	if err != nil {
		return err
	}
	out.VestingFunds = emptyMap

	err = experts.Put(abi.AddrKey(expert), &out)
	if err != nil {
		return err
	}
	if st.Experts, err = experts.Root(); err != nil {
		return err
	}
	return nil
}

// updateVestingFunds update vest
func (st *State) updateVestingFunds(rt Runtime, pool *PoolInfo, out *ExpertInfo) error {
	if out.DataSize > 0 {
		pending := big.Mul(abi.NewTokenAmount(int64(out.DataSize)), pool.AccPerShare)
		pending = big.Div(pending, AccumulatedMultiplier)
		pending = big.Sub(pending, out.RewardDebt)
		out.LockedFunds = big.Add(out.LockedFunds, pending)

		vestingFund, err := adt.AsMap(adt.AsStore(rt), out.VestingFunds)
		if err != nil {
			return err
		}
		if err := vestingFund.Put(abi.IntKey(int64(rt.CurrEpoch())), &pending); err != nil {
			return err
		}

		keys, err := vestingFund.CollectKeys()
		if err != nil {
			return err
		}
		unlocked := abi.NewTokenAmount(0)
		for _, key := range keys {
			epoch, err := strconv.ParseInt(key, 10, 64)
			if err != nil {
				return err
			}

			var amount abi.TokenAmount
			if _, err := vestingFund.Get(abi.IntKey(epoch), &amount); err != nil {
				return err
			}
			if epoch+int64(RewardVestingSpec.VestPeriod) < int64(rt.CurrEpoch()) {
				unlocked = big.Add(unlocked, amount)
				if err := vestingFund.Delete(abi.IntKey(epoch)); err != nil {
					return err
				}
			}
		}

		out.LockedFunds = big.Sub(out.LockedFunds, unlocked)
		out.UnlockedFunds = big.Add(out.UnlockedFunds, unlocked)
	}
	return nil
}

// GetPoolInfo get pool info
func (st *State) GetPoolInfo(rt Runtime) (*PoolInfo, error) {
	store := adt.AsStore(rt)
	var info PoolInfo
	if err := store.Get(store.Context(), st.PoolInfo, &info); err != nil {
		return nil, xerrors.Errorf("failed to get pool info %w", err)
	}
	return &info, nil
}

// SavePoolInfo save info
func (st *State) SavePoolInfo(rt Runtime, info *PoolInfo) error {
	store := adt.AsStore(rt)
	c, err := store.Put(store.Context(), info)
	if err != nil {
		return err
	}
	st.PoolInfo = c
	return nil
}

// UpdatePool update pool.
func (st *State) UpdatePool(rt Runtime) error {

	pool, err := st.GetPoolInfo(rt)
	if err != nil {
		return err
	}

	if st.TotalExpertDataSize == 0 {
		pool.LastRewardBlock = rt.CurrEpoch()
		if err = st.SavePoolInfo(rt, pool); err != nil {
			return err
		}
		return nil
	}

	reward := big.Sub(rt.CurrentBalance(), st.LastFundBalance)
	if reward.LessThan(big.Zero()) {
		return xerrors.Errorf("failed to settlement with balance error.")
	}
	accPerShare := big.Div(big.Mul(reward, AccumulatedMultiplier), abi.NewTokenAmount(int64(st.TotalExpertDataSize)))
	pool.AccPerShare = big.Add(pool.AccPerShare, accPerShare)
	pool.LastRewardBlock = rt.CurrEpoch()
	if err = st.SavePoolInfo(rt, pool); err != nil {
		return err
	}
	st.LastFundBalance = rt.CurrentBalance()
	return nil
}
