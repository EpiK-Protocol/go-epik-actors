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
		2:                         a.AddBalance,
		3:                         a.ApplyForWithdraw,
		4:                         a.WithdrawBalance,
		5:                         a.RetrievalData,
		6:                         a.ConfirmData,
		7:                         a.ApplyRewards,
		8:                         a.TotalCollateral,
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

// Deposits the received value into the balance held in escrow.
func (a Actor) AddBalance(rt Runtime, providerOrClientAddress *addr.Address) *abi.EmptyValue {
	msgValue := rt.ValueReceived()
	builtin.RequireParam(rt, msgValue.GreaterThan(big.Zero()), "balance to add must be greater than zero")

	// only signing parties can add balance for client AND provider.
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	nominal, _, _ := escrowAddress(rt, *providerOrClientAddress)

	var st State
	rt.StateTransaction(&st, func() {
		err := st.AddBalance(rt, nominal, rt.ValueReceived())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add balance")
	})
	return nil
}

type WithdrawBalanceParams struct {
	ProviderOrClientAddress addr.Address
	Amount                  abi.TokenAmount
}

// Attempt to withdraw the specified amount from the balance held in escrow.
// If less than the specified amount is available, yields the entire available balance.
func (a Actor) ApplyForWithdraw(rt Runtime, params *WithdrawBalanceParams) *abi.EmptyValue {
	if params.Amount.LessThan(big.Zero()) {
		rt.Abortf(exitcode.ErrIllegalArgument, "negative amount %v", params.Amount)
	}

	nominal, _, approvedCallers := escrowAddress(rt, params.ProviderOrClientAddress)
	// for providers -> only corresponding owner or worker can withdraw
	// for clients -> only the client i.e the recipient can withdraw
	rt.ValidateImmediateCallerIs(approvedCallers...)

	var st State
	rt.StateTransaction(&st, func() {
		code, err := st.ApplyForWithdraw(rt, nominal, params.Amount)
		builtin.RequireNoErr(rt, err, code, "failed to withdraw")
	})
	return nil
}

// Attempt to withdraw the specified amount from the balance held in escrow.
// If less than the specified amount is available, yields the entire available balance.
func (a Actor) WithdrawBalance(rt Runtime, params *WithdrawBalanceParams) *abi.EmptyValue {
	if params.Amount.LessThan(big.Zero()) {
		rt.Abortf(exitcode.ErrIllegalArgument, "negative amount %v", params.Amount)
	}

	nominal, _, approvedCallers := escrowAddress(rt, params.ProviderOrClientAddress)
	// for providers -> only corresponding owner or worker can withdraw
	// for clients -> only the client i.e the recipient can withdraw
	rt.ValidateImmediateCallerIs(approvedCallers...)

	var st State
	rt.StateTransaction(&st, func() {
		code, err := st.Withdraw(rt, nominal, params.Amount)
		builtin.RequireNoErr(rt, err, code, "failed to withdraw")
	})
	code := rt.Send(params.ProviderOrClientAddress, builtin.MethodSend, nil, params.Amount, &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to send withdraw amount")
	return nil
}

// Resolves a provider or client address to the canonical form against which a balance should be held, and
// the designated recipient address of withdrawals (which is the same, for simple account parties).
func escrowAddress(rt Runtime, address addr.Address) (nominal addr.Address, recipient addr.Address, approved []addr.Address) {
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
		ownerAddr, workerAddr, _ := builtin.RequestMinerControlAddrs(rt, nominal)
		return nominal, ownerAddr, []addr.Address{ownerAddr, workerAddr}
	}

	return nominal, nominal, []addr.Address{nominal}
}

// RetrievalDataParams retrieval data params
type RetrievalDataParams struct {
	Flowch   addr.Address
	Size     uint64
	Client   addr.Address
	Provider addr.Address
}

// RetrievalData retrieval data statistics
func (a Actor) RetrievalData(rt Runtime, params *RetrievalDataParams) *abi.EmptyValue {
	nominal, _, _ := escrowAddress(rt, params.Client)
	// for providers -> only corresponding owner or worker can withdraw
	// for clients -> only the client i.e the recipient can withdraw
	rt.ValidateImmediateCallerType(builtin.FlowChannelActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		statistics := &RetrievalState{
			Flowch:    params.Flowch,
			PieceSize: abi.PaddedPieceSize(params.Size),
			Client:    params.Client,
			Provider:  params.Provider,
			Epoch:     rt.CurrEpoch(),
		}
		code, err := st.RetrievalData(rt, nominal, statistics)
		builtin.RequireNoErr(rt, err, code, "failed to Statistics")
	})
	return nil
}

// ConfirmData retrieval data statistics
func (a Actor) ConfirmData(rt Runtime, params *RetrievalDataParams) *abi.EmptyValue {
	nominal, _, _ := escrowAddress(rt, params.Client)
	// _, _, providerCallers := escrowAddress(rt, params.Provider)
	// approvedCallers = append(approvedCallers, providerCallers...)
	// for providers -> only corresponding owner or worker can withdraw
	// for clients -> only the client i.e the recipient can withdraw
	rt.ValidateImmediateCallerType(builtin.FlowChannelActorCodeID)

	var reward abi.TokenAmount
	var st State
	rt.StateTransaction(&st, func() {
		statistics := &RetrievalState{
			Flowch:    params.Flowch,
			PieceSize: abi.PaddedPieceSize(params.Size),
			Client:    params.Client,
			Provider:  params.Provider,
			Epoch:     rt.CurrEpoch(),
		}
		amount, err := st.ConfirmData(adt.AsStore(rt), nominal, statistics)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to confirm data")
		reward = amount
	})
	code := rt.Send(params.Provider, builtin.MethodSend, nil, reward, &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to send retrieval reward")
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
