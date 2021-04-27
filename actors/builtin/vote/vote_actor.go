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
		6:                         a.ApplyRewards,
		7:                         a.OnEpochTickEnd,
		8:                         a.GetCandidates,
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

	candAddrs := map[addr.Address]struct{}{
		*candAddr: {},
	}

	var st State
	rt.StateTransaction(&st, func() {
		candidates, err := adt.AsMap(adt.AsStore(rt), st.Candidates, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

		_, err = st.BlockCandidates(candidates, candAddrs, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to block candidates")

		st.Candidates, err = candidates.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush candidates")
	})

	return nil
}

func (a Actor) Vote(rt Runtime, candidate *addr.Address) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	votes := rt.ValueReceived()
	builtin.RequireParam(rt, votes.GreaterThan(big.Zero()), "non positive votes to vote")

	candAddr, ok := rt.ResolveAddress(*candidate)
	builtin.RequireParam(rt, ok, "unable to resolve address %s", candidate)
	actorCode, ok := rt.GetActorCodeCID(candAddr)
	builtin.RequireParam(rt, ok, "no code for address %s", candidate)
	builtin.RequireParam(rt, actorCode == builtin.ExpertActorCodeID, "not an expert %s", candidate)

	allowed := builtin.CheckVoteAllowed(rt, candAddr)
	builtin.RequireParam(rt, allowed, "vote not allowed %s", candidate)

	var afterVoted *Candidate

	var st State
	store := adt.AsStore(rt)
	rt.StateTransaction(&st, func() {
		candidates, err := adt.AsMap(store, st.Candidates, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

		voters, err := adt.AsMap(store, st.Voters, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load voters")

		voter, found, err := getVoter(voters, rt.Caller())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get voter")
		if !found {
			tally, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to make tally for voter")
			voter = &Voter{
				SettleEpoch:              rt.CurrEpoch(),
				SettleCumEarningsPerVote: st.CumEarningsPerVote,
				Withdrawable:             abi.NewTokenAmount(0),
				Tally:                    tally,
			}
		} else {
			// settle
			err = st.settle(store, voter, candidates, rt.CurrEpoch())
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to settle")
		}

		afterVoted, err = st.addToCandidate(candidates, candAddr, votes)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add votes to candidate")

		err = st.addToTally(store, voter, candAddr, votes)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add voting record")

		st.Candidates, err = candidates.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush candidates")

		err = setVoter(voters, rt.Caller(), voter)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to set voter")
		st.Voters, err = voters.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush voters")

		st.TotalVotes = big.Add(st.TotalVotes, votes)
	})

	notifyCandidateVotesUpdated(rt, candAddr, afterVoted.Votes)

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

	var st State
	var afterRescind *Candidate
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)

		candidates, err := adt.AsMap(store, st.Candidates, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

		voters, err := adt.AsMap(store, st.Voters, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load voters")

		voter, found, err := getVoter(voters, rt.Caller())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get voter")
		builtin.RequireParam(rt, found, "voter not found")

		// settle
		err = st.settle(store, voter, candidates, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to settle")

		rescindedVotes, err := st.subFromTally(store, voter, candAddr, params.Votes, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to subtract votes from tally")

		afterRescind, err = st.subFromCandidate(candidates, candAddr, rescindedVotes)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to subtract votes from candidate")

		st.Candidates, err = candidates.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush candidates")

		err = setVoter(voters, rt.Caller(), voter)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to set voter")
		st.Voters, err = voters.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush voters")

		// If blocked, TotalVotes has been subtracted in BlockCandidate.
		if !afterRescind.IsBlocked() {
			st.TotalVotes = big.Sub(st.TotalVotes, rescindedVotes)
			builtin.RequireState(rt, st.TotalVotes.GreaterThanEqual(big.Zero()), "negative total votes %v after sub %v", st.TotalVotes, rescindedVotes)
		}
	})

	if !afterRescind.IsBlocked() {
		notifyCandidateVotesUpdated(rt, candAddr, afterRescind.Votes)
	}

	return nil
}

// Withdraws unlocked rescinding votes and rewards, returns actual sent amount
func (a Actor) Withdraw(rt Runtime, _ *abi.EmptyValue) *abi.TokenAmount {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	total := abi.NewTokenAmount(0)
	var st State
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)

		candidates, err := adt.AsMap(store, st.Candidates, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

		voters, err := adt.AsMap(store, st.Voters, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load voters")

		voter, found, err := getVoter(voters, rt.Caller())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get voter")
		builtin.RequireParam(rt, found, "voter not found")

		// settle
		err = st.settle(store, voter, candidates, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to settle")

		// unlock votes
		unlockedVotes, isVoterEmpty, err := st.withdrawUnlockedVotes(store, voter, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to claim unlocked votes")

		total = big.Add(voter.Withdrawable, unlockedVotes)

		if !isVoterEmpty {
			voter.Withdrawable = abi.NewTokenAmount(0)
			err = setVoter(voters, rt.Caller(), voter)
		} else {
			err = deleteVoter(voters, rt.Caller())
		}
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update voter")

		st.Voters, err = voters.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush voters")
	})
	builtin.RequireState(rt, total.LessThanEqual(rt.CurrentBalance()), "expected withdrawn amount %v exceeds balance %v", total, rt.CurrentBalance())

	if total.GreaterThan(big.Zero()) {
		code := rt.Send(rt.Caller(), builtin.MethodSend, nil, total, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds")
	}

	return &total
}

func (a Actor) ApplyRewards(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.RewardActorAddr)
	builtin.RequireParam(rt, rt.ValueReceived().GreaterThanEqual(big.Zero()), "negative amount to apply")

	if rt.ValueReceived().Sign() == 0 {
		return nil
	}

	var st State
	rt.StateTransaction(&st, func() {
		st.UnownedFunds = big.Add(st.UnownedFunds, rt.ValueReceived())
	})
	return nil
}

func (a Actor) OnEpochTickEnd(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.CronActorAddr)

	toFallback := abi.NewTokenAmount(0)
	var st State
	rt.StateTransaction(&st, func() {
		builtin.RequireParam(rt, st.UnownedFunds.GreaterThanEqual(big.Zero()), "non positive unowned funds")

		if st.UnownedFunds.IsZero() {
			return
		}

		if st.TotalVotes.IsZero() {
			toFallback = st.UnownedFunds
			st.UnownedFunds = big.Zero()
			return
		}
		deltaPerVote := big.Div(big.Mul(st.UnownedFunds, Multiplier1E12), st.TotalVotes)
		st.CumEarningsPerVote = big.Add(st.CumEarningsPerVote, deltaPerVote)
		st.UnownedFunds = big.Sub(st.UnownedFunds, big.Div(big.Mul(deltaPerVote, st.TotalVotes), Multiplier1E12))
	})

	if !toFallback.IsZero() {
		code := rt.Send(st.FallbackReceiver, builtin.MethodSend, nil, toFallback, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds to fallback")
	}

	return nil
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
