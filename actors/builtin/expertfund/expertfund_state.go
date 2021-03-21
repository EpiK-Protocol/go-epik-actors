package expertfund

import (
	"math"
	"strconv"

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

	Datas cid.Cid // Map, HAMT[key]address

	// TotalExpertDataSize total expert registered data size
	TotalExpertDataSize abi.PaddedPieceSize

	// TotalExpertReward total expert fund receive rewards
	TotalExpertReward abi.TokenAmount // TODO: remove

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
	fixedSize := abi.PaddedPieceSize(math.Sqrt(float64(originSize)))
	st.TotalExpertDataSize += fixedSize
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
		return xerrors.Errorf("expert not found: %s", fromAddr)
	}

	if err := st.updateVestingFunds(rt, pool, &out); err != nil {
		return err
	}

	out.DataSize += fixedSize
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
func (st *State) Claim(rt Runtime, fromAddr address.Address, amount abi.TokenAmount) (abi.TokenAmount, error) {
	if err := st.UpdatePool(rt); err != nil {
		return big.Zero(), err
	}

	pool, err := st.GetPoolInfo(rt)
	if err != nil {
		return big.Zero(), err
	}

	experts, err := adt.AsMap(adt.AsStore(rt), st.Experts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return big.Zero(), err
	}
	var out ExpertInfo
	found, err := experts.Get(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return big.Zero(), err
	}

	if !found {
		return big.Zero(), xerrors.Errorf("expert not found: %s", fromAddr)
	}

	if err := st.updateVestingFunds(rt, pool, &out); err != nil {
		return big.Zero(), err
	}

	debt := big.Mul(abi.NewTokenAmount(int64(out.DataSize)), pool.AccPerShare)
	out.RewardDebt = big.Div(debt, AccumulatedMultiplier)

	actual := big.Min(out.UnlockedFunds, amount)
	out.UnlockedFunds = big.Sub(out.UnlockedFunds, actual)

	err = experts.Put(abi.AddrKey(fromAddr), &out)
	if err != nil {
		return big.Zero(), err
	}
	if st.Experts, err = experts.Root(); err != nil {
		return big.Zero(), err
	}
	st.LastFundBalance = big.Sub(st.LastFundBalance, actual)
	return actual, nil
}

// UpdateExpert update expert.
func (st *State) Reset(rt Runtime, expert addr.Address) error {
	pool, err := st.GetPoolInfo(rt)
	if err != nil {
		return err
	}

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
		return xerrors.Errorf("expert not found: %s", expert)
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
	emptyMapCid, err := adt.StoreEmptyMap(adt.AsStore(rt), builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	out.VestingFunds = emptyMapCid

	err = experts.Put(k, &out)
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

		vestingFund, err := adt.AsMap(adt.AsStore(rt), out.VestingFunds, builtin.DefaultHamtBitwidth)
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

func (st *State) GetExpert(s adt.Store, a addr.Address) (*ExpertInfo, error) {
	experts, err := adt.AsMap(s, st.Experts, builtin.DefaultHamtBitwidth)
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
