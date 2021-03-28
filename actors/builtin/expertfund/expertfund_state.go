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

	Datas cid.Cid // Map, HAMT[piece]ExpertAddress

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

type PoolInfo struct {
	// AccPerShare Accumulated EPK per share, times 1e12.
	AccPerShare abi.TokenAmount

	TotalExpertDataSize abi.PaddedPieceSize

	LastRewardBalance abi.TokenAmount
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

		DataStoreThreshold: DefaultDataStoreThreshold,
	}, nil
}

// MustPutAbsentData returns error if data already exists.
func (st *State) MustPutAbsentData(store adt.Store, pieceID string, expert addr.Address) error {
	datas, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	absent, err := datas.PutIfAbsent(adt.StringKey(pieceID), &expert)
	if err != nil {
		return xerrors.Errorf("failed to put data %s from expert %s: %w", pieceID, expert, err)
	}
	if !absent {
		return xerrors.Errorf("put duplicate data %s from expert %s", pieceID, expert)
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

// Deposit deposit expert data to fund.
func (st *State) Deposit(rt Runtime, fromAddr address.Address, originSize abi.PaddedPieceSize) error {
	// Note: Considering that audio files are larger than text files, it is not fair to text files, so take the square root of size
	sqrtSize := big.Zero().Sqrt(big.NewIntUnsigned(uint64(originSize)).Int)
	fixedSize := abi.PaddedPieceSize(sqrtSize.Uint64())

	store := adt.AsStore(rt)

	// update Pool
	pool, err := st.UpdatePool(rt)
	if err != nil {
		return err
	}

	// update ExpertInfo
	out, err := st.GetExpert(store, fromAddr)
	if err != nil {
		return err
	}
	if err := st.updateVestingFunds(rt, pool, out); err != nil {
		return err
	}

	pool.TotalExpertDataSize += fixedSize
	if err = st.SavePool(store, pool); err != nil {
		return err
	}

	out.DataSize += fixedSize
	out.RewardDebt = big.Div(
		big.Mul(
			big.NewIntUnsigned(uint64(out.DataSize)),
			pool.AccPerShare),
		AccumulatedMultiplier)
	return st.SetExpert(store, fromAddr, out, false)
}

// Claim claim expert fund.
func (st *State) Claim(rt Runtime, fromAddr address.Address, amount abi.TokenAmount) (abi.TokenAmount, error) {

	pool, err := st.UpdatePool(rt)
	if err != nil {
		return big.Zero(), err
	}

	store := adt.AsStore(rt)

	out, err := st.GetExpert(store, fromAddr)
	if err != nil {
		return big.Zero(), err
	}
	if err := st.updateVestingFunds(rt, pool, out); err != nil {
		return big.Zero(), err
	}

	// save expert
	actual := big.Min(out.UnlockedFunds, amount)
	out.UnlockedFunds = big.Sub(out.UnlockedFunds, actual)
	out.RewardDebt = big.Div(
		big.Mul(
			big.NewIntUnsigned(uint64(out.DataSize)),
			pool.AccPerShare),
		AccumulatedMultiplier)
	if err = st.SetExpert(store, fromAddr, out, false); err != nil {
		return big.Zero(), err
	}

	// save pool
	pool.LastRewardBalance = big.Sub(pool.LastRewardBalance, actual)
	if err = st.SavePool(store, pool); err != nil {
		return big.Zero(), err
	}

	return actual, nil
}

// Reset expert's contributions.
func (st *State) Reset(rt Runtime, expert addr.Address) (abi.TokenAmount, error) {

	pool, err := st.UpdatePool(rt)
	if err != nil {
		return big.Zero(), err
	}

	store := adt.AsStore(rt)

	out, err := st.GetExpert(store, expert)
	if err != nil {
		return big.Zero(), err
	}
	if err := st.updateVestingFunds(rt, pool, out); err != nil {
		return big.Zero(), err
	}

	pool.TotalExpertDataSize -= out.DataSize
	if err = st.SavePool(store, pool); err != nil {
		return big.Zero(), err
	}

	toBurn := out.LockedFunds
	out.DataSize = 0
	out.RewardDebt = abi.NewTokenAmount(0)
	out.LockedFunds = abi.NewTokenAmount(0)
	emptyMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return big.Zero(), err
	}
	out.VestingFunds = emptyMapCid
	if err = st.SetExpert(store, expert, out, false); err != nil {
		return big.Zero(), err
	}

	return toBurn, nil
}

func (st *State) updateVestingFunds(rt Runtime, pool *PoolInfo, out *ExpertInfo) error {
	pending := big.Mul(big.NewIntUnsigned(uint64(out.DataSize)), pool.AccPerShare)
	pending = big.Div(pending, AccumulatedMultiplier)
	if pending.LessThan(out.RewardDebt) {
		return xerrors.Errorf("unexpect: RewardDebt %s greater than pending %s", out.RewardDebt, pending)
	}
	pending = big.Sub(pending, out.RewardDebt)
	out.LockedFunds = big.Add(out.LockedFunds, pending)

	vestingFund, err := adt.AsMap(adt.AsStore(rt), out.VestingFunds, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load VestingFunds: %w", err)
	}

	currEpoch := rt.CurrEpoch()

	// add new pending value
	if !pending.IsZero() {
		k := abi.IntKey(int64(currEpoch))
		var old abi.TokenAmount
		found, err := vestingFund.Get(k, &old)
		if err != nil {
			return xerrors.Errorf("failed to get old vesting at epoch %d: %w", currEpoch, err)
		}
		if found {
			pending = big.Add(pending, old)
		}
		if err := vestingFund.Put(k, &pending); err != nil {
			return xerrors.Errorf("failed to put new vesting at epoch %d: %w", currEpoch, err)
		}
	}

	unlocked := abi.NewTokenAmount(0)
	// calc unlocked amounts
	var amount abi.TokenAmount
	err = vestingFund.ForEach(&amount, func(k string) error {
		epoch, err := abi.ParseIntKey(k)
		if err != nil {
			return xerrors.Errorf("failed to parse vestingFund key: %w", err)
		}
		if abi.ChainEpoch(epoch)+RewardVestingSpec.VestPeriod < currEpoch {
			unlocked = big.Add(unlocked, amount)
			return vestingFund.Delete(abi.IntKey(epoch))
		}
		return nil
	})
	if err != nil {
		return xerrors.Errorf("failed to iterate vestingFund: %w", err)
	}
	out.VestingFunds, err = vestingFund.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush VestingFunds: %w", err)
	}

	out.LockedFunds = big.Sub(out.LockedFunds, unlocked)
	out.UnlockedFunds = big.Add(out.UnlockedFunds, unlocked)
	return nil
}

func (st *State) SavePool(store adt.Store, pool *PoolInfo) error {
	c, err := store.Put(store.Context(), pool)
	if err == nil {
		st.PoolInfo = c
	}
	return err
}

func (st *State) GetPool(store adt.Store) (*PoolInfo, error) {
	var pool PoolInfo
	if err := store.Get(store.Context(), st.PoolInfo, &pool); err != nil {
		return nil, xerrors.Errorf("failed to get pool: %w", err)
	}
	return &pool, nil
}

// UpdatePool update pool.
func (st *State) UpdatePool(rt Runtime) (*PoolInfo, error) {
	pool, err := st.GetPool(adt.AsStore(rt))
	if err != nil {
		return nil, err
	}

	currBalance := rt.CurrentBalance()
	if pool.TotalExpertDataSize != 0 {
		reward := big.Sub(currBalance, pool.LastRewardBalance)
		if reward.LessThan(big.Zero()) {
			return nil, xerrors.Errorf("unexpected current balance less than last: %s, %s", currBalance, pool.LastRewardBalance)
		}
		accPerShare := big.Div(big.Mul(reward, AccumulatedMultiplier), big.NewIntUnsigned(uint64(pool.TotalExpertDataSize)))
		pool.AccPerShare = big.Add(pool.AccPerShare, accPerShare)
	}
	pool.LastRewardBalance = currBalance
	return pool, nil
}

func (st *State) GetExpert(store adt.Store, a addr.Address) (*ExpertInfo, error) {
	experts, err := adt.AsMap(store, st.Experts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to load experts: %w", err)
	}

	var out ExpertInfo
	found, err := experts.Get(abi.AddrKey(a), &out)
	if err != nil {
		return nil, xerrors.Errorf("failed to get expert for address %v from store %s: %w", a, st.Experts, err)
	}
	if !found {
		return nil, xerrors.Errorf("expert not found: %s", a)
	}
	return &out, nil
}

func (st *State) SetExpert(store adt.Store, ida addr.Address, expert *ExpertInfo, mustAbsent bool) error {
	experts, err := adt.AsMap(store, st.Experts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load experts: %w", err)
	}

	if mustAbsent {
		absent, err := experts.PutIfAbsent(abi.AddrKey(ida), expert)
		if err != nil {
			return xerrors.Errorf("failed to put absent expert %s: %w", ida, err)
		}
		if !absent {
			return xerrors.Errorf("expert already exists: %s", ida)
		}
	} else {
		if err = experts.Put(abi.AddrKey(ida), expert); err != nil {
			return xerrors.Errorf("failed to put expert %s: %w", ida, err)
		}
	}

	st.Experts, err = experts.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush experts: %w", err)
	}
	return nil
}

func (st *State) ListTrackedExperts(s adt.Store) ([]addr.Address, error) {
	tracked, err := adt.AsSet(s, st.TrackedExperts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to load tracked experts: %w", err)
	}

	var addrs []addr.Address
	err = tracked.ForEach(func(k string) error {
		a, err := addr.NewFromBytes([]byte(k))
		if err != nil {
			return err
		}
		addrs = append(addrs, a)
		return nil
	})
	if err != nil {
		return nil, xerrors.Errorf("failed to iterate traced expert: %w", err)
	}
	return addrs, nil
}

func (st *State) DeleteTrackedExperts(s adt.Store, addrs []addr.Address) error {
	if len(addrs) == 0 {
		return nil
	}

	tracked, err := adt.AsSet(s, st.TrackedExperts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load tracked experts: %w", err)
	}

	for _, adr := range addrs {
		_, err := tracked.TryDelete(abi.AddrKey(adr))
		if err != nil {
			return xerrors.Errorf("failed to delete tracked expert %s: %w", adr, err)
		}
	}

	st.TrackedExperts, err = tracked.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush tracked experts: %w", err)
	}
	return nil
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
