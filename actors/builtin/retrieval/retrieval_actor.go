package retrieval

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type Actor struct{}

type Runtime = runtime.Runtime

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.Pledge,
		3:                         a.ApplyForWithdraw,
		4:                         a.WithdrawBalance,
		5:                         a.RetrievalData,
		6:                         a.ConfirmData,
		7:                         a.ApplyRewards,
		8:                         a.TotalCollateral,
		9:                         a.MinerRetrieval,
		10:                        a.BindMiners,
		11:                        a.UnbindMiners,
	}
}

func (a Actor) Code() cid.Cid {
	return builtin.RetrievalFundActorCodeID
}

func (a Actor) IsSingleton() bool {
	return true
}

func (a Actor) State() cbor.Er {
	return new(State)
}

var _ runtime.VMActor = Actor{}

////////////////////////////////////////////////////////////////////////////////
// Actor methods
////////////////////////////////////////////////////////////////////////////////

func (a Actor) Constructor(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	st, err := ConstructState(adt.AsStore(rt))
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")
	rt.StateCreate(st)
	return nil
}

type PledgeParams struct {
	Address addr.Address
	Miners  []addr.Address
}

// Pledge the received value into the balance held in escrow.
func (a Actor) Pledge(rt Runtime, params *PledgeParams) *abi.EmptyValue {
	msgValue := rt.ValueReceived()
	builtin.RequireParam(rt, msgValue.GreaterThan(big.Zero()), "balance to pledge must be greater than zero")

	// only signing parties can add balance for client AND provider.
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	pledger, _, _, _ := escrowAddress(rt, rt.Caller())
	nominal, _, _, _ := escrowAddress(rt, params.Address)

	var st State
	rt.StateTransaction(&st, func() {
		err := st.Pledge(adt.AsStore(rt), pledger, nominal, rt.ValueReceived())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to pledge")

		if len(params.Miners) > 0 {
			err = st.BindMiners(adt.AsStore(rt), nominal, params.Miners)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to bind miners")
		}
	})

	for _, miner := range params.Miners {
		code := rt.Send(miner, builtin.MethodsMiner.BindRetrievalPledger, &builtin.RetrievalPledgeParams{Pledger: nominal}, abi.NewTokenAmount(0), &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send bind retrieval pledger")
	}
	return nil
}

type WithdrawBalanceParams struct {
	ProviderOrClientAddress addr.Address
	Amount                  abi.TokenAmount
}

// Attempt to withdraw the specified amount from the balance held in escrow.
// If less than the specified amount is available, yields the entire available balance.
func (a Actor) ApplyForWithdraw(rt Runtime, params *WithdrawBalanceParams) *abi.EmptyValue {
	if params.Amount.LessThanEqual(big.Zero()) {
		rt.Abortf(exitcode.ErrIllegalArgument, "invalid amount %v", params.Amount)
	}

	nominal, _, approvedCallers, _ := escrowAddress(rt, params.ProviderOrClientAddress)
	// for providers -> only corresponding owner or worker can withdraw
	// for clients -> only the client i.e the recipient can withdraw
	rt.ValidateImmediateCallerIs(approvedCallers...)

	var st State
	rt.StateTransaction(&st, func() {
		code, err := st.ApplyForWithdraw(adt.AsStore(rt), rt.CurrEpoch(), nominal, params.Amount)
		builtin.RequireNoErr(rt, err, code, "failed to apply withdraw")
	})
	return nil
}

// Attempt to withdraw the specified amount from the balance held in escrow.
// If less than the specified amount is available, yields the entire available balance.
func (a Actor) WithdrawBalance(rt Runtime, params *WithdrawBalanceParams) *abi.EmptyValue {
	if params.Amount.LessThanEqual(big.Zero()) {
		rt.Abortf(exitcode.ErrIllegalArgument, "invalid amount %v", params.Amount)
	}

	nominal, _, approvedCallers, _ := escrowAddress(rt, params.ProviderOrClientAddress)
	// for providers -> only corresponding owner or worker can withdraw
	// for clients -> only the client i.e the recipient can withdraw
	rt.ValidateImmediateCallerIs(approvedCallers...)

	var st State
	rt.StateTransaction(&st, func() {
		code, err := st.Withdraw(adt.AsStore(rt), rt.CurrEpoch(), nominal, params.Amount)
		builtin.RequireNoErr(rt, err, code, "failed to withdraw")
	})
	code := rt.Send(params.ProviderOrClientAddress, builtin.MethodSend, nil, params.Amount, &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to send withdraw amount")
	return nil
}

// Resolves a provider or client address to the canonical form against which a balance should be held, and
// the designated recipient address of withdrawals (which is the same, for simple account parties).
func escrowAddress(rt Runtime, address addr.Address) (nominal addr.Address, recipient addr.Address, approved []addr.Address, coinbase addr.Address) {
	// Resolve the provided address to the canonical form against which the balance is held.
	nominal, ok := rt.ResolveAddress(address)
	if !ok {
		rt.Abortf(exitcode.ErrIllegalArgument, "failed to resolve address %v", address)
	}

	codeID, ok := rt.GetActorCodeCID(nominal)
	if !ok {
		rt.Abortf(exitcode.ErrIllegalArgument, "no code for address %v", nominal)
	}

	if codeID.Equals(builtin.StorageMinerActorCodeID) {
		// Storage miner actor entry; implied funds recipient is the associated owner address.
		ownerAddr, workerAddr, _, coinbase := builtin.RequestMinerControlAddrs(rt, nominal)
		return nominal, ownerAddr, []addr.Address{ownerAddr, workerAddr}, coinbase
	}

	return nominal, nominal, []addr.Address{nominal}, nominal
}

// RetrievalDataParams retrieval data params
type RetrievalDataParams struct {
	PayloadId string
	Size      uint64
	Client    addr.Address
	Provider  addr.Address
}

// RetrievalData retrieval data statistics
func (a Actor) RetrievalData(rt Runtime, params *RetrievalDataParams) *abi.EmptyValue {
	nominal, _, _, _ := escrowAddress(rt, params.Client)
	_, _, _, coinbase := escrowAddress(rt, params.Provider)
	// for providers -> only corresponding owner or worker can withdraw
	// for clients -> only the client i.e the recipient can withdraw
	rt.ValidateImmediateCallerType(builtin.FlowChannelActorCodeID)

	reward := abi.NewTokenAmount(0)
	var st State
	rt.StateTransaction(&st, func() {
		data := RetrievalData{
			PayloadId: params.PayloadId,
			PieceSize: abi.PaddedPieceSize(params.Size),
			Client:    params.Client,
			Provider:  params.Provider,
			Epoch:     rt.CurrEpoch(),
		}
		code, err := st.RetrievalData(adt.AsStore(rt), rt.CurrEpoch(), nominal, data, false)
		builtin.RequireNoErr(rt, err, code, "failed to Statistics")

		reward = big.Mul(big.NewInt(int64(data.PieceSize)), RetrievalRewardPerByte)
		if st.PendingReward.GreaterThanEqual(reward) {
			st.PendingReward = big.Sub(st.PendingReward, reward)
		} else {
			reward = st.PendingReward
			st.PendingReward = abi.NewTokenAmount(0)
		}
	})
	if reward.GreaterThan(big.Zero()) {
		code := rt.Send(coinbase, builtin.MethodSend, nil, reward, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send retrieval reward")
	}
	return nil
}

// ConfirmData retrieval data statistics
func (a Actor) ConfirmData(rt Runtime, params *RetrievalDataParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.FlowChannelActorCodeID)

	nominal, _, _, _ := escrowAddress(rt, params.Client)
	// approvedCallers = append(approvedCallers, providerCallers...)
	// for providers -> only corresponding owner or worker can withdraw
	// for clients -> only the client i.e the recipient can withdraw

	var st State
	rt.StateTransaction(&st, func() {
		data := RetrievalData{
			PayloadId: params.PayloadId,
			PieceSize: abi.PaddedPieceSize(params.Size),
			Client:    params.Client,
			Provider:  params.Provider,
			Epoch:     rt.CurrEpoch(),
		}
		err := st.ConfirmData(adt.AsStore(rt), rt.CurrEpoch(), nominal, data)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to confirm data")
	})
	return nil
}

// MinerRetrieval miner retrieval
func (a Actor) MinerRetrieval(rt Runtime, params *RetrievalDataParams) *abi.EmptyValue {
	nominal, _, _, _ := escrowAddress(rt, params.Client)
	// _, _, providerCallers := escrowAddress(rt, params.Provider)
	// approvedCallers = append(approvedCallers, providerCallers...)
	// for providers -> only corresponding owner or worker can withdraw
	// for clients -> only the client i.e the recipient can withdraw
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		data := RetrievalData{
			PayloadId: params.PayloadId,
			PieceSize: abi.PaddedPieceSize(params.Size),
			Client:    params.Client,
			Provider:  params.Provider,
			Epoch:     rt.CurrEpoch(),
		}
		code, err := st.RetrievalData(adt.AsStore(rt), rt.CurrEpoch(), nominal, data, true)
		builtin.RequireNoErr(rt, err, code, "failed to handle miner retrieval")
	})
	return nil
}

// ApplyRewards receive data retrievel
func (a Actor) ApplyRewards(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	msgValue := rt.ValueReceived()
	builtin.RequireParam(rt, msgValue.GreaterThan(big.Zero()), "reward to add must be greater than zero")

	rt.ValidateImmediateCallerIs(builtin.RewardActorAddr)

	var st State
	rt.StateTransaction(&st, func() {
		st.PendingReward = big.Add(st.PendingReward, msgValue)
		st.TotalRetrievalReward = big.Add(st.TotalRetrievalReward, msgValue)
	})

	return nil
}

// TotalCollateralReturn return total collateral
type TotalCollateralReturn struct {
	TotalCollateral abi.TokenAmount
}

// TotalCollateral return total collateral
func (a Actor) TotalCollateral(rt Runtime, _ *abi.EmptyValue) *TotalCollateralReturn {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)

	return &TotalCollateralReturn{
		TotalCollateral: st.TotalCollateral,
	}
}

// BindMinersParams params
type BindMinersParams struct {
	Pledger addr.Address
	Miners  []addr.Address
}

// BindMiners bind miners
func (a Actor) BindMiners(rt Runtime, params *BindMinersParams) *abi.EmptyValue {
	pledger, _, _, _ := escrowAddress(rt, params.Pledger)
	rt.ValidateImmediateCallerIs(pledger)

	var st State
	rt.StateTransaction(&st, func() {
		err := st.BindMiners(adt.AsStore(rt), pledger, params.Miners)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to bind miners")
	})

	for _, miner := range params.Miners {
		code := rt.Send(miner, builtin.MethodsMiner.BindRetrievalPledger, &builtin.RetrievalPledgeParams{Pledger: pledger}, abi.NewTokenAmount(0), &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send bind retrieval depositor")
	}

	return nil
}

// UnbindMiners unbind miners
func (a Actor) UnbindMiners(rt Runtime, params *BindMinersParams) *abi.EmptyValue {
	pledger, _, _, _ := escrowAddress(rt, params.Pledger)
	rt.ValidateImmediateCallerIs(pledger)

	var st State
	rt.StateTransaction(&st, func() {
		err := st.UnbindMiners(adt.AsStore(rt), pledger, params.Miners)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to unbind miners")
	})

	return nil
}
