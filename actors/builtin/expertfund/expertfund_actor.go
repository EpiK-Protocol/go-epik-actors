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
		11:                        a.OnExpertNominated,
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
		TotalExpertDataSize: 0,
		AccPerShare:         abi.NewTokenAmount(0),
		LastRewardBalance:   abi.NewTokenAmount(0),
	})
	st, err := ConstructState(adt.AsStore(rt), pool)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")
	rt.StateCreate(st)
	return nil
}

// ApplyRewards apply the received value into the fund balance.
func (a Actor) ApplyRewards(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.RewardActorAddr)
	builtin.RequireParam(rt, rt.ValueReceived().GreaterThan(big.Zero()), "balance to add must be greater than zero")
	return nil
}

// ClaimFundParams params
type ClaimFundParams struct {
	Expert address.Address
	Amount abi.TokenAmount
}

// Claim claim the received value into the balance.
func (a Actor) Claim(rt Runtime, params *ClaimFundParams) *abi.TokenAmount {
	builtin.RequireParam(rt, params.Amount.GreaterThan(big.Zero()), "non-positive amount")

	var actual abi.TokenAmount
	var st State
	rt.StateTransaction(&st, func() {
		rt.ValidateImmediateCallerIs(builtin.RequestExpertControlAddr(rt, params.Expert))

		var err error
		actual, err = st.Claim(rt, params.Expert, params.Amount)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to claim expert fund: %s, %s", params.Expert, params.Amount)
	})
	if !actual.IsZero() {
		code := rt.Send(rt.Caller(), builtin.MethodSend, nil, actual, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send claimed fund")
	}
	return &actual
}

// OnExpertImport
func (a Actor) OnExpertImport(rt Runtime, params *builtin.CheckedCID) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		dataByPiece, err := adt.AsMap(adt.AsStore(rt), st.DataByPiece, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load DataByPiece")

		absent, err := dataByPiece.PutIfAbsent(abi.CidKey(params.CID), &DataInfo{
			Expert:    rt.Caller(),
			Deposited: false,
		})
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put absent data: %s %s", params.CID, rt.Caller())
		builtin.RequireParam(rt, absent, "duplicate imported data: %s, %s", params.CID, rt.Caller())

		st.DataByPiece, err = dataByPiece.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush DataByPiece")
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

type BatchCheckDataParams struct {
	CheckedPieces []CheckedPiece
}

type CheckedPiece struct {
	PieceCID  cid.Cid `checked:"true"`
	PieceSize abi.PaddedPieceSize
}

// BatchCheckData batch check data imported
func (a Actor) BatchCheckData(rt Runtime, params *BatchCheckDataParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)

	checkedPieces := make([]cid.Cid, 0, len(params.CheckedPieces))
	checkedPieceSizes := make(map[cid.Cid]abi.PaddedPieceSize)
	for _, cp := range params.CheckedPieces {
		checkedPieces = append(checkedPieces, cp.PieceCID)
		checkedPieceSizes[cp.PieceCID] = cp.PieceSize
	}
	pieceToInfo, err := st.GetDataInfos(adt.AsStore(rt), checkedPieces...)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get data in fund record")

	expertPieces := make(map[address.Address]*builtin.BatchPieceCIDParams)
	// Check expert status
	for pieceCID, dataInfo := range pieceToInfo {
		batch, ok := expertPieces[dataInfo.Expert]
		if !ok {
			var out expert.CheckStateReturn
			code := rt.Send(dataInfo.Expert, builtin.MethodsExpert.CheckState, nil, abi.NewTokenAmount(0), &out)
			builtin.RequireSuccess(rt, code, "failed to check expert state: %s", dataInfo.Expert)
			builtin.RequireState(rt, out.Qualified, "unqualified expert: %s", dataInfo.Expert)

			batch = &builtin.BatchPieceCIDParams{}
			expertPieces[dataInfo.Expert] = batch
		}
		batch.PieceCIDs = append(batch.PieceCIDs, builtin.CheckedCID{CID: pieceCID})
	}

	// Check piece size
	for expertAddr, pieces := range expertPieces {
		var out expert.GetDatasReturn
		code := rt.Send(expertAddr, builtin.MethodsExpert.GetDatas, pieces, big.Zero(), &out)
		builtin.RequireSuccess(rt, code, "failed to get datas from expert: %s", expertAddr)

		for _, info := range out.Infos {
			pieceCID, err := cid.Decode(info.PieceID)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed decode piece CID %s", info.PieceID)
			builtin.RequireState(rt, checkedPieceSizes[pieceCID] == info.PieceSize,
				"piece size mismatched, checked %d, registered %d", checkedPieceSizes[pieceCID], info.PieceSize)
		}
	}
	return nil
}

// BatchStoreData batch store data
func (a Actor) BatchStoreData(rt Runtime, params *builtin.BatchPieceCIDParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.StorageMarketActorAddr)

	if len(params.PieceCIDs) == 0 {
		return nil
	}

	var st State
	rt.StateReadonly(&st)
	store := adt.AsStore(rt)

	dbp, err := adt.AsMap(store, st.DataByPiece, builtin.DefaultHamtBitwidth)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load DataByPiece")

	pieceToDataInfo := make(map[string]DataInfo)
	expertToPieces := make(map[address.Address][]builtin.CheckedCID)

	for _, checked := range params.PieceCIDs {

		var dataInfo DataInfo
		found, err := dbp.Get(abi.CidKey(checked.CID), &dataInfo)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get DataInfo %s", checked.CID)
		builtin.RequireParam(rt, found, "DataInfo not found %s", checked.CID)

		expertToPieces[dataInfo.Expert] = append(expertToPieces[dataInfo.Expert], checked)

		_, exist := pieceToDataInfo[checked.CID.String()]
		builtin.RequireParam(rt, !exist, "duplicated data %s", checked.CID)
		pieceToDataInfo[checked.CID.String()] = dataInfo
	}

	var onchainInfos []*expert.DataOnChainInfo
	for exp, pieces := range expertToPieces {
		var out expert.StoreDataReturn
		code := rt.Send(exp, builtin.MethodsExpert.StoreData, &builtin.BatchPieceCIDParams{PieceCIDs: pieces}, abi.NewTokenAmount(0), &out)
		builtin.RequireSuccess(rt, code, "failed to store data of %s", exp)

		onchainInfos = append(onchainInfos, out.Infos...)
	}

	rt.StateTransaction(&st, func() {
		expertDepositSize := make(map[address.Address]abi.PaddedPieceSize)

		for _, onchainInfo := range onchainInfos {
			di, ok := pieceToDataInfo[onchainInfo.PieceID]
			builtin.RequireParam(rt, ok, "data info not exist: %s", onchainInfo.PieceID)

			if di.Deposited {
				continue
			}

			if onchainInfo.Redundancy >= st.DataStoreThreshold {
				expertDepositSize[di.Expert] += onchainInfo.PieceSize

				di.Deposited = true
				pcid, err := cid.Decode(onchainInfo.PieceID)
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed decode piece CID %s", onchainInfo.PieceID)
				err = dbp.Put(abi.CidKey(pcid), &di)
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put DataInfo for %s", onchainInfo.PieceID)
			}
		}

		err := st.Deposit(rt, expertDepositSize)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to deposit")

		st.DataByPiece, err = dbp.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush DataByPiece")
	})

	return nil
}

type GetDataReturn struct {
	Expert address.Address
	Data   *expert.DataOnChainInfo
}

// GetData returns store data info
func (a Actor) GetData(rt Runtime, params *builtin.CheckedCID) *GetDataReturn {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)

	pieceToInfo, err := st.GetDataInfos(adt.AsStore(rt), params.CID)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get data in fund record")

	for _, info := range pieceToInfo {
		var out expert.GetDatasReturn
		code := rt.Send(info.Expert, builtin.MethodsExpert.GetDatas, &builtin.BatchPieceCIDParams{PieceCIDs: []builtin.CheckedCID{*params}}, abi.NewTokenAmount(0), &out)
		builtin.RequireSuccess(rt, code, "failed to get data in expert record")

		return &GetDataReturn{
			Expert: info.Expert,
			Data:   &out.Infos[0],
		}
	}
	return nil
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
		emptyMapCid, err := adt.StoreEmptyMap(adt.AsStore(rt), builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to create empty vestingFunds")
		ei := ExpertInfo{
			RewardDebt:    abi.NewTokenAmount(0),
			LockedFunds:   abi.NewTokenAmount(0),
			UnlockedFunds: abi.NewTokenAmount(0),
			VestingFunds:  emptyMapCid,
			Active:        false,
		}
		err = st.SetExpert(adt.AsStore(rt), addresses.IDAddress, &ei, true)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to set new expert: %v", err)

		st.ExpertsCount++
	})
	return &ApplyForExpertReturn{
		IDAddress:     addresses.IDAddress,
		RobustAddress: addresses.RobustAddress,
	}
}

// Called by Expert.OnNominate
func (a Actor) OnExpertNominated(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		err := st.ActivateExpert(rt, rt.Caller())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to activate expert %s", rt.Caller())

		trackedExperts, err := adt.AsSet(adt.AsStore(rt), st.TrackedExperts, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load TrackedExperts")

		absent, err := trackedExperts.PutIfAbsent(abi.AddrKey(rt.Caller()))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add tracked expert %s", rt.Caller())
		builtin.RequireParam(rt, absent, "expert already on track %s", rt.Caller())

		st.TrackedExperts, err = trackedExperts.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush TrackedExperts")
	})
	return nil
}

// Called by Cron.
// 	1. track experts' votes and update their status
func (a Actor) OnEpochTickEnd(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.CronActorAddr)

	var st State
	rt.StateReadonly(&st)

	experts, err := st.ListTrackedExperts(adt.AsStore(rt))
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load TrackedExperts")

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

	burn := abi.NewTokenAmount(0)
	rt.StateTransaction(&st, func() {
		burn, err = st.InactivateExperts(rt, resetExperts)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to inactivate experts")

		err := st.DeleteTrackedExperts(adt.AsStore(rt), untrackExperts)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to delete tracked experts")
	})
	if burn.GreaterThan(big.Zero()) {
		code := rt.Send(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, burn, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to burn funds")
	}
	return nil
}
