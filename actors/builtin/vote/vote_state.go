package vote

import (
	"sort"

	"github.com/filecoin-project/go-address"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	. "github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"
)

type State struct {

	// Information for each candidate.
	Candidates cid.Cid // Map, HAMT[Candidate ID-Address]Candidate

	// Information for each voter.
	Voters cid.Cid // Map, HAMT [Voter ID-Address]Voter

	// Total valid votes(atto), excluding rescinded and blocked votes(atto).
	TotalVotes abi.TokenAmount

	// Total unowned funds.
	UnownedFunds abi.TokenAmount
	// Cumulative earnings per vote(atto) since genesis. Updated at each epoch tick end
	CumEarningsPerVote abi.TokenAmount

	// Fallback rewards receiver when no votes
	FallbackReceiver address.Address
}

type Candidate struct {
	// Epoch in which this candidate was firstly blocked.
	BlockEpoch abi.ChainEpoch

	// CumEarningsPerVote in epoch just previous to BlockEpoch.
	BlockCumEarningsPerVote abi.TokenAmount

	// Number of votes(atto) currently received.
	Votes abi.TokenAmount
}

func (c *Candidate) IsBlocked() bool {
	return c.BlockEpoch > 0
}

func (c *Candidate) BlockedBefore(e abi.ChainEpoch) bool {
	return c.BlockEpoch > 0 && c.BlockEpoch < e
}

type Voter struct {
	// Epoch in which the last settle occurs.
	SettleEpoch abi.ChainEpoch
	// CumEarningsPerVote in epoch just previous to LastSettleEpoch.
	SettleCumEarningsPerVote abi.TokenAmount

	// Withdrawable rewards since last withdrawal.
	Withdrawable abi.TokenAmount

	// Tally for each candidate.
	Tally cid.Cid // Map, HAMT [Candidate ID-Address]VotesInfo
}

type VotesInfo struct {
	// Number of valid votes(atto) for candidate.
	Votes abi.TokenAmount
	// Number of votes being rescinded.
	RescindingVotes abi.TokenAmount
	// Epoch during which the last rescind called.
	LastRescindEpoch abi.ChainEpoch
}

func ConstructState(emptyMapCid cid.Cid, fallback address.Address) *State {
	return &State{
		Candidates:         emptyMapCid,
		Voters:             emptyMapCid,
		TotalVotes:         abi.NewTokenAmount(0),
		UnownedFunds:       abi.NewTokenAmount(0),
		CumEarningsPerVote: abi.NewTokenAmount(0),
		FallbackReceiver:   fallback,
	}
}

func (st *State) BlockCandidates(candidates *adt.Map, candAddrs map[addr.Address]struct{}, cur abi.ChainEpoch) (int, error) {
	blocked := 0
	for candAddr := range candAddrs {
		cand, found, err := getCandidate(candidates, candAddr)
		if err != nil {
			return 0, err
		}
		if !found {
			return 0, errors.Errorf("candidate not found: %s", candAddr)
		}

		if cand.IsBlocked() {
			continue
		}

		cand.BlockEpoch = cur
		cand.BlockCumEarningsPerVote = st.CumEarningsPerVote
		err = setCandidate(candidates, candAddr, cand)
		if err != nil {
			return 0, err
		}
		st.TotalVotes = big.Sub(st.TotalVotes, cand.Votes)
		Assert(st.TotalVotes.GreaterThanEqual(big.Zero()))

		blocked++
	}
	return blocked, nil
}

// Allow to rescind from blocked candidate.
func (st *State) subFromCandidate(
	candidates *adt.Map,
	candAddr addr.Address,
	votes abi.TokenAmount,
) (*Candidate, error) {
	cand, found, err := getCandidate(candidates, candAddr)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, xerrors.Errorf("candidate %s not exist", candAddr)
	}
	AssertMsg(cand.Votes.GreaterThanEqual(votes), "insufficient votes in candidate")

	cand.Votes = big.Sub(cand.Votes, votes)
	err = setCandidate(candidates, candAddr, cand)
	if err != nil {
		return nil, err
	}

	return cand, nil
}

func (st *State) subFromTally(
	s adt.Store,
	voter *Voter,
	candAddr addr.Address,
	votes abi.TokenAmount,
	cur abi.ChainEpoch,
) (abi.TokenAmount, error) {

	tally, err := adt.AsMap(s, voter.Tally)
	if err != nil {
		return abi.NewTokenAmount(0), errors.Wrapf(err, "failed to load tally")
	}
	info, found, err := getVotesInfo(tally, candAddr)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}
	if !found {
		return abi.NewTokenAmount(0), xerrors.Errorf("tally item for %s not found", candAddr)
	}
	if info.Votes.LessThan(votes) {
		votes = info.Votes
	}

	// update VotesInfo
	info.Votes = big.Sub(info.Votes, votes)
	info.RescindingVotes = big.Add(info.RescindingVotes, votes)
	info.LastRescindEpoch = cur
	err = setVotesInfo(tally, candAddr, info)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}
	voter.Tally, err = tally.Root()
	if err != nil {
		return abi.NewTokenAmount(0), errors.Wrapf(err, "failed to flush tally")
	}
	return votes, nil
}

// Assuming this candidate is eligible.
func (st *State) addToCandidate(
	candidates *adt.Map,
	candAddr addr.Address,
	votes abi.TokenAmount,
) (*Candidate, error) {
	cand, found, err := getCandidate(candidates, candAddr)
	if err != nil {
		return nil, err
	}
	if found {
		if cand.IsBlocked() {
			return nil, xerrors.Errorf("candidate %s blocked", candAddr)
		}
		cand.Votes = big.Add(votes, cand.Votes)
	} else {
		cand = &Candidate{
			Votes:      votes,
			BlockEpoch: abi.ChainEpoch(0),
		}
	}
	err = setCandidate(candidates, candAddr, cand)
	if err != nil {
		return nil, err
	}
	return cand, nil
}

// Assuming this candidate is eligible.
func (st *State) addToTally(s adt.Store, voter *Voter, candAddr addr.Address, votes abi.TokenAmount) error {

	tally, err := adt.AsMap(s, voter.Tally)
	if err != nil {
		return errors.Wrapf(err, "failed to load tally")
	}

	// set or update tally
	info, found, err := getVotesInfo(tally, candAddr)
	if err != nil {
		return err
	}
	if found {
		info.Votes = big.Add(info.Votes, votes)
	} else {
		info = &VotesInfo{
			Votes:            votes,
			RescindingVotes:  big.Zero(),
			LastRescindEpoch: abi.ChainEpoch(0),
		}
	}

	err = setVotesInfo(tally, candAddr, info)
	if err != nil {
		return err
	}
	voter.Tally, err = tally.Root()
	if err != nil {
		return errors.Wrapf(err, "failed to flush tally")
	}
	return nil
}

// NOTE this method is only for test!
func (st *State) EstimateSettleAll(s adt.Store, cur abi.ChainEpoch) (map[addr.Address]abi.TokenAmount, error) {
	candidates, err := adt.AsMap(s, st.Candidates)
	if err != nil {
		return nil, err
	}

	voters, err := adt.AsMap(s, st.Voters)
	if err != nil {
		return nil, err
	}

	ret := make(map[addr.Address]abi.TokenAmount)

	var voter Voter
	err = voters.ForEach(&voter, func(k string) error {
		vid, err := addr.NewFromBytes([]byte(k))
		if err != nil {
			return err
		}

		err = st.settle(s, &voter, candidates, cur)
		if err != nil {
			return err
		}

		ret[vid] = voter.Withdrawable
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (st *State) EstimateSettle(s adt.Store, voter *Voter, cur abi.ChainEpoch) error {
	candidates, err := adt.AsMap(s, st.Candidates)
	if err != nil {
		return err
	}

	err = st.settle(s, voter, candidates, cur)
	if err != nil {
		return err
	}

	return err
}

func (st *State) settle(s adt.Store, voter *Voter, candidates *adt.Map, cur abi.ChainEpoch) error {

	tally, err := adt.AsMap(s, voter.Tally)
	if err != nil {
		return xerrors.Errorf("failed to load tally: %w", err)
	}

	blockedCands := make(map[abi.ChainEpoch][]*Candidate)
	blockedVotes := make(map[abi.ChainEpoch]abi.TokenAmount)
	totalVotes := big.Zero()
	var info VotesInfo
	err = tally.ForEach(&info, func(key string) error {
		candAddr, err := addr.NewFromBytes([]byte(key))
		if err != nil {
			return err
		}
		cand, found, err := getCandidate(candidates, candAddr)
		if err != nil {
			return err
		}
		AssertMsg(found, "candidate %s not found", candAddr)

		if cand.IsBlocked() {
			if cand.BlockedBefore(voter.SettleEpoch) {
				return nil
			}
			blockedCands[cand.BlockEpoch] = append(blockedCands[cand.BlockEpoch], cand)
			if _, ok := blockedVotes[cand.BlockEpoch]; !ok {
				blockedVotes[cand.BlockEpoch] = info.Votes
			} else {
				blockedVotes[cand.BlockEpoch] = big.Add(blockedVotes[cand.BlockEpoch], info.Votes)
			}
		}
		totalVotes = big.Add(totalVotes, info.Votes)
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to count valid votes in tally")
	}
	blocked := make([][]*Candidate, 0, len(blockedCands))
	for _, sameEpoch := range blockedCands {
		blocked = append(blocked, sameEpoch)
	}
	sort.Slice(blocked, func(i, j int) bool {
		return blocked[i][0].BlockEpoch < blocked[j][0].BlockEpoch
	})

	for _, sameEpoch := range blocked {
		deltaEarningsPerVote := big.Sub(sameEpoch[0].BlockCumEarningsPerVote, voter.SettleCumEarningsPerVote)
		AssertMsg(deltaEarningsPerVote.GreaterThanEqual(big.Zero()), "unexpected negative delta earnigs")

		voter.Withdrawable = big.Add(voter.Withdrawable, big.Mul(totalVotes, deltaEarningsPerVote))
		voter.SettleCumEarningsPerVote = sameEpoch[0].BlockCumEarningsPerVote

		totalVotes = big.Sub(totalVotes, blockedVotes[sameEpoch[0].BlockEpoch])
		Assert(totalVotes.GreaterThanEqual(big.Zero()))
	}
	// st.CumEarningsPerVote is the value in parent epoch if invoked by Vote/Rescind/Withdraw, otherwise that in 'cur'
	deltaEarningsPerVote := big.Sub(st.CumEarningsPerVote, voter.SettleCumEarningsPerVote)
	AssertMsg(deltaEarningsPerVote.GreaterThanEqual(big.Zero()), "negative delta earnigs")

	voter.Withdrawable = big.Add(voter.Withdrawable, big.Mul(totalVotes, deltaEarningsPerVote))
	voter.SettleEpoch = cur
	voter.SettleCumEarningsPerVote = st.CumEarningsPerVote
	return nil
}

func (st *State) withdrawUnlockedVotes(s adt.Store, voter *Voter, cur abi.ChainEpoch) (
	unlocked abi.TokenAmount,
	isVoterEmpty bool,
	err error,
) {

	tally, err := adt.AsMap(s, voter.Tally)
	if err != nil {
		return abi.NewTokenAmount(0), false, errors.Wrapf(err, "failed to load tally")
	}

	deletes := make([]addr.Address, 0)
	updates := make(map[addr.Address]*VotesInfo)
	totalUnlocked := big.Zero()

	count := 0
	var old VotesInfo
	err = tally.ForEach(&old, func(key string) error {
		count++
		if old.RescindingVotes.IsZero() || cur <= old.LastRescindEpoch+RescindingUnlockDelay {
			return nil
		}
		totalUnlocked = big.Add(totalUnlocked, old.RescindingVotes)

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
		updates[candAddr] = &VotesInfo{
			Votes:            old.Votes,
			RescindingVotes:  big.Zero(),
			LastRescindEpoch: old.LastRescindEpoch,
		}
		return nil
	})
	if err != nil {
		return abi.NewTokenAmount(0), false, err
	}
	if totalUnlocked.IsZero() {
		return abi.NewTokenAmount(0), false, nil
	}

	if count == len(deletes) {
		return totalUnlocked, true, nil
	}

	for _, candAddr := range deletes {
		err := tally.Delete(abi.AddrKey(candAddr))
		if err != nil {
			return abi.NewTokenAmount(0), false, errors.Wrapf(err, "failed to delete tally item")
		}
	}
	for candAddr, newInfo := range updates {
		err := setVotesInfo(tally, candAddr, newInfo)
		if err != nil {
			return abi.NewTokenAmount(0), false, err
		}
	}

	voter.Tally, err = tally.Root()
	if err != nil {
		return abi.NewTokenAmount(0), false, errors.Wrap(err, "failed to flush tally")
	}
	return totalUnlocked, false, nil
}

func setCandidate(candidates *adt.Map, candAddr addr.Address, cand *Candidate) error {
	Assert(cand.Votes.GreaterThanEqual(big.Zero()))

	// Should not delete even if candidate has no votes, for it may be inspect in settle.
	if err := candidates.Put(abi.AddrKey(candAddr), cand); err != nil {
		return errors.Wrapf(err, "failed to put candidate %s", candAddr)
	}
	return nil
}

func getCandidate(candidates *adt.Map, candAddr addr.Address) (*Candidate, bool, error) {
	var out Candidate
	found, err := candidates.Get(abi.AddrKey(candAddr), &out)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to get candidate for %v", candAddr)
	}
	if !found {
		return nil, false, nil
	}
	return &out, true, nil
}

func setVoter(voters *adt.Map, voterAddr addr.Address, voter *Voter) error {
	if err := voters.Put(abi.AddrKey(voterAddr), voter); err != nil {
		return errors.Wrapf(err, "failed to put voter %s", voterAddr)
	}
	return nil
}

func deleteVoter(voters *adt.Map, voterAddr addr.Address) error {
	if err := voters.Delete(abi.AddrKey(voterAddr)); err != nil {
		return errors.Wrapf(err, "failed to delete voter %s", voterAddr)
	}
	return nil
}

func getVoter(voters *adt.Map, voterAddr addr.Address) (*Voter, bool, error) {
	var voter Voter
	found, err := voters.Get(abi.AddrKey(voterAddr), &voter)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to get voter %v", voterAddr)
	}
	if !found {
		return nil, false, nil
	}
	return &voter, true, nil
}

func setVotesInfo(tally *adt.Map, candAddr addr.Address, info *VotesInfo) error {
	if err := tally.Put(abi.AddrKey(candAddr), info); err != nil {
		return errors.Wrapf(err, "failed to put tally item for candidate %s", candAddr)
	}
	return nil
}

func getVotesInfo(tally *adt.Map, candAddr addr.Address) (*VotesInfo, bool, error) {
	var info VotesInfo
	found, err := tally.Get(abi.AddrKey(candAddr), &info)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to get tally item for candidate %v", candAddr)
	}
	if !found {
		return nil, false, nil
	}
	return &info, true, nil
}
