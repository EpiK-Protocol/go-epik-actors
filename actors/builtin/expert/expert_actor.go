package expert

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
	_ "github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/ipfs/go-cid"
)

type Runtime = runtime.Runtime

// Actor of expert
type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.ControlAddress,
		3:                         a.ChangeOwner,
		4:                         a.ImportData,
		5:                         a.GetDatas,
		6:                         a.StoreData,
		7:                         a.Nominate,
		8:                         a.OnNominated,
		9:                         a.OnBlocked,
		10:                        a.OnImplicated,
		11:                        a.GovChangeOwner,
		12:                        a.CheckState,
		13:                        a.OnVotesUpdated,
	}
}

func (a Actor) Code() cid.Cid {
	return builtin.ExpertActorCodeID
}

func (a Actor) State() cbor.Er {
	return new(State)
}

var _ runtime.VMActor = Actor{}

type ConstructorParams struct {
	Owner addr.Address // ID address
	// ApplicationHash expert application hash
	ApplicationHash string

	// owner itself or agent, diffs from ExpertInfo.Proposer
	Proposer addr.Address // ID address

	// Type expert type
	Type builtin.ExpertType
}

func (a Actor) Constructor(rt Runtime, params *ConstructorParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.InitActorAddr)

	if params.Type != builtin.ExpertFoundation {
		builtin.RequireParam(rt, rt.ValueReceived().GreaterThanEqual(ExpertApplyCost), "fund for expert proposal not enough")
		code := rt.Send(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, ExpertApplyCost, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to burn funds")

		change := big.Sub(rt.ValueReceived(), ExpertApplyCost)
		if !change.IsZero() {
			code := rt.Send(params.Proposer, builtin.MethodSend, nil, change, &builtin.Discard{})
			builtin.RequireSuccess(rt, code, "failed to send change funds")
		}
	}

	owner := resolveOwnerAddress(rt, params.Owner)
	info := &ExpertInfo{
		Owner:              owner,
		Proposer:           owner, // same with owner means new registered expert
		Type:               params.Type,
		ApplicationHash:    params.ApplicationHash,
		ApplyNewOwner:      owner,
		ApplyNewOwnerEpoch: -1,
	}
	infoCid := rt.StorePut(info)

	eState := ExpertStateRegistered
	if info.Type == builtin.ExpertFoundation {
		eState = ExpertStateQualified
	}

	st, err := ConstructState(adt.AsStore(rt), infoCid, eState)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct initial state")
	rt.StateCreate(st)
	return nil
}

func (a Actor) ControlAddress(rt Runtime, _ *abi.EmptyValue) *addr.Address {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)

	info, changed := checkAndGetExpertInfo(rt, &st)
	if changed {
		rt.StateTransaction(&st, func() {
			info.ApplyNewOwnerEpoch = -1 // clear
			err := st.SaveInfo(adt.AsStore(rt), info)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "faile to save new owner")
		})
	}
	return &info.Owner
}

func checkAndGetExpertInfo(rt Runtime, st *State) (info *ExpertInfo, ownerChanged bool) {
	store := adt.AsStore(rt)

	info, err := st.GetInfo(store)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get info")

	if info.ApplyNewOwnerEpoch > 0 && info.Owner != info.ApplyNewOwner &&
		(rt.CurrEpoch()-info.ApplyNewOwnerEpoch) >= ActivateNewOwnerDelay {
		info.Owner = info.ApplyNewOwner
		return info, true
	}
	return info, false
}

// Resolves an address to an ID address and verifies that it is address of an account or multisig actor.
func resolveOwnerAddress(rt Runtime, raw addr.Address) addr.Address {
	resolved, ok := rt.ResolveAddress(raw)
	builtin.RequireParam(rt, ok, "unable to resolve address %v", raw)

	ownerCode, ok := rt.GetActorCodeCID(resolved)
	builtin.RequireParam(rt, ok, "no code for address %v", resolved)
	builtin.RequireParam(rt, builtin.IsPrincipal(ownerCode), "owner actor type must be a principal, was %v", ownerCode)
	return resolved
}

func (a Actor) ChangeOwner(rt Runtime, newOwner *addr.Address) *abi.EmptyValue {
	resolvedNewOwner := resolveOwnerAddress(rt, *newOwner)

	var st State
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)

		info, _ := checkAndGetExpertInfo(rt, &st)
		rt.ValidateImmediateCallerIs(info.Owner)

		info.Owner = resolvedNewOwner
		info.ApplyNewOwnerEpoch = -1

		err := st.SaveInfo(store, info)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to change owner")
	})
	return nil
}

type BatchImportDataParams struct {
	Datas []ImportDataParams
}

type ImportDataParams struct {
	RootID    cid.Cid `checked:"true"`
	PieceID   cid.Cid `checked:"true"`
	PieceSize abi.PaddedPieceSize
}

func (a Actor) ImportData(rt Runtime, params *BatchImportDataParams) *abi.EmptyValue {
	var st State
	store := adt.AsStore(rt)
	rt.StateTransaction(&st, func() {
		builtin.RequireState(rt, st.ExpertState.Qualified(), "unqualified expert %s", rt.Receiver())

		info, ownerChanged := checkAndGetExpertInfo(rt, &st)
		rt.ValidateImmediateCallerIs(info.Owner)
		if ownerChanged {
			info.ApplyNewOwnerEpoch = -1
			err := st.SaveInfo(store, info)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "faile to apply new owner")
		}

		for _, data := range params.Datas {
			infos, err := st.GetDatas(store, false, data.PieceID)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get data")
			builtin.RequireParam(rt, len(infos) == 0, "duplicate data %s", data.PieceID)

			err = st.PutDatas(store, &DataOnChainInfo{
				RootID:    data.RootID.String(),
				PieceID:   data.PieceID.String(),
				PieceSize: data.PieceSize,
			})
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to import data")
			st.DataCount++
		}
	})
	cids := []cid.Cid{}
	for _, data := range params.Datas {
		cids = append(cids, data.PieceID)
	}
	builtin.NotifyExpertImport(rt, cids)
	return nil
}

type GetDatasReturn struct {
	Infos         []*DataOnChainInfo
	ExpertBlocked bool
}

func (a Actor) GetDatas(rt Runtime, params *builtin.BatchPieceCIDParams) *GetDatasReturn {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)

	pieceCIDs := make([]cid.Cid, 0, len(params.PieceCIDs))
	for _, checked := range params.PieceCIDs {
		pieceCIDs = append(pieceCIDs, checked.CID)
	}

	infos, err := st.GetDatas(adt.AsStore(rt), true, pieceCIDs...)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get datas")

	return &GetDatasReturn{
		Infos:         infos,
		ExpertBlocked: st.ExpertState == ExpertStateBlocked,
	}
}

func (a Actor) StoreData(rt Runtime, params *builtin.BatchPieceCIDParams) *GetDatasReturn {
	rt.ValidateImmediateCallerIs(builtin.ExpertFundActorAddr)

	pieceCIDs := make([]cid.Cid, 0, len(params.PieceCIDs))
	for _, checked := range params.PieceCIDs {
		pieceCIDs = append(pieceCIDs, checked.CID)
	}

	var ret GetDatasReturn

	store := adt.AsStore(rt)
	var st State
	rt.StateTransaction(&st, func() {
		infos, err := st.GetDatas(store, true, pieceCIDs...)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get datas")

		for _, info := range infos {
			info.Redundancy++
		}

		err = st.PutDatas(store, infos...)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put datas")

		ret.Infos = infos
		ret.ExpertBlocked = st.ExpertState == ExpertStateBlocked
	})
	return &ret
}

func (a Actor) Nominate(rt Runtime, nominatedExpert *addr.Address) *abi.EmptyValue {

	var st State
	rt.StateReadonly(&st)

	builtin.RequireState(rt, st.ExpertState.Qualified(), "nominator is unqualified")

	resolved, ok := rt.ResolveAddress(*nominatedExpert)
	builtin.RequireParam(rt, ok, "failed to resolve address %s", *nominatedExpert)
	actorCode, ok := rt.GetActorCodeCID(resolved)
	builtin.RequireParam(rt, ok, "failed to get actor code ", *nominatedExpert)
	builtin.RequireParam(rt, actorCode == builtin.ExpertActorCodeID, "not an expert address %s", *nominatedExpert)

	info, changed := checkAndGetExpertInfo(rt, &st)
	rt.ValidateImmediateCallerIs(info.Owner)
	if changed {
		rt.StateTransaction(&st, func() {
			info.ApplyNewOwnerEpoch = -1
			err := st.SaveInfo(adt.AsStore(rt), info)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "faile to save expert info")
		})
	}

	code := rt.Send(resolved, builtin.MethodsExpert.OnNominated, nil, abi.NewTokenAmount(0), &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to nominate expert")
	return nil
}

func (a Actor) OnNominated(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

	store := adt.AsStore(rt)
	var st State
	rt.StateTransaction(&st, func() {
		info, err := st.GetInfo(store)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get info")
		builtin.RequireParam(rt, info.Type != builtin.ExpertFoundation, "foundation expert cannot be nominated")
		builtin.RequireParam(rt, st.ExpertState == ExpertStateRegistered, "nominate expert with invalid status")

		info.Proposer = rt.Caller()
		err = st.SaveInfo(store, info)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save info")

		st.ExpertState = ExpertStateUnqualified
	})
	return nil
}

type OnBlockedReturn struct {
	ImplicatedExpert            addr.Address // ID address
	ImplicatedExpertVotesEnough bool
}

func (a Actor) OnBlocked(rt Runtime, _ *abi.EmptyValue) *OnBlockedReturn {
	rt.ValidateImmediateCallerIs(builtin.ExpertFundActorAddr)

	var st State
	var info *ExpertInfo
	rt.StateTransaction(&st, func() {
		var err error
		info, err = st.GetInfo(adt.AsStore(rt))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get expert info")

		builtin.RequireParam(rt, info.Type != builtin.ExpertFoundation, "foundation expert cannot be blocked")
		builtin.RequireParam(rt, st.ExpertState == ExpertStateQualified || st.ExpertState == ExpertStateUnqualified, "try to block expert with invalid state %s", st.ExpertState)

		st.ExpertState = ExpertStateBlocked
		st.CurrentVotes = abi.NewTokenAmount(0)
	})

	var votesEnough cbg.CborBool
	code := rt.Send(info.Proposer, builtin.MethodsExpert.OnImplicated, nil, abi.NewTokenAmount(0), &votesEnough)
	builtin.RequireSuccess(rt, code, "failed to call OnImplicated")

	return &OnBlockedReturn{
		ImplicatedExpert:            info.Proposer,
		ImplicatedExpertVotesEnough: bool(votesEnough),
	}
}

func (a Actor) OnImplicated(rt Runtime, _ *abi.EmptyValue) *cbg.CborBool {
	rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

	votesEnough := cbg.CborBool(true)

	var st State
	rt.StateTransaction(&st, func() {
		info, err := st.GetInfo(adt.AsStore(rt))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get expert info")

		if info.Type == builtin.ExpertFoundation || st.ExpertState == ExpertStateBlocked {
			return
		}
		st.ImplicatedTimes++

		if st.CurrentVotes.LessThan(st.VoteThreshold()) {
			votesEnough = cbg.CborBool(false)
			st.ExpertState = ExpertStateUnqualified
		}
	})

	return &votesEnough
}

func (a Actor) GovChangeOwner(rt Runtime, newOwner *addr.Address) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)
	builtin.ValidateCallerGranted(rt, rt.Caller(), builtin.MethodsExpert.GovChangeOwner)

	builtin.RequireParam(rt, !newOwner.Empty(), "empty address")
	builtin.RequireParam(rt, newOwner.Protocol() == addr.ID, "owner address must be an ID address")

	var st State
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)

		info, err := st.GetInfo(store)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get expert info")

		info.ApplyNewOwner = *newOwner
		info.ApplyNewOwnerEpoch = rt.CurrEpoch()

		err = st.SaveInfo(store, info)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save info")
	})
	return nil
}

func (a Actor) CheckState(rt Runtime, _ *abi.EmptyValue) *builtin.CheckExpertStateReturn {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)
	return &builtin.CheckExpertStateReturn{
		AllowVote: st.ExpertState.AllowVote(),
		Qualified: st.ExpertState.Qualified(),
	}
}

type OnVotesUpdatedReturn struct {
	VotesEnough bool
}

func (a Actor) OnVotesUpdated(rt Runtime, params *builtin.OnExpertVotesUpdatedParams) *OnVotesUpdatedReturn {
	rt.ValidateImmediateCallerIs(builtin.ExpertFundActorAddr)

	var ret OnVotesUpdatedReturn

	var st State
	rt.StateTransaction(&st, func() {
		st.CurrentVotes = params.Votes

		info, err := st.GetInfo(adt.AsStore(rt))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get expert info")

		if info.Type == builtin.ExpertFoundation {
			ret.VotesEnough = true
			return
		}

		builtin.RequireState(rt, st.ExpertState == ExpertStateQualified || st.ExpertState == ExpertStateUnqualified, "unexpected expert state %d", st.ExpertState)

		if st.CurrentVotes.GreaterThanEqual(st.VoteThreshold()) {
			st.ExpertState = ExpertStateQualified
			ret.VotesEnough = true
		} else {
			st.ExpertState = ExpertStateUnqualified
			ret.VotesEnough = false
		}
	})

	return &ret
}
