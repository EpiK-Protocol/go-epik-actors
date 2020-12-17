package expertfund

import (
	"github.com/filecoin-project/go-address"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	. "github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	cid "github.com/ipfs/go-cid"
	xerrors "golang.org/x/xerrors"
)

// State state of expert fund.
type State struct {
	// Information for all submit rdf data experts.
	Experts cid.Cid // Map, AMT[key]ExpertInfo

	PoolInfo cid.Cid

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

		TotalExpertDataSize: abi.PaddedPieceSize(0),
		TotalExpertReward:   abi.NewTokenAmount(0),
		DataStoreThreshold:  DefaultDataStoreThreshold,
	}
}

// LoadVestingFunds loads the vesting funds table from the store
func (st *State) LoadVestingFunds(store adt.Store, expert *ExpertInfo) (*VestingFunds, error) {
	var funds VestingFunds
	if err := store.Get(store.Context(), expert.VestingFunds, &funds); err != nil {
		return nil, xerrors.Errorf("failed to load vesting funds (%s): %w", expert.VestingFunds, err)
	}

	return &funds, nil
}

// SaveVestingFunds saves the vesting table to the store
func (st *State) SaveVestingFunds(store adt.Store, expert *ExpertInfo, funds *VestingFunds) error {
	c, err := store.Put(store.Context(), funds)
	if err != nil {
		return err
	}
	expert.VestingFunds = c
	return nil
}

// AddLockedFunds first vests and unlocks the vested funds AND then locks the given funds in the vesting table.
func (st *State) AddLockedFunds(rt Runtime, expert *ExpertInfo, vestingSum abi.TokenAmount) (vested abi.TokenAmount, err error) {
	AssertMsg(vestingSum.GreaterThanEqual(big.Zero()), "negative vesting sum %s", vestingSum)

	vestingFunds, err := st.LoadVestingFunds(adt.AsStore(rt), expert)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to load vesting funds: %w", err)
	}

	// unlock vested funds first
	amountUnlocked := vestingFunds.unlockVestedFunds(rt.CurrEpoch())
	expert.LockedFunds = big.Sub(expert.LockedFunds, amountUnlocked)
	Assert(expert.LockedFunds.GreaterThanEqual(big.Zero()))

	// add locked funds now
	vestingFunds.addLockedFunds(rt.CurrEpoch(), vestingSum, rt.CurrEpoch(), &RewardVestingSpec)
	expert.LockedFunds = big.Add(expert.LockedFunds, vestingSum)

	// save the updated vesting table state
	if err := st.SaveVestingFunds(adt.AsStore(rt), expert, vestingFunds); err != nil {
		return big.Zero(), xerrors.Errorf("failed to save vesting funds: %w", err)
	}

	return amountUnlocked, nil
}

// Deposit deposit expert data to fund.
func (st *State) Deposit(rt Runtime, fromAddr address.Address, size abi.PaddedPieceSize) error {
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
		emptyVestingFunds := ConstructVestingFunds()
		emptyVestingFundsCid := rt.StorePut(emptyVestingFunds)
		out.VestingFunds = emptyVestingFundsCid
	}
	if out.DataSize > 0 {
		pending := big.Mul(abi.NewTokenAmount(int64(out.DataSize)), pool.AccPerShare)
		pending = big.Div(pending, AccumulatedMultiplier)
		pending = big.Sub(pending, out.RewardDebt)
		unlocked, err := st.AddLockedFunds(rt, &out, pending)
		if err != nil {
			return err
		}
		out.UnlockedFunds = big.Add(out.UnlockedFunds, unlocked)
	}
	st.TotalExpertDataSize = st.TotalExpertDataSize + size

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
	st.TotalExpertDataSize += size
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
	_, err = experts.Get(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return err
	}
	if out.DataSize > 0 {
		pending := big.Mul(abi.NewTokenAmount(int64(out.DataSize)), pool.AccPerShare)
		pending = big.Div(pending, AccumulatedMultiplier)
		pending = big.Sub(pending, out.RewardDebt)
		unlocked, err := st.AddLockedFunds(rt, &out, pending)
		if err != nil {
			return err
		}
		out.UnlockedFunds = big.Add(out.UnlockedFunds, unlocked)
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
	if err := st.UpdatePool(rt); err != nil {
		return err
	}

	experts, err := adt.AsMap(adt.AsStore(rt), st.Experts)
	if err != nil {
		return err
	}
	var out ExpertInfo
	_, err = experts.Get(abi.AddrKey(expert), &out)
	if err != nil {
		return err
	}

	st.TotalExpertDataSize = st.TotalExpertDataSize - out.DataSize

	out.DataSize = 0
	out.RewardDebt = abi.NewTokenAmount(0)
	out.LockedFunds = abi.NewTokenAmount(0)

	err = experts.Put(abi.AddrKey(expert), &out)
	if err != nil {
		return err
	}
	if st.Experts, err = experts.Root(); err != nil {
		return err
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

	pool.LastRewardBlock = rt.CurrEpoch()
	reward := big.Sub(rt.CurrentBalance(), st.LastFundBalance)
	if reward.LessThan(big.Zero()) {
		return xerrors.Errorf("failed to settlement with balance error.")
	}
	accPerShare := big.Div(big.Mul(reward, AccumulatedMultiplier), abi.NewTokenAmount(int64(st.TotalExpertDataSize)))
	pool.AccPerShare = big.Add(pool.AccPerShare, accPerShare)
	if err = st.SavePoolInfo(rt, pool); err != nil {
		return err
	}
	st.LastFundBalance = rt.CurrentBalance()
	return nil
}
