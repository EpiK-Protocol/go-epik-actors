package retrieval

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

// State of retrieval
type State struct {
	RetrievalStates cid.Cid // Map, HAMT[Address]RetrievalState

	Pledges cid.Cid // Map, HAMT[Address]PledgeState

	// Amount locked, indexed by actor address.
	LockedTable cid.Cid // // BalanceTable

	// Total Client Collateral that is locked -> unlocked when deal is terminated
	TotalLockedCollateral abi.TokenAmount

	// Total escrow table.
	TotalCollateral abi.TokenAmount

	// TotalRetrievalReward retrieval reward
	TotalRetrievalReward abi.TokenAmount

	// PendingReward temp pending reward
	PendingReward abi.TokenAmount
}

// RetrievalState state
type RetrievalState struct {
	Miners        cid.Cid         // Map, HAMT[Address]abi.TokenAmount
	Datas         cid.Cid         // Map, HAMT[PayloadId]RetrievalData
	Amount        abi.TokenAmount // total pledge amount
	EpochDate     uint64
	DailyDataSize abi.PaddedPieceSize // date retrieval size
}

// RetrievalData record retrieval data statistics
type RetrievalData struct {
	PayloadId string
	PieceSize abi.PaddedPieceSize
	Client    addr.Address
	Provider  addr.Address
	Epoch     abi.ChainEpoch
}

// PledgeState record pledge state
type PledgeState struct {
	Targets cid.Cid // Map, HAMT[Address]abi.TokenAmount
	Amount  abi.TokenAmount
}

// LockedState record lock state
type LockedState struct {
	Amount     abi.TokenAmount
	ApplyEpoch abi.ChainEpoch
}

// ConstructState retrieval construct
func ConstructState(store adt.Store) (*State, error) {

	emptyMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty map: %w", err)
	}

	return &State{
		RetrievalStates: emptyMapCid,
		Pledges:         emptyMapCid,
		LockedTable:     emptyMapCid,

		TotalLockedCollateral: abi.NewTokenAmount(0),
		TotalCollateral:       abi.NewTokenAmount(0),
		TotalRetrievalReward:  abi.NewTokenAmount(0),
		PendingReward:         abi.NewTokenAmount(0),
	}, nil
}

// Pledge add balance for
func (st *State) Pledge(store adt.Store, pledger addr.Address, target addr.Address, amount abi.TokenAmount) error {
	if amount.LessThanEqual(big.Zero()) {
		return xerrors.Errorf("invalid amount %v of funds to add", amount)
	}

	// update pledges
	pledgesMap, err := adt.AsMap(store, st.Pledges, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load pledges:%v", err)
	}

	var pledge PledgeState
	found, err := pledgesMap.Get(abi.AddrKey(pledger), &pledge)
	if err != nil {
		return xerrors.Errorf("failed to get pledger info:%v", err)
	}

	tAmount := amount
	var tmap *adt.Map
	if found {
		tmap, err = adt.AsMap(store, pledge.Targets, builtin.DefaultHamtBitwidth)
		if err != nil {
			return xerrors.Errorf("failed to load pledge target:%v", err)
		}
		var outAmount abi.TokenAmount
		tfound, err := tmap.Get(abi.AddrKey(target), &outAmount)
		if err != nil {
			return err
		}
		if tfound {
			tAmount = big.Add(amount, outAmount)
		}
	} else {
		tmap, err = adt.MakeEmptyMap(store, builtin.DefaultHamtBitwidth)
		if err != nil {
			return xerrors.Errorf("failed to create empty map: %w", err)
		}
		pledge = PledgeState{
			Amount: abi.NewTokenAmount(0),
		}
	}
	if err := tmap.Put(abi.AddrKey(target), &tAmount); err != nil {
		return err
	}
	if pledge.Targets, err = tmap.Root(); err != nil {
		return err
	}
	pledge.Amount = big.Add(pledge.Amount, amount)

	if err = pledgesMap.Put(abi.AddrKey(pledger), &pledge); err != nil {
		return err
	}
	if st.Pledges, err = pledgesMap.Root(); err != nil {
		return err
	}

	// update retrieval state
	stateMap, err := adt.AsMap(store, st.RetrievalStates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}

	var state RetrievalState
	found, err = stateMap.Get(abi.AddrKey(target), &state)
	if err != nil {
		return err
	}
	if !found {
		emptyMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
		if err != nil {
			return xerrors.Errorf("failed to create empty map: %w", err)
		}
		state = RetrievalState{
			Miners:        emptyMapCid,
			Datas:         emptyMapCid,
			Amount:        abi.NewTokenAmount(0),
			EpochDate:     0,
			DailyDataSize: abi.PaddedPieceSize(0),
		}
	}
	state.Amount = big.Add(state.Amount, amount)
	if err = stateMap.Put(abi.AddrKey(target), &state); err != nil {
		return err
	}
	if st.RetrievalStates, err = stateMap.Root(); err != nil {
		return err
	}

	// update TotalCollateral
	st.TotalCollateral = big.Add(st.TotalCollateral, amount)
	return nil
}

// ApplyForWithdraw apply for withdraw amount
func (st *State) ApplyForWithdraw(store adt.Store, curEpoch abi.ChainEpoch, pledger addr.Address, target addr.Address, amount abi.TokenAmount) (exitcode.ExitCode, error) {
	if amount.LessThanEqual(big.Zero()) {
		return exitcode.ErrIllegalState, xerrors.Errorf("invalid amount %v of funds to apply", amount)
	}

	// update pledges
	pledgesMap, err := adt.AsMap(store, st.Pledges, builtin.DefaultHamtBitwidth)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}

	var pState PledgeState
	found, err := pledgesMap.Get(abi.AddrKey(pledger), &pState)
	if err != nil {
		return exitcode.ErrIllegalState, xerrors.Errorf("failed to load pledges map: %w", err)
	}
	if !found {
		return exitcode.ErrIllegalState, xerrors.Errorf("failed to find pledge with addr:%s", pledger)
	}
	if pState.Amount.LessThan(amount) {
		return exitcode.ErrIllegalState, xerrors.Errorf("pledge is less than apply amount:%v, pledge:%v", amount, pState.Amount)
	}
	pState.Amount = big.Sub(pState.Amount, amount)

	tmap, err := adt.AsMap(store, pState.Targets, builtin.DefaultHamtBitwidth)
	if err != nil {
		return exitcode.ErrIllegalState, xerrors.Errorf("failed to load pledger target map: %w", err)
	}
	var outAmount abi.TokenAmount
	found, err = tmap.Get(abi.AddrKey(target), &outAmount)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}
	if !found {
		return exitcode.ErrIllegalState, xerrors.Errorf("failed to get pledge target state: %v", target)
	}
	if outAmount.LessThan(amount) {
		return exitcode.ErrIllegalState, xerrors.Errorf("failed to apply withdraw with pledge amount:%v less than amount: %v", outAmount, amount)
	}
	left := big.Sub(outAmount, amount)
	if left.GreaterThan(big.Zero()) {
		if err = tmap.Put(abi.AddrKey(target), &left); err != nil {
			return exitcode.ErrIllegalState, err
		}
	} else {
		if _, err := tmap.TryDelete(abi.AddrKey(target)); err != nil {
			return exitcode.ErrIllegalState, err
		}
	}
	if pState.Targets, err = tmap.Root(); err != nil {
		return exitcode.ErrIllegalState, err
	}
	if err = pledgesMap.Put(abi.AddrKey(pledger), &pState); err != nil {
		return exitcode.ErrIllegalState, err
	}
	if st.Pledges, err = pledgesMap.Root(); err != nil {
		return exitcode.ErrIllegalState, err
	}

	// update retrieve state
	stateMap, err := adt.AsMap(store, st.RetrievalStates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}
	var state RetrievalState
	found, err = stateMap.Get(abi.AddrKey(target), &state)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}
	if !found {
		return exitcode.ErrIllegalState, xerrors.Errorf("failed to get retrieval state: %v", target)
	}
	if state.Amount.LessThan(amount) {
		return exitcode.ErrIllegalState, xerrors.Errorf("apply amount less than state pledge: %v amount:%v", target, state.Amount)
	}
	state.Amount = big.Sub(state.Amount, amount)
	if err = stateMap.Put(abi.AddrKey(target), &state); err != nil {
		return exitcode.ErrIllegalState, err
	}
	if st.RetrievalStates, err = stateMap.Root(); err != nil {
		return exitcode.ErrIllegalState, err
	}

	// update locked state
	lockedMap, err := adt.AsMap(store, st.LockedTable, builtin.DefaultHamtBitwidth)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}
	var outLocked LockedState
	found, err = lockedMap.Get(abi.AddrKey(pledger), &outLocked)

	if err != nil {
		return exitcode.ErrIllegalState, xerrors.Errorf("failed to get locked: %w", err)
	}

	if !found {
		outLocked = LockedState{
			Amount: abi.NewTokenAmount(0),
		}
	}
	outLocked.Amount = big.Add(outLocked.Amount, amount)
	outLocked.ApplyEpoch = curEpoch
	if err = lockedMap.Put(abi.AddrKey(pledger), &outLocked); err != nil {
		return exitcode.ErrForbidden, err
	}
	if st.LockedTable, err = lockedMap.Root(); err != nil {
		return exitcode.ErrIllegalState, err
	}

	st.TotalCollateral = big.Sub(st.TotalCollateral, amount)
	st.TotalLockedCollateral = big.Add(st.TotalLockedCollateral, amount)
	return exitcode.Ok, nil
}

// Withdraw withdraw amount
func (st *State) Withdraw(store adt.Store, curEpoch abi.ChainEpoch, pledger addr.Address, amount abi.TokenAmount) (exitcode.ExitCode, error) {
	if amount.LessThan(big.Zero()) {
		return exitcode.ErrIllegalState, xerrors.Errorf("negative amount %v of funds to withdraw", amount)
	}

	lockedMap, err := adt.AsMap(store, st.LockedTable, builtin.DefaultHamtBitwidth)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}
	var out LockedState
	found, err := lockedMap.Get(abi.AddrKey(pledger), &out)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}
	if !found {
		return exitcode.ErrIllegalState, xerrors.Errorf("withdraw not applied")
	}
	if curEpoch-out.ApplyEpoch < RetrievalLockPeriod || big.Sub(out.Amount, amount).LessThan(big.Zero()) {
		return exitcode.ErrForbidden, xerrors.Errorf("failed to withdraw at %d: %s", out.ApplyEpoch, amount)
	}
	out.Amount = big.Sub(out.Amount, amount)
	if out.Amount.GreaterThan(big.Zero()) {
		if err := lockedMap.Put(abi.AddrKey(pledger), &out); err != nil {
			return exitcode.ErrIllegalState, err
		}
	} else {
		if _, err := lockedMap.TryDelete(abi.AddrKey(pledger)); err != nil {
			return exitcode.ErrIllegalState, err
		}
	}
	if st.LockedTable, err = lockedMap.Root(); err != nil {
		return exitcode.ErrIllegalState, err
	}
	st.TotalLockedCollateral = big.Sub(st.TotalLockedCollateral, out.Amount)
	return exitcode.Ok, nil
}

// RetrievalData record the retrieval data
func (st *State) RetrievalData(store adt.Store, curEpoch abi.ChainEpoch, fromAddr addr.Address, data RetrievalData, minerCheck bool) (exitcode.ExitCode, error) {
	stateMap, err := adt.AsMap(store, st.RetrievalStates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return exitcode.ErrIllegalState, xerrors.Errorf("failed to load retrieval state: %w", err)
	}

	curEpochDay := uint64(curEpoch / RetrievalStateDuration)

	var state RetrievalState
	found, err := stateMap.Get(abi.AddrKey(fromAddr), &state)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}
	if !found {
		return exitcode.ErrIllegalState, xerrors.Errorf("failed to load retrieval state: %v", fromAddr)
	}

	if curEpochDay > state.EpochDate {
		state.DailyDataSize = 0
		state.EpochDate = curEpochDay
	}

	required := big.Mul(big.NewInt(int64(data.PieceSize+state.DailyDataSize)), builtin.TokenPrecision)
	required = big.Div(required, big.NewInt(RetrievalSizePerEPK))
	if big.Sub(state.Amount, required).LessThan(big.Zero()) {
		return exitcode.ErrInsufficientFunds, xerrors.Errorf("not enough balance to statistics for addr %s: escrow balance %s < required %s", fromAddr, state.Amount, required)
	}

	state.DailyDataSize = state.DailyDataSize + data.PieceSize

	// check if miner bind target
	if minerCheck {
		mmap, err := adt.AsMap(store, state.Miners, builtin.DefaultHamtBitwidth)
		if err != nil {
			return exitcode.ErrIllegalState, err
		}
		var mRetrieve abi.TokenAmount
		match, err := mmap.Get(abi.AddrKey(data.Provider), &mRetrieve)
		if err != nil {
			return exitcode.ErrIllegalState, err
		}
		if !match {
			return exitcode.ErrIllegalState, xerrors.Errorf("failed to match miner with pledger: %v, miner:%v", fromAddr, data.Provider)
		}
		required := big.Mul(big.NewInt(int64(data.PieceSize)), builtin.TokenPrecision)
		required = big.Div(required, big.NewInt(RetrievalSizePerEPK))
		mRetrieve = big.Add(mRetrieve, required)
		if err = mmap.Put(abi.AddrKey(data.Provider), &mRetrieve); err != nil {
			return exitcode.ErrIllegalState, err
		}
		if state.Miners, err = mmap.Root(); err != nil {
			return exitcode.ErrIllegalState, err
		}
	}

	// update retrieve data
	dataMap, err := adt.AsMap(store, state.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}

	var out RetrievalData
	found, err = dataMap.Get(adt.StringKey(data.PayloadId), &out)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}
	if found {
		if (out.Epoch / RetrievalStateDuration) >= abi.ChainEpoch(state.EpochDate) {
			data.PieceSize += out.PieceSize
		}
	}

	if err = dataMap.Put(adt.StringKey(data.PayloadId), &data); err != nil {
		return exitcode.ErrIllegalState, err
	}
	if state.Datas, err = dataMap.Root(); err != nil {
		return exitcode.ErrIllegalState, err
	}
	if err = stateMap.Put(abi.AddrKey(fromAddr), &state); err != nil {
		return exitcode.ErrIllegalState, err
	}
	if st.RetrievalStates, err = stateMap.Root(); err != nil {
		return exitcode.ErrIllegalState, err
	}

	return exitcode.Ok, nil
}

// ConfirmData record the retrieval data
func (st *State) ConfirmData(store adt.Store, curEpoch abi.ChainEpoch, fromAddr addr.Address, data RetrievalData) error {
	stateMap, err := adt.AsMap(store, st.RetrievalStates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load retrieval state: %w", err)
	}

	var state RetrievalState
	found, err := stateMap.Get(abi.AddrKey(fromAddr), &state)
	if err != nil {
		return err
	}
	if !found {
		return xerrors.Errorf("failed to load retrieval state: %v", fromAddr)
	}

	dataMap, err := adt.AsMap(store, state.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	var out RetrievalData
	if found, err = dataMap.Get(adt.StringKey(data.PayloadId), &out); !found || err != nil {
		return xerrors.Errorf("failed to load retrieval data: %v", data.PayloadId)
	}

	curEpochDay := curEpoch / RetrievalStateDuration
	if (out.Epoch/RetrievalStateDuration) >= curEpochDay && out.PieceSize > data.PieceSize {
		if state.DailyDataSize+data.PieceSize > out.PieceSize {
			state.DailyDataSize = state.DailyDataSize + data.PieceSize - out.PieceSize
		} else {
			state.DailyDataSize = 0
		}
	}
	if err = dataMap.Put(adt.StringKey(data.PayloadId), &data); err != nil {
		return err
	}
	if state.Datas, err = dataMap.Root(); err != nil {
		return err
	}
	if err = stateMap.Put(abi.AddrKey(fromAddr), &state); err != nil {
		return err
	}
	if st.RetrievalStates, err = stateMap.Root(); err != nil {
		return err
	}

	return nil
}

// StateInfo state info
func (st *State) StateInfo(store adt.Store, fromAddr addr.Address) (*RetrievalState, error) {
	stateMap, err := adt.AsMap(store, st.RetrievalStates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to load retrieval state: %w", err)
	}

	var state RetrievalState
	found, err := stateMap.Get(abi.AddrKey(fromAddr), &state)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, xerrors.Errorf("failed to find retrieval state: %v", fromAddr)
	}
	return &state, nil
}

// DayExpend balance for address
func (st *State) DayExpend(store adt.Store, epoch abi.ChainEpoch, target addr.Address) (abi.TokenAmount, error) {
	stateMap, err := adt.AsMap(store, st.RetrievalStates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to load retrieval state: %w", err)
	}

	var state RetrievalState
	found, err := stateMap.Get(abi.AddrKey(target), &state)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}
	if !found {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to find retrieval state: %v", target)
	}
	expend := big.Mul(big.NewInt(int64(state.DailyDataSize)), builtin.TokenPrecision)
	expend = big.Div(expend, big.NewInt(RetrievalSizePerEPK))
	return expend, nil
}

// BindMiners bind miners
func (st *State) BindMiners(store adt.Store, target addr.Address, miners []addr.Address) error {
	stateMap, err := adt.AsMap(store, st.RetrievalStates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load retrieval state: %w", err)
	}

	var state RetrievalState
	found, err := stateMap.Get(abi.AddrKey(target), &state)
	if err != nil {
		return err
	}
	if !found {
		return xerrors.Errorf("failed to find retrieval state: %v", target)
	}

	mmap, err := adt.AsMap(store, state.Miners, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	for _, miner := range miners {
		mRetrieve := abi.NewTokenAmount(0)
		if _, err := mmap.PutIfAbsent(abi.AddrKey(miner), &mRetrieve); err != nil {
			return err
		}
	}
	if state.Miners, err = mmap.Root(); err != nil {
		return err
	}
	if err = stateMap.Put(abi.AddrKey(target), &state); err != nil {
		return err
	}
	if st.RetrievalStates, err = stateMap.Root(); err != nil {
		return err
	}

	return nil
}

// UnbindMiners unbind miners
func (st *State) UnbindMiners(store adt.Store, target addr.Address, miners []addr.Address) error {
	stateMap, err := adt.AsMap(store, st.RetrievalStates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load retrieval state: %w", err)
	}

	var state RetrievalState
	found, err := stateMap.Get(abi.AddrKey(target), &state)
	if err != nil {
		return err
	}
	if !found {
		return xerrors.Errorf("failed to find retrieval state: %v", target)
	}
	mmap, err := adt.AsMap(store, state.Miners, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	for _, miner := range miners {
		if _, err := mmap.TryDelete(abi.AddrKey(miner)); err != nil {
			return err
		}
	}
	if state.Miners, err = mmap.Root(); err != nil {
		return err
	}
	if err = stateMap.Put(abi.AddrKey(target), &state); err != nil {
		return err
	}
	if st.RetrievalStates, err = stateMap.Root(); err != nil {
		return err
	}

	return nil
}
