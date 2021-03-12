package expertfund

import (
	"github.com/filecoin-project/go-address"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	cid "github.com/ipfs/go-cid"
	xerrors "golang.org/x/xerrors"
)

// State state of expert fund.
type State struct {
	// Information for all submit rdf data experts.
	Experts cid.Cid // Map, HAMT[expert]ExpertInfo

	ExpertsCount uint64

	TrackedExperts cid.Cid // Set[expert]

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
func ConstructState(store adt.Store, pool cid.Cid) (*State, error) {
	emptyExpertsMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty map: %w", err)
	}

	emptyDatasMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty map: %w", err)
	}

	emptyTrackedExpertsSetCid, err := adt.StoreEmptySet(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty map: %w", err)
	}

	return &State{
		Experts:        emptyExpertsMapCid,
		PoolInfo:       pool,
		Datas:          emptyDatasMapCid,
		TrackedExperts: emptyTrackedExpertsSetCid,

		TotalExpertDataSize: abi.PaddedPieceSize(0),
		TotalExpertReward:   abi.NewTokenAmount(0),
		LastFundBalance:     abi.NewTokenAmount(0),
		DataStoreThreshold:  DefaultDataStoreThreshold,
	}, nil
}

func (st *State) HasDataID(store adt.Store, pieceID string) (bool, error) {
	pieces, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
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
	datas, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}

	if err := datas.Put(adt.StringKey(pieceID), &expert); err != nil {
		return xerrors.Errorf("failed to put expert %v: %w", expert, err)
	}
	st.Datas, err = datas.Root()
	return err
}

func (st *State) GetData(store adt.Store, pieceID string) (addr.Address, bool, error) {
	datas, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return addr.Undef, false, err
	}

	var expert addr.Address
	found, err := datas.Get(adt.StringKey(pieceID), &expert)
	if err != nil {
		return addr.Undef, false, xerrors.Errorf("failed to get data %v: %w", pieceID, err)
	}
	return expert, found, nil
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
	if vestingSum.LessThan(big.Zero()) {
		return big.Zero(), xerrors.Errorf("negative vesting sum %s", vestingSum)
	}

	vestingFunds, err := st.LoadVestingFunds(adt.AsStore(rt), expert)
	if err != nil {
		return big.Zero(), xerrors.Errorf("failed to load vesting funds: %w", err)
	}

	// unlock vested funds first
	amountUnlocked := vestingFunds.unlockVestedFunds(rt.CurrEpoch())
	expert.LockedFunds = big.Sub(expert.LockedFunds, amountUnlocked)
	if expert.LockedFunds.LessThan(big.Zero()) {
		return big.Zero(), xerrors.Errorf("negative locked funds %v after subtracting %v", expert.LockedFunds, amountUnlocked)
	}

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
	st.TotalExpertDataSize += size
	if err := st.UpdatePool(rt); err != nil {
		return err
	}

	pool, err := st.GetPoolInfo(rt)
	if err != nil {
		return err
	}

	experts, err := adt.AsMap(adt.AsStore(rt), st.Experts, builtin.DefaultHamtBitwidth)
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

	experts, err := adt.AsMap(adt.AsStore(rt), st.Experts, builtin.DefaultHamtBitwidth)
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
	k := abi.AddrKey(expert)
	experts, err := adt.AsMap(adt.AsStore(rt), st.Experts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	var out ExpertInfo
	found, err := experts.Get(k, &out)
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
	out.LockedFunds = abi.NewTokenAmount(0)

	err = experts.Put(k, &out)
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

func (st *State) getExpert(s adt.Store, a addr.Address) (*ExpertInfo, bool, error) {
	experts, err := adt.AsMap(s, st.Experts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, false, err
	}

	var out ExpertInfo
	found, err := experts.Get(abi.AddrKey(a), &out)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to get expert for address %v from store %s: %w", a, st.Experts, err)
	}
	if !found {
		return nil, false, nil
	}
	return &out, true, nil
}

func (st *State) setExpert(store adt.Store, ida addr.Address, expert *ExpertInfo) error {
	hm, err := adt.AsMap(store, st.Experts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	k := abi.AddrKey(ida)
	has, err := hm.Has(k)
	if err != nil {
		return err
	}
	if has {
		// should never happen
		return xerrors.Errorf("expert %s already exist", ida)
	}

	if err = hm.Put(k, expert); err != nil {
		return xerrors.Errorf("failed to put expert with address %s expert %v in store %s: %w", ida, expert, st.Experts, err)
	}

	st.Experts, err = hm.Root()
	return err
}

func (st *State) ForEachExpert(store adt.Store, f func(*ExpertInfo)) error {
	experts, err := adt.AsMap(store, st.Experts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	var info ExpertInfo
	return experts.ForEach(&info, func(key string) error {
		f(&info)
		return nil
	})
}

func (st *State) expertActors(store adt.Store) ([]addr.Address, error) {
	experts, err := adt.AsMap(store, st.Experts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, err
	}
	addrs, err := experts.CollectKeys()
	if err != nil {
		return nil, err
	}
	var actors []addr.Address
	for _, a := range addrs {
		addr, err := addr.NewFromBytes([]byte(a))
		if err != nil {
			return nil, err
		}
		actors = append(actors, addr)
	}
	return actors, nil
}
