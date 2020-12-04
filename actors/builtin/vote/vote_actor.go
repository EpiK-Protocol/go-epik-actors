package vote

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
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
		4:                         a.Rescind,
		5:                         a.Withdraw,
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

func (a Actor) Constructor(rt Runtime, fallback *addr.Address) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	st, err := ConstructState(adt.AsStore(rt), *fallback)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")
	rt.StateCreate(st)
	return nil
}

func (a Actor) BlockCandidates(rt Runtime, params *builtin.BlockCandidatesParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.ExpertFundActorAddr)

	if len(params.Candidates) == 0 {
		return nil
	}

	candAddrs := make(map[addr.Address]struct{})
	for _, cand := range params.Candidates {
		resolved, ok := rt.ResolveAddress(cand)
		builtin.RequireParam(rt, ok, "unable to resolve address %v", cand)

		candAddrs[resolved] = struct{}{}
	}

	var st State
	rt.StateTransaction(&st, func() {
		candidates, err := adt.AsMap(adt.AsStore(rt), st.Candidates, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

		n, err := st.BlockCandidates(candidates, candAddrs, rt.CurrEpoch())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to block candidates")

		if n == 0 {
			return
		}

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
	builtin.RequireParam(rt, ok, "unable to resolve address %v", candidate)

	builtin.RequestExpertControlAddr(rt, candAddr)

	var st State
	var afterVote *Candidate
	store := adt.AsStore(rt)
	rt.StateTransaction(&st, func() {
		candidates, err := adt.AsMap(store, st.Candidates, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load candidates")

		voters, err := adt.AsMap(store, st.Voters, builtin.DefaultHamtBitwidth)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load voters")

		voter, found, err := getVoter(voters, rt.Caller())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get voter")
		if !found {
			tally, err := adt.MakeEmptyMap(store, builtin.DefaultHamtBitwidth).Root()
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

		afterVote, err = st.addToCandidate(candidates, candAddr, votes)
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

	NotifyExpertVote(rt, candAddr, afterVote.Votes)
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

	// send notification even if blocked
	NotifyExpertVote(rt, candAddr, afterRescind.Votes)

	return nil
}

func NotifyExpertVote(rt runtime.Runtime, expertAddr addr.Address, voteAmount abi.TokenAmount) {
	params := &expert.ExpertVoteParams{
		Amount: voteAmount,
	}
	code := rt.Send(expertAddr, builtin.MethodsExpert.Vote, params, abi.NewTokenAmount(0), &builtin.Discard{})
	builtin.RequireSuccess(rt, code, "failed to notify expert vote")
}

// Withdraws unlocked rescinding votes and rewards, returns actual sent amount
func (a Actor) Withdraw(rt Runtime, to *addr.Address) *abi.TokenAmount {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	recipient, ok := rt.ResolveAddress(*to)
	builtin.RequireParam(rt, ok, "failed to resolve address %v", to)

	codeID, ok := rt.GetActorCodeCID(recipient)
	builtin.RequireParam(rt, ok, "no code for address %v", recipient)

	if codeID.Equals(builtin.StorageMinerActorCodeID) {
		recipient, _, _ = builtin.RequestMinerControlAddrs(rt, recipient)
	}

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
		code := rt.Send(recipient, builtin.MethodSend, nil, total, &builtin.Discard{})
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

		st.CumEarningsPerVote = big.Add(st.CumEarningsPerVote, big.Div(st.UnownedFunds, st.TotalVotes))
		st.UnownedFunds = big.Mod(st.UnownedFunds, st.TotalVotes)
	})

	if !toFallback.IsZero() {
		code := rt.Send(st.FallbackReceiver, builtin.MethodSend, nil, toFallback, &builtin.Discard{})
		builtin.RequireSuccess(rt, code, "failed to send funds to fallback")
	}

	return nil
}
