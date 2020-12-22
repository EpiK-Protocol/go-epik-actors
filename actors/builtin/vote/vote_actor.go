package vote

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
	. "github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/ipfs/go-cid"
)

type Runtime = runtime.Runtime

type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.BlockCandidates,
		3:                         a.Vote,
		4:                         a.Revoke,
		5:                         a.Claim,
		6:                         a.ApplyRewards,
		7:                         a.OnEpochTickEnd,
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

func (a Actor) Constructor(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")

	st := ConstructState(emptyMap)
	rt.StateCreate(st)
	return nil
}

func (a Actor) BlockCandidates(rt Runtime, params *builtin.BlockCandidatesParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.ExpertFundActorAddr)

	candAddrs := make(map[addr.Address]struct{})
	for _, cand := range params.Candidates {
		resolved, ok := rt.ResolveAddress(cand)
		builtin.RequireParam(rt, ok, "unable to resolve address %v", cand)

		candAddrs[resolved] = struct{}{}
	}

	var st State
	rt.StateTransaction(&st, func() {
		candidates, err := adt.AsMap(adt.AsStore(rt), st.Candidates)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

		err = st.blockCandidates(candidates, candAddrs, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to block candidates")

		st.Candidates, err = candidates.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush candidates")
	})

	return nil
}

type VoteParams struct {
	Candidate addr.Address
}

func (a Actor) Vote(rt Runtime, params *VoteParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	votes := rt.ValueReceived()
	builtin.RequireParam(rt, votes.GreaterThan(big.Zero()), "non positive votes to vote")

	resovled, ok := rt.ResolveAddress(params.Candidate)
	builtin.RequireParam(rt, ok, "unable to resolve address %v", params.Candidate)

	candAddr := builtin.RequestExpertControlAddr(rt, resovled)

	var st State
	var afterVote *Candidate
	store := adt.AsStore(rt)
	rt.StateTransaction(&st, func() {
		candidates, err := adt.AsMap(store, st.Candidates)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

		voters, err := adt.AsMap(store, st.Voters)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load voters")

		voter, found, err := getVoter(voters, rt.Caller())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get voter")
		if !found {
			voter, err = newEmptyVoter(store)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to new voter")
		} else {
			// settle
			err = st.settle(store, voter, candidates, rt.CurrEpoch())
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to settle")
		}

		afterVote, err = st.addToCandidate(candidates, candAddr, votes)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add votes to candidate")
		builtin.RequireParam(rt, !afterVote.IsBlocked(), "cannot vote for blocked candidate")

		err = st.addVotingRecord(store, voter, candAddr, votes)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add voting record")

		st.Candidates, err = candidates.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush candidates")

		err = setVoter(voters, rt.Caller(), voter)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to set voter")
		st.Voters, err = voters.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush voters")

		st.TotalVotes = big.Add(st.TotalVotes, votes)
	})

	builtin.NotifyExpertVote(rt, candAddr, afterVote.Votes)
	return nil
}

//
type RevokeParams struct {
	Candidate addr.Address
	Votes     abi.TokenAmount
}

func (a Actor) Revoke(rt Runtime, params *RevokeParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	builtin.RequireParam(rt, params.Votes.GreaterThan(big.Zero()), "non positive votes to revoke")

	candAddr, ok := rt.ResolveAddress(params.Candidate)
	builtin.RequireParam(rt, ok, "unable to resolve address %v", params.Candidate)

	var st State
	var afterRevoke *Candidate
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)

		candidates, err := adt.AsMap(store, st.Candidates)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

		voters, err := adt.AsMap(store, st.Voters)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load voters")

		voter, found, err := getVoter(voters, rt.Caller())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get voter")
		builtin.RequireParam(rt, found, "voter not found")

		// settle
		err = st.settle(store, voter, candidates, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to settle")

		revokedVotes := big.Zero()
		revokedVotes, err = st.subFromVotingRecord(store, voter, candAddr, params.Votes, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to subtract votes from voting record")
		if revokedVotes.IsZero() {
			return
		}

		afterRevoke, err = st.subFromCandidate(candidates, candAddr, revokedVotes)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to subtract votes from candidate")

		st.Candidates, err = candidates.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush candidates")

		err = setVoter(voters, rt.Caller(), voter)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to set voter")
		st.Voters, err = voters.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush voters")

		// If blocked, TotalVotes has been subtracted in BlockCandidate.
		if !afterRevoke.IsBlocked() {
			st.TotalVotes = big.Sub(st.TotalVotes, revokedVotes)
			Assert(st.TotalVotes.GreaterThanEqual(big.Zero()))
		}
	})
	if afterRevoke != nil && !afterRevoke.IsBlocked() {
		builtin.NotifyExpertVote(rt, candAddr, afterRevoke.Votes)
	}
	return nil
}

// Claims unlocked revoking votes and vested rewards.
func (a Actor) Claim(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	total := abi.NewTokenAmount(0)
	var st State
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)

		candidates, err := adt.AsMap(store, st.Candidates)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

		voters, err := adt.AsMap(store, st.Voters)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load voters")

		voter, found, err := getVoter(voters, rt.Caller())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get voter")
		builtin.RequireParam(rt, found, "voter not found")

		// settle
		err = st.settle(store, voter, candidates, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to settle")
		total = big.Add(total, voter.UnclaimedFunds)

		unlockedVotes, err := st.claimUnlockedVotes(store, voter, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to claim unlocked votes")
		total = big.Add(total, unlockedVotes)

		voter.UnclaimedFunds = abi.NewTokenAmount(0)
		err = setVoter(voters, rt.Caller(), voter)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update voter")

		st.Voters, err = voters.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush voters")
	})
	Assert(total.LessThanEqual(rt.CurrentBalance()))
	if total.GreaterThan(big.Zero()) {
		code := rt.Send(rt.Caller(), builtin.MethodSend, nil, total, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds")
	}

	return nil
}

func (a Actor) ApplyRewards(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.RewardActorAddr)
	builtin.RequireParam(rt, rt.ValueReceived().GreaterThanEqual(big.Zero()), "cannot add a negative amount of funds")
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

	var st State
	rt.StateTransaction(&st, func() {
		if st.UnownedFunds.IsZero() || st.TotalVotes.IsZero() {
			return
		}

		st.CumEarningsPerVote = big.Add(st.CumEarningsPerVote, big.Div(st.UnownedFunds, st.TotalVotes))
		st.UnownedFunds = big.Mod(st.UnownedFunds, st.TotalVotes)
	})
	return nil
}
