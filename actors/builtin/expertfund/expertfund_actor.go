package expertfund

import (
	"github.com/filecoin-project/go-address"
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
		2:                         a.ApplyRewards,
		3:                         a.Deposit,
		4:                         a.Claim,
		5:                         a.Reset,
		6:                         a.NotifyVote,
		7:                         a.CheckExpert,
	}
}

func (a Actor) Code() cid.Cid {
	return builtin.ExpertFundActorCodeID
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

	emptyArray, err := adt.MakeEmptyArray(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to create state")

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")

	st := ConstructState(emptyArray, emptyMap)
	rt.StateCreate(st)
	return nil
}

// ApplyRewards apply the received value into the fund balance.
func (a Actor) ApplyRewards(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	msgValue := rt.ValueReceived()
	builtin.RequireParam(rt, msgValue.GreaterThan(big.Zero()), "balance to add must be greater than zero")

	var st State
	rt.StateTransaction(&st, func() {
		rt.ValidateImmediateCallerIs(builtin.RewardActorAddr)

		st.TotalExpertReward = big.Add(st.TotalExpertReward, msgValue)
	})

	return nil
}

type ExpertDepositParams struct {
	Expert    address.Address
	PieceID   cid.Cid `checked:"true"`
	PieceSize abi.PaddedPieceSize
}

// Deposit the received value into the fund pool.
func (a Actor) Deposit(rt Runtime, params *ExpertDepositParams) *abi.EmptyValue {

	var st State
	rt.StateTransaction(&st, func() {
		rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

		err := st.Deposit(rt, params.Expert, params.PieceSize)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to deposit expert")
	})
	return nil
}

// ClaimFundParams params
type ClaimFundParams struct {
	Expert address.Address
	Amount abi.TokenAmount
}

// Claim claim the received value into the balance.
func (a Actor) Claim(rt Runtime, params *ClaimFundParams) *abi.EmptyValue {

	var st State
	rt.StateTransaction(&st, func() {
		expertOwner := builtin.RequestExpertControlAddr(rt, params.Expert)

		rt.ValidateImmediateCallerIs(expertOwner)

		err := st.Claim(rt, params.Expert, params.Amount)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to claim expert fund")

		code := rt.Send(expertOwner, builtin.MethodSend, nil, params.Amount, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send claim amount")
	})
	return nil
}

// ExpertParams params
type ExpertParams struct {
	Expert address.Address
}

// Reset reset the expert data.
func (a Actor) Reset(rt Runtime, params *ExpertParams) *abi.EmptyValue {

	var st State
	rt.StateTransaction(&st, func() {
		rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

		err := st.UpdateExpert(rt, params.Expert, 0, UndefTokenAmount)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to reset expert data")
	})
	return nil
}

// NotifyVoteParams vote params
type NotifyVoteParams struct {
	Expert address.Address
	Amount abi.TokenAmount
}

// NotifyVote notify vote
func (a Actor) NotifyVote(rt Runtime, params *NotifyVoteParams) *abi.EmptyValue {

	var st State
	rt.StateTransaction(&st, func() {
		rt.ValidateImmediateCallerType(builtin.VoteActorCodeID)

		err := st.UpdateExpert(rt, params.Expert, -1, params.Amount)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update expert data")
	})
	return nil
}

// CheckExpert check expert
func (a Actor) CheckExpert(rt Runtime, params *ExpertParams) *abi.EmptyValue {

	var st State
	rt.StateTransaction(&st, func() {
		rt.ValidateImmediateCallerType(builtin.VoteActorCodeID)

		builtin.RequestExpertControlAddr(rt, params.Expert)

		err := st.CheckInBlacklist(rt, params.Expert)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to check expert")
	})
	return nil
}
