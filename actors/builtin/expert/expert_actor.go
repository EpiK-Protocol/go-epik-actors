package expert

import (
	"github.com/filecoin-project/go-address"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"

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
		5:                         a.GetData,
		6:                         a.StoreData,
		7:                         a.Nominate,
		8:                         a.OnNominated,
		9:                         a.GovBlock,
		10:                        a.OnImplicated,
		11:                        a.GovChangeOwner,
		12:                        a.OnTrackUpdate,
		13:                        a.CheckState,
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
		eState = ExpertStateNormal
	}

	st, err := ConstructState(adt.AsStore(rt), infoCid, eState)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct initial state")
	rt.StateCreate(st)
	return nil
}

func (a Actor) ControlAddress(rt Runtime, _ *abi.EmptyValue) *builtin.ExpertControlAddressReturn {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)

	info, changed := checkAndGetExpertInfo(rt, &st)
	if changed {
		rt.StateTransaction(&st, func() {
			info.ApplyNewOwnerEpoch = -1 // clear
			err := st.SaveInfo(adt.AsStore(rt), info)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "faile to save expert info")
		})
	}
	return &builtin.ExpertControlAddressReturn{Owner: info.Owner}
}

func checkAndGetExpertInfo(rt Runtime, st *State) (info *ExpertInfo, ownerChanged bool) {
	store := adt.AsStore(rt)

	info, err := st.GetInfo(store)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get info")

	if info.ApplyNewOwnerEpoch > 0 && info.Owner != info.ApplyNewOwner &&
		(rt.CurrEpoch()-info.ApplyNewOwnerEpoch) >= NewOwnerActivateDelay {
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

type ExpertDataParams struct {
	RootID    string
	PieceID   cid.Cid `checked:"true"`
	PieceSize abi.PaddedPieceSize
}

func (a Actor) ImportData(rt Runtime, params *ExpertDataParams) *abi.EmptyValue {
	var st State
	store := adt.AsStore(rt)
	rt.StateTransaction(&st, func() {
		builtin.RequireState(rt, st.Status.Qualified(), "expert is unqualified")

		info, ownerChanged := checkAndGetExpertInfo(rt, &st)
		rt.ValidateImmediateCallerIs(info.Owner)
		if ownerChanged {
			info.ApplyNewOwnerEpoch = -1
			err := st.SaveInfo(store, info)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "faile to save expert info")
		}

		_, found, err := st.GetData(store, params.PieceID.String())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get data")
		builtin.RequireParam(rt, !found, "duplicate expert import")

		newDataInfo := &DataOnChainInfo{
			RootID:    params.RootID,
			PieceID:   params.PieceID.String(),
			PieceSize: params.PieceSize,
		}

		err = st.PutData(store, newDataInfo)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to import data")
		st.DataCount++
	})
	builtin.NotifyExpertImport(rt, params.PieceID)
	return nil
}

func (a Actor) GetData(rt Runtime, params *ExpertDataParams) *DataOnChainInfo {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)
	store := adt.AsStore(rt)

	data, found, err := st.GetData(store, params.PieceID.String())
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load expert data: %s", params.PieceID)
	builtin.RequireParam(rt, found, "data not imported: %s", params.PieceID)
	return data
}

func (a Actor) StoreData(rt Runtime, params *ExpertDataParams) (out *DataOnChainInfo) {
	rt.ValidateImmediateCallerIs(builtin.ExpertFundActorAddr)

	var st State
	rt.StateTransaction(&st, func() {

		builtin.RequireState(rt, st.Status.Qualified(), "expert is unqualified")

		store := adt.AsStore(rt)
		data, found, err := st.GetData(store, params.PieceID.String())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load expert data: %s", params.PieceID)
		builtin.RequireParam(rt, found, "data not imported: %s", params.PieceID)

		data.Redundancy++
		err = st.PutData(store, data)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to store data")

		out = data
	})
	return
}

type NominateExpertParams struct {
	Expert addr.Address
}

func (a Actor) Nominate(rt Runtime, params *NominateExpertParams) *abi.EmptyValue {

	var st State
	rt.StateReadonly(&st)

	builtin.RequireState(rt, st.Status.Qualified(), "nominator is unqualified")

	info, changed := checkAndGetExpertInfo(rt, &st)
	rt.ValidateImmediateCallerIs(info.Owner)
	if changed {
		rt.StateTransaction(&st, func() {
			info.ApplyNewOwnerEpoch = -1
			err := st.SaveInfo(adt.AsStore(rt), info)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "faile to save expert info")
		})
	}

	code := rt.Send(params.Expert, builtin.MethodsExpert.OnNominated, nil, abi.NewTokenAmount(0), &builtin.Discard{})
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
		builtin.RequireParam(rt, st.Status == ExpertStateRegistered || st.Status == ExpertStateDisqualified, "nominate expert with invalid status")

		info.Proposer = rt.Caller()
		err = st.SaveInfo(store, info)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save info")

		// reset LostEpoch of both new registered & unqualified experts to NoLostEpoch
		st.LostEpoch = NoLostEpoch
		st.Status = ExpertStateNominated
	})

	code := rt.Send(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.TrackNewNominated, nil, abi.NewTokenAmount(0), &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to track new nominated")
	return nil
}

func (a Actor) GovBlock(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)
	builtin.ValidateCallerGranted(rt, rt.Caller(), builtin.MethodsExpert.GovBlock)

	var st State
	var proposer address.Address
	rt.StateTransaction(&st, func() {
		info, err := st.GetInfo(adt.AsStore(rt))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not read expert info")

		builtin.RequireParam(rt, info.Type != builtin.ExpertFoundation, "foundation expert cannot be blocked")
		builtin.RequireParam(rt, st.Status != ExpertStateBlocked, "expert already blocked")
		builtin.RequireParam(rt, st.Status != ExpertStateRegistered, "expert not nominated")

		// Blocking disqualified/nominated/normal experts are allowed, for
		// they probably own valid votes.

		st.Status = ExpertStateBlocked
		st.LostEpoch = rt.CurrEpoch() // record epoch being blocked
		proposer = info.Proposer
	})

	code := rt.Send(proposer, builtin.MethodsExpert.OnImplicated, nil, abi.NewTokenAmount(0), &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to call OnImplicated")

	code = rt.Send(builtin.VoteFundActorAddr, builtin.MethodsVote.OnCandidateBlocked, nil, abi.NewTokenAmount(0), &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to call OnCandidateBlocked")

	return nil
}

// Implicated for being the proposer of the blocked expert
func (a Actor) OnImplicated(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.ExpertActorCodeID)

	var st State
	rt.StateTransaction(&st, func() {
		info, err := st.GetInfo(adt.AsStore(rt))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not read expert info")

		if info.Type == builtin.ExpertFoundation {
			return
		}
		st.ImplicatedTimes++
	})
	return nil
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

type CheckStateReturn struct {
	AllowVote bool
	Qualified bool
}

func (a Actor) CheckState(rt Runtime, _ *abi.EmptyValue) *CheckStateReturn {
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.StateReadonly(&st)
	return &CheckStateReturn{
		AllowVote: st.Status.AllowVote(),
		Qualified: st.Status.Qualified(),
	}
}

type OnTrackUpdateParams struct {
	Votes abi.TokenAmount
}

type OnTrackUpdateReturn struct {
	ResetMe   bool
	UntrackMe bool // tell expertfund_actor to untrack this expert
}

// Called by expertfund.OnEpochTickEnd
func (a Actor) OnTrackUpdate(rt Runtime, params *OnTrackUpdateParams) *OnTrackUpdateReturn {
	rt.ValidateImmediateCallerIs(builtin.ExpertFundActorAddr)

	var ret OnTrackUpdateReturn

	var st State
	store := adt.AsStore(rt)
	rt.StateTransaction(&st, func() {
		info, err := st.GetInfo(store)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get expert info")

		builtin.RequireState(rt, st.Status != ExpertStateRegistered &&
			st.Status != ExpertStateDisqualified &&
			info.Type != builtin.ExpertFoundation, "unexpected expert status or type: %d, %d", st.Status, info.Type)

		if st.Status == ExpertStateBlocked {
			ret.ResetMe = true
			ret.UntrackMe = true
			return
		}

		beforeBelow := st.LostEpoch != NoLostEpoch
		nowBelow := params.Votes.LessThan(st.VoteThreshold())

		if !beforeBelow {
			if nowBelow {
				st.LostEpoch = rt.CurrEpoch()
			} else {
				st.Status = ExpertStateNormal
			}
		} else {
			if nowBelow {
				if rt.CurrEpoch() >= st.LostEpoch+ExpertVoteCheckPeriod {
					ret.ResetMe = true
					ret.UntrackMe = true
					st.Status = ExpertStateDisqualified
					// DO NOT change st.LostEpoch
				}
			} else {
				st.LostEpoch = NoLostEpoch
				st.Status = ExpertStateNormal
			}
		}
	})

	return &ret
}
