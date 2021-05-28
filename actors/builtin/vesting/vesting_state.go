package vesting

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/ipfs/go-cid"
	xerrors "golang.org/x/xerrors"
)

type State struct {
	CoinbaseVestings cid.Cid // Map, HAMT[Coinbase]VestingFund
	MinerCumulations cid.Cid // Map, HAMT[minerID]TokenAmount
	LockedFunds      abi.TokenAmount
}

func ConstructState(store adt.Store) (*State, error) {
	emptyVestingMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty map: %w", err)
	}

	emptyCumMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty map: %w", err)
	}

	return &State{
		LockedFunds:      abi.NewTokenAmount(0),
		CoinbaseVestings: emptyVestingMapCid,
		MinerCumulations: emptyCumMapCid,
	}, nil
}

func (st *State) AddLockedFunds(vestingFunds *VestingFunds, currEpoch abi.ChainEpoch, vestingSum abi.TokenAmount) error {
	if vestingSum.LessThan(big.Zero()) {
		return xerrors.Errorf("negative amount to lock %s", vestingSum)
	}

	// unlock vested funds first
	amountUnlocked := vestingFunds.unlockVestedFunds(currEpoch)
	st.LockedFunds = big.Sub(st.LockedFunds, amountUnlocked)
	if st.LockedFunds.LessThan(big.Zero()) {
		return xerrors.Errorf("negative locked funds %v after unlocking %v", st.LockedFunds, amountUnlocked)
	}
	vestingFunds.UnlockedBalance = big.Add(vestingFunds.UnlockedBalance, amountUnlocked)

	// add locked funds now
	vestingFunds.addLockedFunds(currEpoch, vestingSum, &RewardVestingSpec)

	st.LockedFunds = big.Add(st.LockedFunds, vestingSum)

	return nil
}

func (st *State) WithdrawVestedFunds(vestingFunds *VestingFunds, currEpoch abi.ChainEpoch, requestedAmount abi.TokenAmount) (abi.TokenAmount, error) {
	// Short-circuit to avoid loading vesting funds if we don't have any.
	if st.LockedFunds.IsZero() || len(vestingFunds.Funds) == 0 {
		return big.Zero(), nil
	}

	// newly unlocked
	amountUnlocked := vestingFunds.unlockVestedFunds(currEpoch)
	st.LockedFunds = big.Sub(st.LockedFunds, amountUnlocked)
	if st.LockedFunds.LessThan(big.Zero()) {
		return big.Zero(), xerrors.Errorf("vesting cause locked funds negative %v", st.LockedFunds)
	}
	vestingFunds.UnlockedBalance = big.Add(vestingFunds.UnlockedBalance, amountUnlocked)

	withdrawn := big.Min(vestingFunds.UnlockedBalance, requestedAmount)
	vestingFunds.UnlockedBalance = big.Sub(vestingFunds.UnlockedBalance, withdrawn)

	return withdrawn, nil
}

func (st *State) SaveVestingFunds(store adt.Store, coinbase address.Address, vf *VestingFunds) error {
	vfs, err := adt.AsMap(store, st.CoinbaseVestings, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load coinbase vestings: %w", err)
	}

	err = vfs.Put(abi.AddrKey(coinbase), vf)
	if err != nil {
		return xerrors.Errorf("failed to save vesting funds of %s: %w", coinbase, err)
	}

	st.CoinbaseVestings, err = vfs.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush coinbase vestings: %w", err)
	}
	return nil
}

func (st *State) LoadVestingFunds(store adt.Store, coinbase address.Address) (*VestingFunds, bool, error) {
	vfs, err := adt.AsMap(store, st.CoinbaseVestings, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to load coinbase vestings: %w", err)
	}

	vestingFunds := VestingFunds{
		UnlockedBalance: abi.NewTokenAmount(0),
	}
	found, err := vfs.Get(abi.AddrKey(coinbase), &vestingFunds)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to get vesting funds of %s: %w", coinbase, err)
	}
	return &vestingFunds, found, nil
}

func (st *State) AddMinerCumulation(store adt.Store, miner address.Address, amount abi.TokenAmount) error {
	if amount.LessThan(big.Zero()) {
		return xerrors.Errorf("negative amount to be added: %s", miner)
	}

	mc, err := adt.AsMap(store, st.MinerCumulations, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load cumulations: %w", err)
	}

	var old abi.TokenAmount
	found, err := mc.Get(abi.AddrKey(miner), &old)
	if err != nil {
		return xerrors.Errorf("failed to get cumulation of %s: %w", miner, err)
	}
	if !found {
		old = amount
	} else {
		old = big.Add(old, amount)
	}
	err = mc.Put(abi.AddrKey(miner), &old)
	if err != nil {
		return xerrors.Errorf("failed to put cumulation of %s: %w", miner, err)
	}
	st.MinerCumulations, err = mc.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush cumulation: %w", err)
	}
	return nil
}

func (st *State) GetMinerCumulation(store adt.Store, miner address.Address) (abi.TokenAmount, error) {
	mc, err := adt.AsMap(store, st.MinerCumulations, builtin.DefaultHamtBitwidth)
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to load cumulations: %w", err)
	}

	out := abi.NewTokenAmount(0)
	_, err = mc.Get(abi.AddrKey(miner), &out)
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to get cumulation of %s: %w", miner, err)
	}
	return out, nil
}
