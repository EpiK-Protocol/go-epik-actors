package vote

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/ipfs/go-cid"
)

type Runtime = runtime.Runtime

type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.OnCandidateBlocked,
		3:                         a.Vote,
		4:                         a.Rescind,
		5:                         a.Withdraw,
		6:                         a.GetCandidates,
	}
}

func (a Actor) Code() cid.Cid {
	return builtin.VoteFundActorCodeID
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

func (a Actor) Constructor(rt Runtime, fallback *addr.Address) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	st, err := ConstructState(adt.AsStore(rt), *fallback)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")
	rt.StateCreate(st)
	return nil
}

func (a Actor) OnCandidateBlocked(rt Runtime, candAddr *addr.Address) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.ExpertFundActorAddr)

	var st State
	rt.StateTransaction(&st, func() {
		rewardBalance := big.Sub(rt.CurrentBalance(), st.TotalVotes)
		_, err := st.BlockCandidates(adt.AsStore(rt), rt.CurrEpoch(), rewardBalance, *candAddr)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to block candidates")
	})

	return nil
}

func (a Actor) Vote(rt Runtime, unresolved *addr.Address) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	votes := rt.ValueReceived()
	builtin.RequireParam(rt, votes.GreaterThan(big.Zero()), "non positive votes to vote")

	candAddr, ok := rt.ResolveAddress(*unresolved)
	builtin.RequireParam(rt, ok, "unable to resolve address %s", unresolved)
	actorCode, ok := rt.GetActorCodeCID(candAddr)
	builtin.RequireParam(rt, ok, "no code for address %s", unresolved)
	builtin.RequireParam(rt, actorCode == builtin.ExpertActorCodeID, "not an expert %s", unresolved)

	allowed := builtin.CheckVoteAllowed(rt, candAddr)
	builtin.RequireParam(rt, allowed, "vote not allowed %s", unresolved)

	store := adt.AsStore(rt)
	currEpoch := rt.CurrEpoch()

	var candidate *Candidate
	var st State
	rt.StateTransaction(&st, func() {
		st.TotalVotes = big.Add(st.TotalVotes, votes)
		rewardBalance := big.Sub(rt.CurrentBalance(), st.TotalVotes)

		voter, found, err := st.GetVoter(store, rt.Caller())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get voter")
		if !found {
			voter, err = newVoter(store, currEpoch, st.PrevEpochEarningsPerVote)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to new voter")
		}

		candidate, found, err = st.GetCandidate(store, candAddr)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get candidate")
		if !found {
			candidate = newCandidate()
		}
		builtin.RequireState(rt, !candidate.IsBlocked(), "candidate already blocked %s", candAddr)

		// update votes
		candidate.Votes = big.Add(votes, candidate.Votes)
		err = st.AddVotes(store, voter, candAddr, votes, currEpoch, rewardBalance)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add voting record")

		err = st.PutVoter(store, rt.Caller(), voter)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put voter")

		err = st.PutCandidate(store, candAddr, candidate)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put candidate")

		st.CurrEpochEffectiveVotes = big.Add(st.CurrEpochEffectiveVotes, votes)
	})

	notifyCandidateVotesUpdated(rt, candAddr, candidate.Votes)

	return nil
}

//
type RescindParams struct {
	Candidate addr.Address
	Votes     abi.TokenAmount
}

func (a Actor) Rescind(rt Runtime, params *RescindParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	builtin.RequireParam(rt, params.Votes.GreaterThan(big.Zero()), "non positive votes to rescind")

	candAddr, ok := rt.ResolveAddress(params.Candidate)
	builtin.RequireParam(rt, ok, "unable to resolve address %v", params.Candidate)

	store := adt.AsStore(rt)
	currEpoch := rt.CurrEpoch()
	var candidate *Candidate
	var st State
	rt.StateTransaction(&st, func() {
		rewardBalance := big.Sub(rt.CurrentBalance(), st.TotalVotes)

		voter, found, err := st.GetVoter(store, rt.Caller())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get voter")
		builtin.RequireParam(rt, found, "voter not found")

		candidate, found, err = st.GetCandidate(store, candAddr)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get candidate")
		builtin.RequireParam(rt, found, "candidate not found %s", candAddr)

		// update votes
		rescindedVotes, err := st.RescindVotes(store, voter, candAddr, params.Votes, currEpoch, rewardBalance)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to subtract votes from tally")

		candidate.Votes = big.Sub(candidate.Votes, rescindedVotes)
		builtin.RequireState(rt, candidate.Votes.GreaterThanEqual(big.Zero()), "unexpect negative votes after rescind")

		// save
		err = st.PutVoter(store, rt.Caller(), voter)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put voter")

		err = st.PutCandidate(store, candAddr, candidate)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put candidate")

		// If blocked, CurrEpochEffectiveVotes has been subtracted in BlockCandidate.
		if !candidate.IsBlocked() {
			st.CurrEpochEffectiveVotes = big.Sub(st.CurrEpochEffectiveVotes, rescindedVotes)
			builtin.RequireState(rt, st.CurrEpochEffectiveVotes.GreaterThanEqual(big.Zero()), "negative total votes %v after sub %v", st.CurrEpochEffectiveVotes, rescindedVotes)
		}
	})

	if !candidate.IsBlocked() {
		notifyCandidateVotesUpdated(rt, candAddr, candidate.Votes)
	}

	return nil
}

// Withdraws unlocked rescinding votes and rewards, returns actual sent amount
func (a Actor) Withdraw(rt Runtime, _ *abi.EmptyValue) *abi.TokenAmount {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	fallbackDebt := abi.NewTokenAmount(0)
	voterVotes := abi.NewTokenAmount(0)
	voterRewards := abi.NewTokenAmount(0)

	store := adt.AsStore(rt)
	currEpoch := rt.CurrEpoch()
	caller := rt.Caller()
	var st State
	rt.StateTransaction(&st, func() {
		rewardBalance := big.Sub(rt.CurrentBalance(), st.TotalVotes)

		voter, found, err := st.GetVoter(store, caller)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get voter")

		if found {
			var isVoterEmpty bool
			voterVotes, isVoterEmpty, err = st.WithdrawUnlockedVotes(store, voter, currEpoch, rewardBalance)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to withdraws")

			voterRewards = voter.Withdrawable
			if !isVoterEmpty {
				voter.Withdrawable = abi.NewTokenAmount(0)
				err = st.PutVoter(store, caller, voter)
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put voter")
			} else {
				err = st.DeleteVoter(store, caller)
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to delete voter")
			}

			st.TotalVotes = big.Sub(st.TotalVotes, voterVotes)
		} else {
			err := st.UpdatePool(store, currEpoch, rewardBalance)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update pool")
		}

		// fallback
		fallbackDebt = st.FallbackDebt
		st.FallbackDebt = abi.NewTokenAmount(0)

		st.LastRewardBalance = big.Sub(st.LastRewardBalance, voterRewards)
		st.LastRewardBalance = big.Sub(st.LastRewardBalance, fallbackDebt)
		builtin.RequireParam(rt, st.LastRewardBalance.GreaterThanEqual(big.Zero()), "unexpected negative LastRewardBalance after sub fallback %s", fallbackDebt)
	})

	voterFunds := big.Add(voterRewards, voterVotes)
	if voterFunds.GreaterThan(big.Zero()) {
		code := rt.Send(caller, builtin.MethodSend, nil, voterFunds, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds to voter")
	}
	if fallbackDebt.GreaterThan(big.Zero()) {
		code := rt.Send(st.FallbackReceiver, builtin.MethodSend, nil, fallbackDebt, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds to fallback")
	}

	return &voterFunds
}

type GetCandidatesParams struct {
	Addresses []addr.Address
}

type GetCandidatesReturn struct {
	Votes []abi.TokenAmount
}

func (a Actor) GetCandidates(rt Runtime, params *GetCandidatesParams) *GetCandidatesReturn {
	rt.ValidateImmediateCallerAcceptAny()

	var ret GetCandidatesReturn

	var st State
	rt.StateReadonly(&st)

	candidates, err := adt.AsMap(adt.AsStore(rt), st.Candidates, builtin.DefaultHamtBitwidth)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

	for _, candAddr := range params.Addresses {
		builtin.RequireParam(rt, candAddr.Protocol() == addr.ID, "ID address required")

		candidate, found, err := getCandidate(candidates, candAddr)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get candidate")
		builtin.RequireParam(rt, found, "candidate not found %s", candAddr)

		ret.Votes = append(ret.Votes, candidate.Votes)
	}

	return &ret
}

func notifyCandidateVotesUpdated(rt Runtime, cand addr.Address, votes abi.TokenAmount) {
	code := rt.Send(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.OnExpertVotesUpdated, &builtin.OnExpertVotesUpdatedParams{
		Expert: cand,
		Votes:  votes,
	}, abi.NewTokenAmount(0), &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to notify expert votes updated")
}
