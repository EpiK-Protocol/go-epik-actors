package vote

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type StateSummary struct {
	BlockedAt       map[address.Address]abi.ChainEpoch  // key is candidate
	BlockEarnings   map[address.Address]abi.TokenAmount // key is candidate
	VoterTallyCount map[address.Address]int             // key is voter

	TotalBlockedVotes    abi.TokenAmount
	TotalNonBlockedVotes abi.TokenAmount
	TotalRescindingVotes abi.TokenAmount
	CandidatesCount      int
	VotersCount          int
}

func CheckStateInvariants(st *State, store adt.Store) (*StateSummary, *builtin.MessageAccumulator) {
	acc := &builtin.MessageAccumulator{}
	sum := &StateSummary{
		BlockedAt:       make(map[address.Address]abi.ChainEpoch),
		BlockEarnings:   make(map[address.Address]abi.TokenAmount),
		VoterTallyCount: make(map[address.Address]int),

		TotalBlockedVotes:    big.Zero(),
		TotalNonBlockedVotes: big.Zero(),
		TotalRescindingVotes: big.Zero(),
	}

	// Candidates
	blockedVotesByCands := make(map[address.Address]abi.TokenAmount)
	nonblockedVotesByCands := make(map[address.Address]abi.TokenAmount)
	candidates, err := adt.AsMap(store, st.Candidates, builtin.DefaultHamtBitwidth)
	if err != nil {
		acc.Addf("error loading candidates: %v", err)
	} else {
		var out Candidate
		err = candidates.ForEach(&out, func(k string) error {
			sum.CandidatesCount++

			ida, err := address.NewFromBytes([]byte(k))
			acc.RequireNoError(err, "error deserializing candidate address: %s", k)
			acc.Require(ida.Protocol() == address.ID, "candidate address not an ID address: %s", k)

			if out.IsBlocked() {
				blockedVotesByCands[ida] = out.Votes
				sum.TotalBlockedVotes = big.Add(sum.TotalBlockedVotes, out.Votes)
				sum.BlockedAt[ida] = out.BlockEpoch
			} else {
				nonblockedVotesByCands[ida] = out.Votes
				sum.TotalNonBlockedVotes = big.Add(sum.TotalNonBlockedVotes, out.Votes)
			}
			return nil
		})
		acc.RequireNoError(err, "error iterating candidates")
	}

	acc.Require(sum.TotalNonBlockedVotes.Equals(st.CurrEpochEffectiveVotes), "st.TotalVotes != sum of non blocked candidates")

	//   Voters
	blockedVotesByVoters := make(map[address.Address]abi.TokenAmount)    // key is candidate address
	nonblockedVotesByVoters := make(map[address.Address]abi.TokenAmount) // key is candidate address
	voters, err := adt.AsMap(store, st.Voters, builtin.DefaultHamtBitwidth)
	if err != nil {
		acc.Addf("error loading voters: %v", err)
	} else {
		var vout Voter
		err = voters.ForEach(&vout, func(k string) error {
			sum.VotersCount++

			idaVoter, err := address.NewFromBytes([]byte(k))
			acc.RequireNoError(err, "error deserializing voter address: %s", k)

			tally, err := adt.AsMap(store, vout.Tally, builtin.DefaultHamtBitwidth)
			if err != nil {
				acc.Addf("error loading tally of voter %s: %v", idaVoter, err)
			} else {
				var out VotesInfo
				err := tally.ForEach(&out, func(k2 string) error {
					sum.VoterTallyCount[idaVoter]++

					idaCand, err := address.NewFromBytes([]byte(k2))
					acc.RequireNoError(err, "error deserializing candidate %s voted by %s", k2, k)

					sum.TotalRescindingVotes = big.Add(sum.TotalRescindingVotes, out.RescindingVotes)

					if _, ok := sum.BlockedAt[idaCand]; ok {
						if old, ok2 := blockedVotesByVoters[idaCand]; ok2 {
							blockedVotesByVoters[idaCand] = big.Add(old, out.Votes)
						} else {
							blockedVotesByVoters[idaCand] = out.Votes
						}
					} else {
						if old, ok2 := nonblockedVotesByVoters[idaCand]; ok2 {
							nonblockedVotesByVoters[idaCand] = big.Add(old, out.Votes)
						} else {
							nonblockedVotesByVoters[idaCand] = out.Votes
						}
					}
					return nil
				})
				acc.RequireNoError(err, "error iterating voter tally: %s", idaVoter)
			}
			return nil
		})
		acc.RequireNoError(err, "error iterating voters")
	}

	acc.Require(len(blockedVotesByCands) == len(blockedVotesByVoters), "length of blocked votes mismatched: %d, %d", len(blockedVotesByCands), len(blockedVotesByVoters))
	acc.Require(len(nonblockedVotesByCands) == len(nonblockedVotesByVoters), "lenght of non-blocked votes mismatched: %d, %d", len(nonblockedVotesByCands), len(nonblockedVotesByVoters))

	for ida, amtC := range blockedVotesByCands {
		amtV, ok := blockedVotesByVoters[ida]
		acc.Require(ok, "candidate %s not found in blockedVotesByVoters", ida)
		acc.Require(amtV.Equals(amtC), "blocked amount of votes not equal: %s, by voters %s, by cands %s", ida, amtV, amtC)
	}

	for ida, amtC := range nonblockedVotesByCands {
		amtV, ok := nonblockedVotesByVoters[ida]
		acc.Require(ok, "candidate %s not found in nonblockedVotesByVoters", ida)
		acc.Require(amtV.Equals(amtC), "non-blocked amount of votes not equal: %s, by voters %s, by cands %s", ida, amtV, amtC)
	}

	return sum, acc
}
