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

	DisqualifiedExperts cid.Cid // MAP, HAMT[expert]DisqualifiedExpertInfo

	PoolInfo cid.Cid

	PieceInfos cid.Cid // Map, HAMT[PieceCID]PieceInfo

	DataStoreThreshold uint64

	// expert not foundation has daily data register size threshold
	DailyImportSizeThreshold uint64
}

// ExpertInfo info of expert registered data
type ExpertInfo struct {
	// DataSize total deposited data size of expert
	DataSize abi.PaddedPieceSize

	Active bool

	// RewardDebt reward debt
	RewardDebt abi.TokenAmount

	LockedFunds abi.TokenAmount // Total rewards and added funds locked in vesting table

	VestingFunds cid.Cid // VestingFunds (Vesting Funds schedule for the expert).

	UnlockedFunds abi.TokenAmount
}

type DisqualifiedExpertInfo struct {
	DisqualifiedAt abi.ChainEpoch
}

type PieceInfo struct {
	Expert           addr.Address
	DepositThreshold uint64
}

type PoolInfo struct {
	// AccPerShare Accumulated EPK per share, times 1e12.
	AccPerShare abi.TokenAmount

	// LastRewardBalance should be updated after any funds withdrawval or burning.
	LastRewardBalance abi.TokenAmount

	PrevEpoch            abi.ChainEpoch
	PrevTotalDataSize    abi.PaddedPieceSize
	CurrentEpoch         abi.ChainEpoch
	CurrentTotalDataSize abi.PaddedPieceSize
}

// ConstructState expert fund construct
func ConstructState(store adt.Store, pool cid.Cid) (*State, error) {
	emptyExpertsMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty map: %w", err)
	}

	emptyPisMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create experts map: %w", err)
	}

	emptyDisqualifiedExpertsMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create tracked experts map: %w", err)
	}

	return &State{
		Experts:             emptyExpertsMapCid,
		PoolInfo:            pool,
		PieceInfos:          emptyPisMapCid,
		DisqualifiedExperts: emptyDisqualifiedExpertsMapCid,

		DataStoreThreshold:       DefaultDataStoreThreshold,
		DailyImportSizeThreshold: DefaultImportThreshold,
	}, nil
}

// Returns err if not found
func (st *State) GetPieceInfos(store adt.Store, pieceCIDs ...cid.Cid) (map[cid.Cid]addr.Address, map[cid.Cid]uint64, error) {
	pieceInfos, err := adt.AsMap(store, st.PieceInfos, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, nil, err
	}

	pieceToExpert := make(map[cid.Cid]addr.Address)
	pieceToThreshold := make(map[cid.Cid]uint64)
	for _, pieceCID := range pieceCIDs {
		var out PieceInfo
		found, err := pieceInfos.Get(abi.CidKey(pieceCID), &out)
		if err != nil {
			return nil, nil, err
		}
		if !found {
			return nil, nil, xerrors.Errorf("piece not found: %s", pieceCID)
		}
		pieceToExpert[pieceCID] = out.Expert
		pieceToThreshold[pieceCID] = out.DepositThreshold
	}
	return pieceToExpert, pieceToThreshold, nil
}

func (st *State) PutPieceInfos(store adt.Store, mustAbsent bool, pieceToInfo map[cid.Cid]*PieceInfo) error {
	if len(pieceToInfo) == 0 {
		return nil
	}

	pieceInfos, err := adt.AsMap(store, st.PieceInfos, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}

	for pieceCid, pieceInfo := range pieceToInfo {
		if mustAbsent {
			absent, err := pieceInfos.PutIfAbsent(abi.CidKey(pieceCid), pieceInfo)
			if err != nil {
				return xerrors.Errorf("failed to put absent: %w", err)
			}
			if !absent {
				return xerrors.Errorf("already exists %s", pieceCid)
			}
		} else {
			err := pieceInfos.Put(abi.CidKey(pieceCid), pieceInfo)
			if err != nil {
				return xerrors.Errorf("failed to put data info: %w", err)
			}
		}
	}

	st.PieceInfos, err = pieceInfos.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush PieceInfos: %w", err)
	}
	return nil
}

// !!!Only called by BatchStoreData.
func (st *State) Deposit(rt Runtime, expertToSize map[addr.Address]abi.PaddedPieceSize) error {

	store := adt.AsStore(rt)

	// update Pool
	pool, err := st.UpdatePool(rt)
	if err != nil {
		return err
	}

	for expertAddr, size := range expertToSize {
		deltaSize := AdjustSize(size)
		// update ExpertInfo
		expertInfo, err := st.GetExpert(store, expertAddr)
		if err != nil {
			return err
		}
		if !expertInfo.Active {
			return xerrors.Errorf("inactive expert cannot deposit: %s", expertAddr)
		}
		if _, err := st.updateVestingFunds(store, rt.CurrEpoch(), pool, expertInfo); err != nil {
			return err
		}

		expertInfo.DataSize += deltaSize
		expertInfo.RewardDebt = big.Div(
			big.Mul(
				big.NewIntUnsigned(uint64(expertInfo.DataSize)),
				pool.AccPerShare),
			AccumulatedMultiplier)
		err = st.SetExpert(store, expertAddr, expertInfo, false)
		if err != nil {
			return err
		}

		pool.CurrentTotalDataSize += deltaSize
	}

	return st.SavePool(store, pool)
}

type ExpertReward struct {
	ExpertInfo
	PendingFunds abi.TokenAmount
	TotalReward  abi.TokenAmount
}

func (st *State) Reward(store adt.Store, currEpoch abi.ChainEpoch, expertAddr address.Address) (*ExpertReward, error) {
	pool, err := st.GetPool(store)
	if err != nil {
		return nil, err
	}
	expert, err := st.GetExpert(store, expertAddr)
	if err != nil {
		return nil, err
	}

	pending, err := st.updateVestingFunds(store, currEpoch, pool, expert)
	if err != nil {
		return nil, err
	}
	total := big.Add(expert.RewardDebt, pending)
	total = big.Add(total, expert.UnlockedFunds)
	total = big.Add(total, expert.LockedFunds)
	return &ExpertReward{
		ExpertInfo:   *expert,
		PendingFunds: pending,
		TotalReward:  total,
	}, nil
}

func (st *State) Claim(rt Runtime, expertAddr address.Address, amount abi.TokenAmount) (abi.TokenAmount, error) {

	pool, err := st.UpdatePool(rt)
	if err != nil {
		return big.Zero(), err
	}

	store := adt.AsStore(rt)

	out, err := st.GetExpert(store, expertAddr)
	if err != nil {
		return big.Zero(), err
	}

	if _, err := st.updateVestingFunds(store, rt.CurrEpoch(), pool, out); err != nil {
		return big.Zero(), err
	}
	if out.Active {
		out.RewardDebt = big.Div(
			big.Mul(
				big.NewIntUnsigned(uint64(out.DataSize)),
				pool.AccPerShare),
			AccumulatedMultiplier)
	}

	actual := big.Min(out.UnlockedFunds, amount)
	out.UnlockedFunds = big.Sub(out.UnlockedFunds, actual)
	if err = st.SetExpert(store, expertAddr, out, false); err != nil {
		return big.Zero(), err
	}

	// save pool
	if pool.LastRewardBalance.LessThan(actual) {
		return big.Zero(), xerrors.Errorf("LastRewardBalance less than expected amount: %s, %s, %s", expertAddr, pool.LastRewardBalance, actual)
	}
	pool.LastRewardBalance = big.Sub(pool.LastRewardBalance, actual)
	if err = st.SavePool(store, pool); err != nil {
		return big.Zero(), err
	}

	return actual, nil
}

func (st *State) ActivateExpert(rt Runtime, expertAddr address.Address) error {
	pool, err := st.UpdatePool(rt)
	if err != nil {
		return err
	}

	store := adt.AsStore(rt)

	expertInfo, err := st.GetExpert(store, expertAddr)
	if err != nil {
		return err
	}
	if !expertInfo.Active {
		expertInfo.Active = true
		// Clear expert's contribution if necessary.
		dInfo, found, err := st.GetDisqualifiedExpertInfo(store, expertAddr)
		if err != nil {
			return xerrors.Errorf("failed to get disqualified for activation: %w", err)
		}
		if found {
			if dInfo.DisqualifiedAt+ClearExpertContributionDelay < rt.CurrEpoch() {
				expertInfo.DataSize = 0
			}
			err = st.DeleteDisqualifiedExpertInfo(store, expertAddr)
			if err != nil {
				return xerrors.Errorf("failed to delete disqualified for activation: %w", err)
			}
		}

		expertInfo.RewardDebt = big.Div(
			big.Mul(
				big.NewIntUnsigned(uint64(expertInfo.DataSize)),
				pool.AccPerShare),
			AccumulatedMultiplier)
		if err := st.SetExpert(store, expertAddr, expertInfo, false); err != nil {
			return err
		}

		pool.CurrentTotalDataSize += expertInfo.DataSize
	}
	return st.SavePool(store, pool)
}

func (st *State) DeactivateExperts(rt Runtime, experts map[addr.Address]bool) (abi.TokenAmount, error) {

	pool, err := st.UpdatePool(rt)
	if err != nil {
		return big.Zero(), err
	}

	totalBurned := abi.NewTokenAmount(0)

	store := adt.AsStore(rt)
	for expertAddr, burnVesting := range experts {
		expertInfo, err := st.GetExpert(store, expertAddr)
		if err != nil {
			return big.Zero(), err
		}
		if !expertInfo.Active {
			continue
		}

		if _, err := st.updateVestingFunds(store, rt.CurrEpoch(), pool, expertInfo); err != nil {
			return big.Zero(), err
		}

		{
			if burnVesting && !expertInfo.LockedFunds.IsZero() {
				if pool.LastRewardBalance.LessThan(expertInfo.LockedFunds) {
					return big.Zero(), xerrors.Errorf("LastRewardBalance %s less than LockedFunds %s", pool.LastRewardBalance, expertInfo.LockedFunds)
				}
				pool.LastRewardBalance = big.Sub(pool.LastRewardBalance, expertInfo.LockedFunds)
				expertInfo.VestingFunds, err = adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
				if err != nil {
					return big.Zero(), xerrors.Errorf("failed to create empty map: %w", err)
				}
				totalBurned = big.Add(totalBurned, expertInfo.LockedFunds)
				expertInfo.LockedFunds = abi.NewTokenAmount(0)
			}
		}

		pool.CurrentTotalDataSize -= expertInfo.DataSize
		// no need to set expertInfo.RewardDebt, as it will be reset when activation
		//
		// Set 'false' after st.updateVestingFunds, not before!
		expertInfo.Active = false
		if err = st.SetExpert(store, expertAddr, expertInfo, false); err != nil {
			return big.Zero(), err
		}

		if err = st.PutDisqualifiedExpertIfAbsent(store, expertAddr, &DisqualifiedExpertInfo{DisqualifiedAt: rt.CurrEpoch()}); err != nil {
			return big.Zero(), err
		}
	}

	return totalBurned, st.SavePool(store, pool)
}

func (st *State) updateVestingFunds(store adt.Store, currEpoch abi.ChainEpoch, pool *PoolInfo, out *ExpertInfo) (abi.TokenAmount, error) {

	pending := abi.NewTokenAmount(0)
	if out.Active {
		pending = big.Mul(big.NewIntUnsigned(uint64(out.DataSize)), pool.AccPerShare)
		pending = big.Div(pending, AccumulatedMultiplier)
		if pending.LessThan(out.RewardDebt) {
			return abi.NewTokenAmount(0), xerrors.Errorf("debt greater than pending: %s, %s", out.RewardDebt, pending)
		}
		pending = big.Sub(pending, out.RewardDebt)
		out.LockedFunds = big.Add(out.LockedFunds, pending)
	}

	vestingFund, err := adt.AsMap(store, out.VestingFunds, builtin.DefaultHamtBitwidth)
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to load VestingFunds: %w", err)
	}

	// add new pending value
	if !pending.IsZero() {
		k := abi.IntKey(int64(currEpoch))
		var old abi.TokenAmount
		found, err := vestingFund.Get(k, &old)
		if err != nil {
			return abi.NewTokenAmount(0), xerrors.Errorf("failed to get old vesting at epoch %d: %w", currEpoch, err)
		}
		if found {
			pending = big.Add(pending, old)
		}
		if err := vestingFund.Put(k, &pending); err != nil {
			return abi.NewTokenAmount(0), xerrors.Errorf("failed to put new vesting at epoch %d: %w", currEpoch, err)
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
		if abi.ChainEpoch(epoch)+RewardVestingDelay < currEpoch {
			unlocked = big.Add(unlocked, amount)
			return vestingFund.Delete(abi.IntKey(epoch))
		}
		return nil
	})
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to iterate vestingFund: %w", err)
	}
	out.VestingFunds, err = vestingFund.Root()
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to flush VestingFunds: %w", err)
	}

	out.LockedFunds = big.Sub(out.LockedFunds, unlocked)
	out.UnlockedFunds = big.Add(out.UnlockedFunds, unlocked)
	return pending, nil
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

// !!!Must save pool if no error occurs during call
// !!!Should only be called once in an actor methods
func (st *State) UpdatePool(rt Runtime) (*PoolInfo, error) {
	pool, err := st.GetPool(adt.AsStore(rt))
	if err != nil {
		return nil, err
	}

	currBalance := rt.CurrentBalance()

	{
		currEpoch := rt.CurrEpoch()
		if currEpoch < pool.CurrentEpoch {
			return nil, xerrors.Errorf("unexpected rt.CurrEpoch %d less than pool.CurrentEpoch", currEpoch, pool.CurrentEpoch)
		}
		// epoch changed
		if currEpoch > pool.CurrentEpoch {
			pool.PrevEpoch = pool.CurrentEpoch
			pool.PrevTotalDataSize = pool.CurrentTotalDataSize
			pool.CurrentEpoch = currEpoch
		}
	}
	if pool.PrevTotalDataSize != 0 {
		reward := big.Sub(currBalance, pool.LastRewardBalance)
		if reward.LessThan(big.Zero()) {
			return nil, xerrors.Errorf("unexpected current balance less than last: %s, %s", currBalance, pool.LastRewardBalance)
		}
		accPerShare := big.Div(big.Mul(reward, AccumulatedMultiplier), big.NewIntUnsigned(uint64(pool.PrevTotalDataSize)))
		pool.AccPerShare = big.Add(pool.AccPerShare, accPerShare)
		pool.LastRewardBalance = currBalance
	}
	return pool, nil
}

func (st *State) GetExpert(store adt.Store, expertAddr addr.Address) (*ExpertInfo, error) {
	experts, err := adt.AsMap(store, st.Experts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to load experts: %w", err)
	}

	var out ExpertInfo
	found, err := experts.Get(abi.AddrKey(expertAddr), &out)
	if err != nil {
		return nil, xerrors.Errorf("failed to get expert for address %v from store %s: %w", expertAddr, st.Experts, err)
	}
	if !found {
		return nil, xerrors.Errorf("expert not found: %s", expertAddr)
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

func (st *State) ListExperts(store adt.Store) (map[addr.Address]ExpertInfo, error) {
	experts, err := adt.AsMap(store, st.Experts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to load experts: %w", err)
	}

	ret := make(map[addr.Address]ExpertInfo)
	var out ExpertInfo
	err = experts.ForEach(&out, func(key string) error {
		expertAddr, err := addr.NewFromBytes([]byte(key))
		if err != nil {
			return err
		}
		ret[expertAddr] = out
		return nil
	})
	if err != nil {
		return nil, xerrors.Errorf("error iterating Experts: %w", err)
	}
	return ret, nil
}

func (st *State) ListDisqualifiedExperts(s adt.Store) (map[addr.Address]abi.ChainEpoch, error) {
	experts, err := adt.AsMap(s, st.DisqualifiedExperts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to load DisqualifiedExperts: %w", err)
	}

	ret := make(map[addr.Address]abi.ChainEpoch)

	var info DisqualifiedExpertInfo
	err = experts.ForEach(&info, func(k string) error {
		expertAddr, err := addr.NewFromBytes([]byte(k))
		if err != nil {
			return err
		}
		ret[expertAddr] = info.DisqualifiedAt
		return nil
	})
	if err != nil {
		return nil, xerrors.Errorf("failed to iterate DisqualifiedExperts: %w", err)
	}
	return ret, nil
}

func (st *State) PutDisqualifiedExpertIfAbsent(s adt.Store, expertAddr addr.Address, info *DisqualifiedExpertInfo) error {
	tracked, err := adt.AsMap(s, st.DisqualifiedExperts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load disqualified experts: %w", err)
	}

	absent, err := tracked.PutIfAbsent(abi.AddrKey(expertAddr), info)
	if err != nil {
		return xerrors.Errorf("failed to put disqualified expert %s", expertAddr)
	}

	if absent {
		st.DisqualifiedExperts, err = tracked.Root()
		return err
	}
	return nil
}

func (st *State) DeleteDisqualifiedExpertInfo(s adt.Store, expertAddr addr.Address) error {
	tracked, err := adt.AsMap(s, st.DisqualifiedExperts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load tracked experts: %w", err)
	}

	present, err := tracked.TryDelete(abi.AddrKey(expertAddr))
	if err != nil {
		return xerrors.Errorf("failed to delete tracked expert %s", expertAddr)
	}
	if present {
		st.DisqualifiedExperts, err = tracked.Root()
		return err
	} else {
		return nil
	}
}

func (st *State) GetDisqualifiedExpertInfo(s adt.Store, expertAddr addr.Address) (*DisqualifiedExpertInfo, bool, error) {
	tracked, err := adt.AsMap(s, st.DisqualifiedExperts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to load tracked experts: %w", err)
	}

	var info DisqualifiedExpertInfo
	found, err := tracked.Get(abi.AddrKey(expertAddr), &info)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to get tracked expert info %s", expertAddr)
	}

	if !found {
		return nil, false, nil
	}
	return &info, true, nil
}

func (st *State) ForEachExpert(store adt.Store, f func(addr.Address, *ExpertInfo)) error {
	experts, err := adt.AsMap(store, st.Experts, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	var info ExpertInfo
	return experts.ForEach(&info, func(key string) error {
		expertAddr, err := addr.NewFromBytes([]byte(key))
		if err != nil {
			return err
		}
		f(expertAddr, &info)
		return nil
	})
}

// Note: Considering that audio files are larger than text files, it is not fair to text files, so take the square root of size
func AdjustSize(originSize abi.PaddedPieceSize) abi.PaddedPieceSize {
	sqrtSize := big.Zero().Sqrt(big.NewIntUnsigned(uint64(originSize)).Int)
	sqrtSize = big.Zero().Sqrt(sqrtSize)
	sqrtSize = big.Zero().Sqrt(sqrtSize)
	return abi.PaddedPieceSize(sqrtSize.Uint64())
}
