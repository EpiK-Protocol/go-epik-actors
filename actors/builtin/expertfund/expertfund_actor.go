package expertfund

import (
	"bytes"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
	initact "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"

	rtt "github.com/filecoin-project/go-state-types/rt"
)

type Actor struct{}

type Runtime = runtime.Runtime

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.ApplyForExpert,
		3:                         a.OnExpertImport,
		4:                         a.GetData,
		5:                         a.BatchCheckData,
		6:                         a.BatchStoreData,
		7:                         a.ChangeThreshold,
		8:                         a.Claim,
		9:                         a.BlockExpert,
		10:                        a.OnExpertVotesUpdated,
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
		AccPerShare:       abi.NewTokenAmount(0),
		LastRewardBalance: abi.NewTokenAmount(0),
	})
	st, err := ConstructState(adt.AsStore(rt), pool)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")
	rt.StateCreate(st)
	return nil
}

// ClaimFundParams params
type ClaimFundParams struct {
	Expert address.Address
	Amount abi.TokenAmount
}

// Claim claim the received value into the balance.
func (a Actor) Claim(rt Runtime, params *ClaimFundParams) *abi.TokenAmount {
	builtin.RequireParam(rt, params.Amount.GreaterThanEqual(big.Zero()), "negative amount requested: %s", params.Amount)

	rt.ValidateImmediateCallerIs(builtin.RequestExpertControlAddr(rt, params.Expert))

	var actual abi.TokenAmount
	var st State
	rt.StateTransaction(&st, func() {
		var err error
		actual, err = st.Claim(rt, params.Expert, params.Amount)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to claim expert fund: %s, %s", params.Expert, params.Amount)
	})
	if actual.GreaterThan(big.Zero()) {
		code := rt.Send(rt.Caller(), builtin.MethodSend, nil, actual, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send claimed fund")
	}
	return &actual
}

// OnExpertImport
func (a Actor) OnExpertImport(rt Runtime, params *builtin.BatchPieceCIDParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

	builtin.RequireParam(rt, len(params.PieceCIDs) > 0, "undefined cid")

	var st State
	rt.StateTransaction(&st, func() {
		// global unique
		for _, checked := range params.PieceCIDs {
			err := st.PutPieceInfos(adt.AsStore(rt), true, map[cid.Cid]*PieceInfo{
				checked.CID: {rt.Caller(), st.DataStoreThreshold},
			})
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put data info: %s, %s", rt.Caller(), checked.CID)
		}
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

	pieceCIDs := make([]cid.Cid, 0, len(params.CheckedPieces))
	checkedPieceSizes := make(map[string]abi.PaddedPieceSize)
	for _, cp := range params.CheckedPieces {
		pieceCIDs = append(pieceCIDs, cp.PieceCID)
		checkedPieceSizes[cp.PieceCID.String()] = cp.PieceSize
	}
	pieceToExpert, _, err := st.GetPieceInfos(adt.AsStore(rt), pieceCIDs...)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get piece info")
	builtin.RequireParam(rt, len(pieceToExpert) == len(pieceCIDs), "duplicate piece")

	expertPieces := make(map[address.Address][]builtin.CheckedCID)
	for pieceCID, expertAddr := range pieceToExpert {
		expertPieces[expertAddr] = append(expertPieces[expertAddr], builtin.CheckedCID{CID: pieceCID})
	}

	// Check piece size
	for expertAddr, pieceCIDs := range expertPieces {
		var out expert.GetDatasReturn
		code := rt.Send(expertAddr, builtin.MethodsExpert.GetDatas, &builtin.BatchPieceCIDParams{PieceCIDs: pieceCIDs}, big.Zero(), &out)
		builtin.RequireSuccess(rt, code, "failed to get datas from expert: %s", expertAddr)
		builtin.RequireState(rt, !out.ExpertBlocked, "expert blocked %s", expertAddr)

		for _, info := range out.Infos {
			builtin.RequireState(rt, checkedPieceSizes[info.PieceID] == info.PieceSize,
				"piece size mismatched, checked %d, registered %d", checkedPieceSizes[info.PieceID], info.PieceSize)
		}
	}
	return nil
}

// BatchStoreData batch store data
func (a Actor) BatchStoreData(rt Runtime, params *builtin.BatchPieceCIDParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.StorageMarketActorAddr)

	rt.Log(rtt.WARN, "expertfund BatchStoreData %v", params.PieceCIDs)

	if len(params.PieceCIDs) == 0 {
		return nil
	}

	var st State
	rt.StateReadonly(&st)
	store := adt.AsStore(rt)

	pieceCIDs := make([]cid.Cid, 0, len(params.PieceCIDs))
	for _, checked := range params.PieceCIDs {
		pieceCIDs = append(pieceCIDs, checked.CID)
	}

	pieceToExpert, pieceToThreshold, err := st.GetPieceInfos(store, pieceCIDs...)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get data infos")
	builtin.RequireParam(rt, len(pieceToExpert) == len(pieceCIDs), "duplicate piece")

	rt.Log(rtt.WARN, "expertfund BatchStoreData 2")

	expertToPieces := make(map[address.Address][]builtin.CheckedCID)
	for piece, expertAddr := range pieceToExpert {
		expertToPieces[expertAddr] = append(expertToPieces[expertAddr], builtin.CheckedCID{CID: piece})
	}

	var onchainInfos []*expert.DataOnChainInfo
	expertToInfo := make(map[address.Address]*ExpertInfo)
	for expertAddr, pieces := range expertToPieces {
		var out expert.GetDatasReturn
		code := rt.Send(expertAddr, builtin.MethodsExpert.StoreData, &builtin.BatchPieceCIDParams{PieceCIDs: pieces}, abi.NewTokenAmount(0), &out)
		builtin.RequireSuccess(rt, code, "failed to store data of %s", expertAddr)
		builtin.RequireState(rt, !out.ExpertBlocked, "expert blocked %s", expertAddr)

		onchainInfos = append(onchainInfos, out.Infos...)

		expertToInfo[expertAddr], err = st.GetExpert(store, expertAddr)
		rt.Log(rtt.WARN, "expertfund BatchStoreData 3, expert blocked %s, %t, err: %v", expertAddr, out.ExpertBlocked, err)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get expert info")
	}

	expertDepositSize := make(map[address.Address]abi.PaddedPieceSize)
	for _, info := range onchainInfos {
		pieceID, err := cid.Parse(info.PieceID)
		rt.Log(rtt.WARN, "expertfund BatchStoreData parse piece %s, err: %v", info.PieceID, err)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to parse info pieceID")
		if info.Redundancy == pieceToThreshold[pieceID] {
			expertAddr := pieceToExpert[pieceID]
			eInfo, ok := expertToInfo[expertAddr]
			builtin.RequireState(rt, ok, "expert of piece %s not found", info.PieceID)

			rt.Log(rtt.WARN, "expertfund BatchStoreData 4, piece %s, expert %s, active %t", pieceID, expertAddr, eInfo.Active)

			if !eInfo.Active {
				continue
			}
			expertDepositSize[expertAddr] += info.PieceSize
		}
	}

	if len(expertDepositSize) > 0 {
		rt.StateTransaction(&st, func() {
			err := st.Deposit(rt, expertDepositSize)
			fmt.Println("expertfund BatchStoreData, deposit: ", expertDepositSize, err == nil, len(err.Error()))
			fmt.Println("expertfund BatchStoreData, deposit error: ", err.Error())
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to deposit")
		})
	}

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

	pieceToExpert, _, err := st.GetPieceInfos(adt.AsStore(rt), params.CID)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get data in fund record")

	expertAddr := pieceToExpert[params.CID]
	var out expert.GetDatasReturn
	code := rt.Send(expertAddr, builtin.MethodsExpert.GetDatas, &builtin.BatchPieceCIDParams{PieceCIDs: []builtin.CheckedCID{*params}}, abi.NewTokenAmount(0), &out)
	builtin.RequireSuccess(rt, code, "failed to get data in expert record")
	builtin.RequireState(rt, !out.ExpertBlocked, "expert blocked %s", expertAddr)

	return &GetDataReturn{
		Expert: expertAddr,
		Data:   out.Infos[0],
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

	store := adt.AsStore(rt)
	var st State
	rt.StateReadonly(&st)

	expertType := builtin.ExpertFoundation
	experts, err := st.ListExperts(store)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to list experts")
	if len(experts) > 0 {
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
	err = ctorParams.MarshalCBOR(ctorParamBuf)
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
		emptyMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to create empty vestingFunds")
		ei := ExpertInfo{
			RewardDebt:    abi.NewTokenAmount(0),
			LockedFunds:   abi.NewTokenAmount(0),
			UnlockedFunds: abi.NewTokenAmount(0),
			VestingFunds:  emptyMapCid,
			Active:        expertType == builtin.ExpertFoundation,
		}
		err = st.SetExpert(store, addresses.IDAddress, &ei, true)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to set new expert: %v", err)
	})
	return &ApplyForExpertReturn{
		IDAddress:     addresses.IDAddress,
		RobustAddress: addresses.RobustAddress,
	}
}

func (a Actor) BlockExpert(rt Runtime, expertAddr *address.Address) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)
	builtin.ValidateCallerGranted(rt, rt.Caller(), builtin.MethodsExpertFunds.BlockExpert)

	blockedExpert, ok := rt.ResolveAddress(*expertAddr)
	builtin.RequireParam(rt, ok, "failed to resolve address %s", *expertAddr)

	expertCode, ok := rt.GetActorCodeCID(blockedExpert)
	builtin.RequireParam(rt, ok, "failed to get actor code id of %s", *expertAddr)
	builtin.RequireParam(rt, expertCode == builtin.ExpertActorCodeID, "not an expert actor")

	var blockRet expert.OnBlockedReturn
	code := rt.Send(blockedExpert, builtin.MethodsExpert.OnBlocked, nil, abi.NewTokenAmount(0), &blockRet)
	builtin.RequireSuccess(rt, code, "failed to block expert %s", *expertAddr)

	code = rt.Send(builtin.VoteFundActorAddr, builtin.MethodsVote.OnCandidateBlocked, &blockedExpert, abi.NewTokenAmount(0), &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to notify vote expert blocked")

	var burned abi.TokenAmount

	var st State
	store := adt.AsStore(rt)
	rt.StateTransaction(&st, func() {
		var err error
		if !blockRet.ImplicatedExpertVotesEnough {
			burned, err = st.DeactivateExperts(rt, map[address.Address]bool{
				blockRet.ImplicatedExpert: false,
				blockedExpert:             true,
			})
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to deactivate experts")
		} else {
			burned, err = st.DeactivateExperts(rt, map[address.Address]bool{
				blockedExpert: true,
			})
			// should never happen
			err = st.DeleteDisqualifiedExpertInfo(store, blockRet.ImplicatedExpert)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to delete disqualified for implication")
		}
	})
	if burned.GreaterThan(big.Zero()) {
		code := rt.Send(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, burned, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to burn funds")
	}

	return nil
}

func (a Actor) OnExpertVotesUpdated(rt Runtime, params *builtin.OnExpertVotesUpdatedParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.VoteFundActorAddr)

	var out expert.OnVotesUpdatedReturn
	code := rt.Send(params.Expert, builtin.MethodsExpert.OnVotesUpdated, params, abi.NewTokenAmount(0), &out)
	builtin.RequireSuccess(rt, code, "failed to check expert state")

	var st State
	rt.StateTransaction(&st, func() {
		if !out.VotesEnough {
			_, err := st.DeactivateExperts(rt, map[address.Address]bool{
				params.Expert: false,
			})
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to deactivate expert for votes changed")
		} else {
			err := st.ActivateExpert(rt, params.Expert)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to activate expert")
		}
	})

	return nil
}
