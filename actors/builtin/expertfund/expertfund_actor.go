package expertfund

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type Actor struct{}

type Runtime = runtime.Runtime

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.ApplyRewards,
		3:                         a.Claim,
		4:                         a.NotifyUpdate,
		5:                         a.FoundationChange,
		6:                         a.BatchCheckData,
		7:                         a.BatchStoreData,
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

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")

	pool := rt.StorePut(&PoolInfo{
		LastRewardBlock: abi.ChainEpoch(0),
		AccPerShare:     abi.NewTokenAmount(0)})
	st := ConstructState(emptyMap, pool)
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

// ClaimFundParams params
type ClaimFundParams struct {
	Expert address.Address
	Amount abi.TokenAmount
}

// Claim claim the received value into the balance.
func (a Actor) Claim(rt Runtime, params *ClaimFundParams) *abi.EmptyValue {

	var st State
	var expertOwner address.Address
	rt.StateTransaction(&st, func() {
		expertOwner = builtin.RequestExpertControlAddr(rt, params.Expert)

		rt.ValidateImmediateCallerIs(expertOwner)

		err := st.Claim(rt, params.Expert, params.Amount)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to claim expert fund")
	})
	code := rt.Send(expertOwner, builtin.MethodSend, nil, params.Amount, &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to send claim amount")
	return nil
}

// NotifyUpdateParams expert params
type NotifyUpdateParams struct {
	Expert   address.Address
	PieceID  cid.Cid
	IsImport bool
}

// NotifyUpdate notify vote
func (a Actor) NotifyUpdate(rt Runtime, params *NotifyUpdateParams) *abi.EmptyValue {

	validateCode := exitcode.Ok
	var out expert.DataOnChainInfo
	if params.IsImport == false {
		validateCode = rt.Send(params.Expert, builtin.MethodsExpert.Validate, nil, abi.NewTokenAmount(0), &builtin.Discard{})
		dataParams := &expert.ExpertDataParams{
			PieceID: params.PieceID,
		}
		code := rt.Send(params.Expert, builtin.MethodsExpert.GetData, dataParams, abi.NewTokenAmount(0), &out)
		builtin.RequireSuccess(rt, code, "failed to get data")
	}

	var st State
	rt.StateTransaction(&st, func() {
		rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

		if params.IsImport {
			found, err := st.HasDataID(adt.AsStore(rt), params.PieceID.String())
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to check data in fund record")
			if found {
				builtin.RequireNoErr(rt, err, exitcode.ErrForbidden, "duplicate import by other expert")
			}
			st.PutData(adt.AsStore(rt), params.PieceID.String(), params.Expert)
		} else {
			if validateCode != exitcode.Ok {
				st.Reset(rt, params.Expert)
			}

			if out.Redundancy >= st.DataStoreThreshold {
				st.Deposit(rt, params.Expert, out.PieceSize)
			}
		}
	})
	return nil
}

// FoundationChangeParams params
type FoundationChangeParams struct {
	DataStoreThreshold uint64
}

// FoundationChange update the fund config
func (a Actor) FoundationChange(rt Runtime, params *FoundationChangeParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.GovernActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		st.DataStoreThreshold = params.DataStoreThreshold
	})
	return nil
}

// BatchCheckData batch check data imported
func (a Actor) BatchCheckData(rt Runtime, params *builtin.BatchPieceCIDParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)
	store := adt.AsStore(rt)

	for _, checked := range params.PieceCIDs {
		found, err := st.HasDataID(store, checked.CID.String())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to check data in fund record")
		if !found {
			builtin.RequireNoErr(rt, err, exitcode.ErrForbidden, "failed to find data in expertfund record")
		}
	}
	return nil
}

// BatchStoreData batch store data
func (a Actor) BatchStoreData(rt Runtime, params *builtin.BatchPieceCIDParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.StorageMarketActorAddr)

	experts := make([]address.Address, 0, len(params.PieceCIDs))
	var st State
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)

		for _, checked := range params.PieceCIDs {
			expert, found, err := st.GetData(store, checked.CID.String())
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get data in fund record")
			if !found {
				builtin.RequireNoErr(rt, err, exitcode.ErrForbidden, "failed to find data in expertfund record")
			}
			experts = append(experts, expert)
		}
	})
	for i, checked := range params.PieceCIDs {
		code := rt.Send(experts[i], builtin.MethodsExpert.StoreData, &expert.ExpertDataParams{PieceID: checked.CID}, abi.NewTokenAmount(0), &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send claim amount")
	}
	return nil
}
