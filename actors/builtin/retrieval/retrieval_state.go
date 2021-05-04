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

	Deposits cid.Cid // Map, HAMT[Address]DepositState

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
	Deposits  cid.Cid // Map, HAMT[Address]abi.TokenAmount
	Miners    []addr.Address
	Datas     cid.Cid         // Map, HAMT[PayloadId]RetrievalData
	Amount    abi.TokenAmount // total deposit amount
	EpochDate uint64
	DateSize  abi.PaddedPieceSize // date retrieval size
}

// RetrievalData record retrieval data statistics
type RetrievalData struct {
	PayloadId string
	PieceSize abi.PaddedPieceSize
	Client    addr.Address
	Provider  addr.Address
	Epoch     abi.ChainEpoch
}

// DepositState record deposit state
type DepositState struct {
	Targets []addr.Address
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
		Deposits:        emptyMapCid,
		LockedTable:     emptyMapCid,

		TotalLockedCollateral: abi.NewTokenAmount(0),
		TotalCollateral:       abi.NewTokenAmount(0),
		TotalRetrievalReward:  abi.NewTokenAmount(0),
		PendingReward:         abi.NewTokenAmount(0),
	}, nil
}

// Deposit add balance for
func (st *State) Deposit(store adt.Store, depositor addr.Address, target addr.Address, amount abi.TokenAmount) error {
	if amount.LessThanEqual(big.Zero()) {
		return xerrors.Errorf("invalid amount %v of funds to add", amount)
	}

	// update deposits
	depositsMap, err := adt.AsMap(store, st.Deposits, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load dsposits:%v", err)
	}

	var deposit DepositState
	found, err := depositsMap.Get(abi.AddrKey(depositor), &deposit)
	if err != nil {
		return xerrors.Errorf("failed to get dspositor info:%v", err)
	}

	if found {
		tfound := false
		for _, t := range deposit.Targets {
			if t == target {
				tfound = true
				break
			}
		}
		if !tfound {
			deposit.Targets = append(deposit.Targets, target)
		}
	} else {
		deposit = DepositState{
			Targets: []addr.Address{target},
		}
	}

	if err = depositsMap.Put(abi.AddrKey(depositor), &deposit); err != nil {
		return err
	}
	if st.Deposits, err = depositsMap.Root(); err != nil {
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
			Deposits:  emptyMapCid,
			Miners:    []addr.Address{},
			Datas:     emptyMapCid,
			Amount:    abi.NewTokenAmount(0),
			EpochDate: 0,
			DateSize:  abi.PaddedPieceSize(0),
		}
	}
	state.Amount = big.Add(state.Amount, amount)
	sdmap, err := adt.AsMap(store, state.Deposits, builtin.DefaultHamtBitwidth)
	if err != nil {
		return err
	}
	var dAmount abi.TokenAmount
	_, err = sdmap.Get(abi.AddrKey(depositor), &dAmount)
	if err != nil {
		return err
	}
	dAmount = big.Add(dAmount, amount)
	if err = sdmap.Put(abi.AddrKey(depositor), &dAmount); err != nil {
		return err
	}
	if state.Deposits, err = sdmap.Root(); err != nil {
		return err
	}
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
func (st *State) ApplyForWithdraw(store adt.Store, curEpoch abi.ChainEpoch, depositor addr.Address, amount abi.TokenAmount) (exitcode.ExitCode, error) {
	if amount.LessThanEqual(big.Zero()) {
		return exitcode.ErrIllegalState, xerrors.Errorf("invalid amount %v of funds to apply", amount)
	}

	depositsMap, err := adt.AsMap(store, st.Deposits, builtin.DefaultHamtBitwidth)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}

	var deposits DepositState
	found, err := depositsMap.Get(abi.AddrKey(depositor), &deposits)
	if err != nil {
		return exitcode.ErrIllegalState, xerrors.Errorf("failed to load deposits map: %w", err)
	}
	if !found {
		return exitcode.ErrIllegalState, xerrors.Errorf("failed to find deposit with addr:%s", depositor)
	}

	stateMap, err := adt.AsMap(store, st.RetrievalStates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}

	total := abi.NewTokenAmount(0)
	i := 0
	for ; i < len(deposits.Targets); i++ {
		target := deposits.Targets[i]
		var state RetrievalState
		found, err := stateMap.Get(abi.AddrKey(target), &state)
		if err != nil {
			return exitcode.ErrIllegalState, err
		}
		if !found {
			return exitcode.ErrIllegalState, xerrors.Errorf("failed to get retrieval state: %v", target)
		}
		dmap, err := adt.AsMap(store, state.Deposits, builtin.DefaultHamtBitwidth)
		if err != nil {
			return exitcode.ErrIllegalState, err
		}
		var outAmount abi.TokenAmount
		found, err = dmap.Get(abi.AddrKey(depositor), &outAmount)
		if err != nil {
			return exitcode.ErrIllegalState, err
		}
		if !found {
			return exitcode.ErrIllegalState, xerrors.Errorf("failed to get retrieval state deposit: %v", target)
		}

		left := abi.NewTokenAmount(0)
		if big.Add(total, outAmount).LessThan(amount) {
			total = big.Add(total, outAmount)
		} else {
			left = big.Sub(big.Add(total, outAmount), amount)
			total = amount
		}
		if err = dmap.Put(abi.AddrKey(depositor), &left); err != nil {
			return exitcode.ErrIllegalState, err
		}
		if state.Deposits, err = dmap.Root(); err != nil {
			return exitcode.ErrIllegalState, err
		}
		state.Amount = big.Sub(state.Amount, big.Sub(outAmount, left))

		if err = stateMap.Put(abi.AddrKey(target), &state); err != nil {
			return exitcode.ErrIllegalState, err
		}
		if left.GreaterThan(big.Zero()) {
			break
		}
	}

	deposits.Targets = deposits.Targets[i:]
	if err = depositsMap.Put(abi.AddrKey(depositor), &deposits); err != nil {
		return exitcode.ErrIllegalState, err
	}
	if st.Deposits, err = depositsMap.Root(); err != nil {
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
	found, err = lockedMap.Get(abi.AddrKey(depositor), &outLocked)

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
	if err = lockedMap.Put(abi.AddrKey(depositor), &outLocked); err != nil {
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
func (st *State) Withdraw(store adt.Store, curEpoch abi.ChainEpoch, depositor addr.Address, amount abi.TokenAmount) (exitcode.ExitCode, error) {
	if amount.LessThan(big.Zero()) {
		return exitcode.ErrIllegalState, xerrors.Errorf("negative amount %v of funds to withdraw", amount)
	}

	lockedMap, err := adt.AsMap(store, st.LockedTable, builtin.DefaultHamtBitwidth)
	if err != nil {
		return exitcode.ErrIllegalState, err
	}
	var out LockedState
	found, err := lockedMap.Get(abi.AddrKey(depositor), &out)
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
	lockedMap.Put(abi.AddrKey(depositor), &out)
	if st.LockedTable, err = lockedMap.Root(); err != nil {
		return exitcode.ErrIllegalState, err
	}
	st.TotalLockedCollateral = big.Sub(st.TotalLockedCollateral, out.Amount)
	return exitcode.Ok, nil
}

// RetrievalData record the retrieval data
func (st *State) RetrievalData(store adt.Store, curEpoch abi.ChainEpoch, fromAddr addr.Address, data RetrievalData) (exitcode.ExitCode, error) {
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
		state.DateSize = 0
		state.EpochDate = curEpochDay
	}

	required := big.Mul(big.NewInt(int64(data.PieceSize+state.DateSize)), builtin.TokenPrecision)
	required = big.Div(required, big.NewInt(RetrievalSizePerEPK))
	if big.Sub(state.Amount, required).LessThan(big.Zero()) {
		return exitcode.ErrInsufficientFunds, xerrors.Errorf("not enough balance to statistics for addr %s: escrow balance %s < required %s", fromAddr, state.Amount, required)
	}

	state.DateSize = data.PieceSize + state.DateSize

	dataMap, err := adt.AsMap(store, state.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return exitcode.ErrIllegalState, err
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
func (st *State) ConfirmData(store adt.Store, curEpoch abi.ChainEpoch, fromAddr addr.Address, data RetrievalData) (abi.TokenAmount, error) {
	stateMap, err := adt.AsMap(store, st.RetrievalStates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to load retrieval state: %w", err)
	}

	var state RetrievalState
	found, err := stateMap.Get(abi.AddrKey(fromAddr), &state)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}
	if !found {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to load retrieval state: %v", fromAddr)
	}

	dataMap, err := adt.AsMap(store, state.Datas, builtin.DefaultHamtBitwidth)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}
	var out RetrievalData
	if found, err = dataMap.Get(adt.StringKey(data.PayloadId), &out); !found || err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to load retrieval data: %v", data.PayloadId)
	}

	curEpochDay := curEpoch / RetrievalStateDuration
	if (out.Epoch / RetrievalStateDuration) == curEpochDay {
		state.DateSize = state.DateSize - out.PieceSize + data.PieceSize
	}
	out.PieceSize = data.PieceSize
	if err = dataMap.Put(adt.StringKey(data.PayloadId), &data); err != nil {
		return abi.NewTokenAmount(0), err
	}
	if state.Datas, err = dataMap.Root(); err != nil {
		return abi.NewTokenAmount(0), err
	}
	if err = stateMap.Put(abi.AddrKey(fromAddr), &state); err != nil {
		return abi.NewTokenAmount(0), err
	}
	if st.RetrievalStates, err = stateMap.Root(); err != nil {
		return abi.NewTokenAmount(0), err
	}

	amount := big.Mul(big.NewInt(int64(data.PieceSize)), RetrievalRewardPerByte)
	if st.PendingReward.GreaterThanEqual(amount) {
		st.PendingReward = big.Sub(st.PendingReward, amount)
	} else {
		amount = st.PendingReward
		st.PendingReward = abi.NewTokenAmount(0)
	}
	return amount, nil
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
func (st *State) DayExpend(store adt.Store, epoch abi.ChainEpoch, fromAddr addr.Address) (abi.TokenAmount, error) {
	stateMap, err := adt.AsMap(store, st.RetrievalStates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to load retrieval state: %w", err)
	}

	var state RetrievalState
	found, err := stateMap.Get(abi.AddrKey(fromAddr), &state)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}
	if !found {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to find retrieval state: %v", fromAddr)
	}
	expend := big.Mul(big.NewInt(int64(state.DateSize)), builtin.TokenPrecision)
	expend = big.Div(expend, big.NewInt(RetrievalSizePerEPK))
	return expend, nil
}

// LockedState locked state for address
func (st *State) LockedState(store adt.Store, fromAddr addr.Address) (*LockedState, error) {
	lockedMap, err := adt.AsMap(store, st.LockedTable, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, err
	}
	var out LockedState
	_, err = lockedMap.Get(abi.AddrKey(fromAddr), &out)
	return &out, nil
}

// BindMiners bind miners
func (st *State) BindMiners(store adt.Store, fromAddr addr.Address, miners []addr.Address) error {
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
		return xerrors.Errorf("failed to find retrieval state: %v", fromAddr)
	}
	for _, bind := range miners {
		found := false
		for _, miner := range state.Miners {
			if bind == miner {
				found = true
				break
			}
		}
		if !found {
			state.Miners = append(state.Miners, bind)
		}
	}
	if err = stateMap.Put(abi.AddrKey(fromAddr), &state); err != nil {
		return err
	}
	if st.RetrievalStates, err = stateMap.Root(); err != nil {
		return err
	}

	return nil
}

// UnbindMiners unbind miners
func (st *State) UnbindMiners(store adt.Store, fromAddr addr.Address, miners []addr.Address) error {
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
		return xerrors.Errorf("failed to find retrieval state: %v", fromAddr)
	}
	for _, bind := range miners {
		found := false
		for index, miner := range state.Miners {
			if bind == miner {
				found = true
				state.Miners = append(state.Miners[:index], state.Miners[index+1:]...)
				break
			}
		}
		if !found {
			return xerrors.Errorf("failed to find retrieval miner: %v", bind)
		}
	}
	if err = stateMap.Put(abi.AddrKey(fromAddr), &state); err != nil {
		return err
	}
	if st.RetrievalStates, err = stateMap.Root(); err != nil {
		return err
	}

	return nil
}
