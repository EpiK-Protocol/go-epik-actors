package expertfund

import (
	"bytes"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
	initact "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/vote"
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
		4:                         a.OnExpertImport,
		5:                         a.ChangeThreshold,
		6:                         a.BatchCheckData,
		7:                         a.BatchStoreData,
		8:                         a.GetData,
		9:                         a.ApplyForExpert,
		10:                        a.OnEpochTickEnd,
		11:                        a.TrackNewNominated,
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

	pool := rt.StorePut(&PoolInfo{
		LastRewardBlock: abi.ChainEpoch(0),
		AccPerShare:     abi.NewTokenAmount(0),
	})
	st, err := ConstructState(adt.AsStore(rt), pool)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")
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

// OnExpertImport
func (a Actor) OnExpertImport(rt Runtime, params *builtin.OnExpertImportParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		found, err := st.HasDataID(adt.AsStore(rt), params.PieceID.String())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to check data in fund record")
		if found {
			rt.Abortf(exitcode.ErrForbidden, "duplicate import by other expert")
		}
		st.PutData(adt.AsStore(rt), params.PieceID.String(), rt.Caller())
	})
	return nil
}

type ChangeThresholdParams struct {
	DataStoreThreshold uint64
}

// ChangeThreshold update the fund config
func (a Actor) ChangeThreshold(rt Runtime, params *ChangeThresholdParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)
	builtin.ValidateCallerGranted(rt, rt.Caller(), builtin.MethodsExpertFunds.ChangeThreshold)

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

	cache := make(map[address.Address]bool)
	for _, checked := range params.PieceCIDs {
		expertAddr, found, err := st.GetData(store, checked.CID.String())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get data in fund record")
		builtin.RequireParam(rt, found, "data not found")

		qualified, ok := cache[expertAddr]
		if !ok {
			var out expert.CheckStateReturn
			code := rt.Send(expertAddr, builtin.MethodsExpert.CheckState, nil, abi.NewTokenAmount(0), &out)
			builtin.RequireSuccess(rt, code, "failed to check expert state")

			qualified = out.Qualified
			cache[expertAddr] = qualified
		}
		builtin.RequireState(rt, qualified, "expert not qualified")
	}
	return nil
}

// BatchStoreData batch store data
func (a Actor) BatchStoreData(rt Runtime, params *builtin.BatchPieceCIDParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.StorageMarketActorAddr)

	var experts []address.Address
	var rst State
	rt.StateReadonly(&rst)
	store := adt.AsStore(rt)
	for _, checked := range params.PieceCIDs {
		expert, found, err := rst.GetData(store, checked.CID.String())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get data in fund record")
		if !found {
			rt.Abortf(exitcode.ErrForbidden, "failed to find data in expertfund record")
		}
		experts = append(experts, expert)
	}

	var datas []*expert.DataOnChainInfo
	for i, checked := range params.PieceCIDs {
		var out expert.DataOnChainInfo
		code := rt.Send(experts[i], builtin.MethodsExpert.StoreData, &expert.ExpertDataParams{PieceID: checked.CID}, abi.NewTokenAmount(0), &out)
		builtin.RequireSuccess(rt, code, "failed to send claim amount")
		datas = append(datas, &out)
	}

	var st State
	rt.StateTransaction(&st, func() {
		for i, data := range datas {
			if data.Redundancy >= st.DataStoreThreshold {
				st.Deposit(rt, experts[i], data.PieceSize)
			}
		}
	})

	return nil
}

type GetDataParams struct {
	PieceID cid.Cid `checked:"true"`
}

type DataInfo struct {
	Expert address.Address
	Data   *expert.DataOnChainInfo
}

// GetData returns store data info
func (a Actor) GetData(rt Runtime, params *GetDataParams) *DataInfo {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)
	store := adt.AsStore(rt)

	expertAddr, found, err := st.GetData(store, params.PieceID.String())
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get data in fund record")
	if !found {
		rt.Abortf(exitcode.ErrForbidden, "data not found.")
	}

	var out expert.DataOnChainInfo
	code := rt.Send(expertAddr, builtin.MethodsExpert.GetData, &expert.ExpertDataParams{PieceID: params.PieceID}, abi.NewTokenAmount(0), &out)
	builtin.RequireSuccess(rt, code, "failed to get data in expert record")

	return &DataInfo{
		Expert: expertAddr,
		Data:   &out,
	}
}

type ApplyForExpertParams struct {
	Owner address.Address
	// ApplicationHash expert application hash
	ApplicationHash string
}

type ApplyForExpertReturn struct {
	IDAddress     address.Address // The canonical ID-based address for the actor.
	RobustAddress address.Address // A more expensive but re-org-safe address for the newly created actor.
}

func (a Actor) ApplyForExpert(rt Runtime, params *ApplyForExpertParams) *ApplyForExpertReturn {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	var st State
	rt.StateReadonly(&st)

	expertType := builtin.ExpertFoundation
	if st.ExpertsCount > 0 {
		expertType = builtin.ExpertNormal
	}

	// We don't resolve owner here, expert constructor will do that.
	ctorParams := expert.ConstructorParams{
		Owner:           params.Owner,
		ApplicationHash: params.ApplicationHash,
		Proposer:        rt.Caller(),
		Type:            expertType,
	}
	ctorParamBuf := new(bytes.Buffer)
	err := ctorParams.MarshalCBOR(ctorParamBuf)
	if err != nil {
		rt.Abortf(exitcode.ErrSerialization, "failed to serialize expert constructor params %v: %v", ctorParams, err)
	}
	var addresses initact.ExecReturn
	code := rt.Send(
		builtin.InitActorAddr,
		builtin.MethodsInit.Exec,
		&initact.ExecParams{
			CodeCID:           builtin.ExpertActorCodeID,
			ConstructorParams: ctorParamBuf.Bytes(),
		},
		rt.ValueReceived(), // Pass on any value to the new actor.
		&addresses,
	)
	builtin.RequireSuccess(rt, code, "failed to init new expert actor")

	rt.StateTransaction(&st, func() {
		ei := ExpertInfo{
			RewardDebt:    abi.NewTokenAmount(0),
			LockedFunds:   abi.NewTokenAmount(0),
			UnlockedFunds: abi.NewTokenAmount(0),
			VestingFunds:  rt.StorePut(ConstructVestingFunds()),
		}
		err = st.setExpert(adt.AsStore(rt), addresses.IDAddress, &ei)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to set new expert: %v", err)

		st.ExpertsCount++
	})
	return &ApplyForExpertReturn{
		IDAddress:     addresses.IDAddress,
		RobustAddress: addresses.RobustAddress,
	}
}

// Called by Expert.Nominate
func (a Actor) TrackNewNominated(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		trackedExperts, err := adt.AsSet(adt.AsStore(rt), st.TrackedExperts, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load TrackedExperts")

		k := abi.AddrKey(rt.Caller())
		has, err := trackedExperts.Has(k)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to check if expert being tracked")
		builtin.RequireParam(rt, !has, "expert already being tracked %s", rt.Caller())

		err = trackedExperts.Put(k)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put new nominated")

		st.TrackedExperts, err = trackedExperts.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush TrackedExperts")
	})
	return nil
}

// Called by Cron.
func (a Actor) OnEpochTickEnd(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.CronActorAddr)

	var st State
	rt.StateReadonly(&st)

	trackedExperts, err := adt.AsSet(adt.AsStore(rt), st.TrackedExperts, builtin.DefaultHamtBitwidth)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load TrackedExperts")

	var experts []address.Address
	err = trackedExperts.ForEach(func(k string) error {
		expertAddr, err := address.NewFromBytes([]byte(k))
		if err != nil {
			return err
		}
		experts = append(experts, expertAddr)
		return nil
	})
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to iterate TrackedExperts")

	if len(experts) == 0 {
		return nil
	}

	var out vote.GetCandidatesReturn
	code := rt.Send(builtin.VoteFundActorAddr, builtin.MethodsVote.GetCandidates, &vote.GetCandidatesParams{Addresses: experts}, abi.NewTokenAmount(0), &out)
	builtin.RequireSuccess(rt, code, "failed to call GetCandidates")

	resetExperts := make([]address.Address, 0, len(experts))
	untrackExperts := make([]address.Address, 0, len(experts))
	for i, expertAddr := range experts {
		var ret expert.OnTrackUpdateReturn
		code := rt.Send(expertAddr, builtin.MethodsExpert.OnTrackUpdate, &expert.OnTrackUpdateParams{Votes: out.Votes[i]}, abi.NewTokenAmount(0), &ret)
		builtin.RequireSuccess(rt, code, "failed to call OnTrackUpdate")

		if ret.ResetMe {
			resetExperts = append(resetExperts, expertAddr)
		}
		if ret.UntrackMe {
			untrackExperts = append(untrackExperts, expertAddr)
		}
	}
	if len(resetExperts) == 0 && len(untrackExperts) == 0 {
		return nil
	}

	rt.StateTransaction(&st, func() {
		for _, expertAddr := range resetExperts {
			err := st.Reset(rt, expertAddr)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to reset expert %s", expertAddr)
		}

		if len(untrackExperts) > 0 {
			trackedExperts, err := adt.AsSet(adt.AsStore(rt), st.TrackedExperts, builtin.DefaultHamtBitwidth)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load TrackedExperts")

			for _, untrackAddr := range untrackExperts {
				err := trackedExperts.Delete(abi.AddrKey(untrackAddr))
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to delete tracked expert %s", untrackAddr)
			}

			st.TrackedExperts, err = trackedExperts.Root()
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush TrackedExperts")
		}
	})
	return nil
}
