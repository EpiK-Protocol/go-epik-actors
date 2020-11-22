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
	}
}

func (a Actor) Code() cid.Cid {
	return builtin.VoteActorCodeID
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

	emptyCandMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")
	emptyVoterMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")

	st := ConstructState(emptyCandMap, emptyVoterMap)
	rt.StateCreate(st)
	return nil
}

func (a Actor) BlockCandidates(rt Runtime, params *builtin.BlockCandidatesParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.ExpertFundsActorAddr)

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
	// TODO: only signable allow to vote?
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	votes := rt.ValueReceived()
	builtin.RequireParam(rt, votes.GreaterThan(big.Zero()), "non positive votes to vote")

	resovled, ok := rt.ResolveAddress(params.Candidate)
	builtin.RequireParam(rt, ok, "unable to resolve address %v", params.Candidate)

	candAddr := builtin.RequestExpertControlAddr(rt, resovled)

	var st State
	notice := false
	var afterVote *Candidate
	store := adt.AsStore(rt)
	rt.StateTransaction(&st, func() {
		candidates, err := adt.AsMap(store, st.Candidates)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

		voters, err := adt.AsMap(store, st.Voters)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load voters")

		afterVote, err = st.addToCandidate(candidates, candAddr, votes)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add votes to candidate")

		err = st.addVotingRecord(store, voters, rt.Caller(), candAddr, votes)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add voting record")

		st.Candidates, err = candidates.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush candidates")

		st.Voters, err = voters.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush voters")

		st.TotalValidVotes = big.Add(st.TotalValidVotes, votes)
		notice = true
	})
	if notice {
		builtin.NotifyExpertVote(rt, candAddr, afterVote.Votes)
	}
	return nil
}

//
type RevokeParams struct {
	Candidate addr.Address
	Votes     abi.TokenAmount
}

func (a Actor) Revoke(rt Runtime, params *RevokeParams) *abi.EmptyValue {
	// TODO: only signable allow to vote?
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	votes := params.Votes
	builtin.RequireParam(rt, votes.GreaterThan(big.Zero()), "non positive votes to revoke")

	candAddr := resolveCandidateAddress(rt, params.Candidate)

	var st State
	notice := false
	var afterRevoke *Candidate
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)

		candidates, err := adt.AsMap(store, st.Candidates)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

		voters, err := adt.AsMap(store, st.Voters)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load voters")

		afterRevoke, err = st.revokeFromCandidate(candidates, candAddr, votes)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to revoke from candidate")

		err = st.updateVotingRecordForRevoking(store, voters, rt.Caller(), candAddr, votes, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to vote")

		st.Candidates, err = candidates.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush candidates")

		st.Voters, err = voters.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush voters")

		// If blocked, TotalValidVotes has been subtracted in BlockCandidate.
		if !afterRevoke.IsBlocked() {
			st.TotalValidVotes = big.Sub(st.TotalValidVotes, votes)
			Assert(st.TotalValidVotes.GreaterThanEqual(big.Zero()))
			notice = true
		}
	})
	if notice {
		builtin.NotifyExpertVote(rt, candAddr, afterRevoke.Votes)
	}
	return nil
}

// withdraws unlocked revoking votes and vested rewards.
func (a Actor) Claim(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	// TODO: only signable allow to vote?
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	totalWithdrawn := abi.NewTokenAmount(0)
	var st State
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)
		voterAddr := rt.Caller()

		voters, err := adt.AsMap(store, st.Voters)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load voters")

		voter, found, err := getVoter(voters, voterAddr)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get voter")
		builtin.RequireParam(rt, found, "voter %s not found", voterAddr)

		votingRecords, err := adt.AsMap(store, voter.VotingRecords)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load voting records")

		deletes, updates, unlocked, allDelete, err := findUnlockedVotes(rt, votingRecords)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to find unlocked votes of %s", voterAddr)
		if allDelete {
			err = deleteVoter(voters, voterAddr)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to delete voter")
		} else {
			err = updateVotingRecords(votingRecords, deletes, updates)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update records")

			newVoter := &Voter{}
			newVoter.VotingRecords, err = votingRecords.Root()
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush records")
			err = setVoter(voters, voterAddr, newVoter)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update voter")
		}

		st.Voters, err = voters.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush voters")

		//TODO: rewards pool
		vestedRewards := big.Zero()
		AssertMsg(st.TotalRewards.GreaterThanEqual(vestedRewards), "total rewards: %v, vested reward: %v", st.TotalRewards, vestedRewards)
		st.TotalRewards = big.Sub(st.TotalRewards, vestedRewards)

		totalWithdrawn = big.Add(unlocked, vestedRewards)
	})
	builtin.RequireParam(rt, !totalWithdrawn.IsZero(), "no balance to withdraw")

	Assert(totalWithdrawn.LessThanEqual(rt.CurrentBalance()))
	code := rt.Send(rt.Caller(), builtin.MethodSend, nil, totalWithdrawn, &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to send funds")

	return nil
}

// called by reward actor
func (a Actor) ApplyRewards(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.RewardActorAddr)

	builtin.RequireParam(rt, rt.ValueReceived().GreaterThanEqual(big.Zero()), "cannot add a negative amount of funds")

	var st State
	rt.StateTransaction(&st, func() {
		st.TotalRewards = big.Add(st.TotalRewards, rt.ValueReceived())
	})
	return nil
}

func findUnlockedVotes(rt Runtime, votingRecords *adt.Map) (
	deletes []addr.Address,
	updates map[addr.Address]*VotingRecord,
	totalUnlocked abi.TokenAmount,
	allDelete bool,
	err error,
) {
	deletes = make([]addr.Address, 0)
	updates = make(map[addr.Address]*VotingRecord)
	totalUnlocked = big.Zero()

	var (
		total int
		old   VotingRecord
	)
	err = votingRecords.ForEach(&old, func(key string) error {
		total++
		if old.RevokingVotes.IsZero() || rt.CurrEpoch() <= old.LastRevokingEpoch+RevokingUnlockDelay {
			return nil
		}
		totalUnlocked = big.Add(totalUnlocked, old.RevokingVotes)

		candAddr, err := addr.NewFromBytes([]byte(key))
		if err != nil {
			return err
		}
		// delete
		if old.Votes.IsZero() {
			deletes = append(deletes, candAddr)
			return nil
		}
		// update
		updates[candAddr] = &VotingRecord{
			Votes:             old.Votes,
			RevokingVotes:     big.Zero(),
			LastRevokingEpoch: old.LastRevokingEpoch,
		}
		return nil
	})
	if err != nil {
		return nil, nil, abi.NewTokenAmount(0), false, err
	}
	allDelete = total == len(deletes)
	return
}

func resolveCandidateAddress(rt Runtime, raw addr.Address) addr.Address {
	resolved, ok := rt.ResolveAddress(raw)
	builtin.RequireParam(rt, ok, "unable to resolve address %v", raw)

	codeCID, ok := rt.GetActorCodeCID(resolved)
	builtin.RequireParam(rt, ok, "no code for address %v", resolved)

	builtin.RequireParam(rt, codeCID == builtin.ExpertActorCodeID, "actor type must be an expert, was: %v", codeCID)

	return resolved
}
