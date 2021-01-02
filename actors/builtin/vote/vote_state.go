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
	// Cumulative earnings per vote(atto) since genesis.
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

	// Cumulative unclaimed funds, including rewards and unlocked votes, since last withdrawal.
	UnclaimedFunds abi.TokenAmount

	// Voting record for each candidate.
	VotingRecords cid.Cid // Map, HAMT [Candidate ID-Address]VotingRecord
}

type VotingRecord struct {
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

func (st *State) blockCandidates(candidates *adt.Map, candAddrs map[addr.Address]struct{}, cur abi.ChainEpoch) error {
	for candAddr := range candAddrs {
		cand, found, err := getCandidate(candidates, candAddr)
		if err != nil {
			return err
		}
		AssertMsg(found, "candidate %s not found", candAddr)
		AssertMsg(!cand.IsBlocked(), "candidate %s already blocked", candAddr)

		cand.BlockEpoch = cur
		cand.BlockCumEarningsPerVote = st.CumEarningsPerVote
		err = setCandidate(candidates, candAddr, cand)
		if err != nil {
			return err
		}
		st.TotalVotes = big.Sub(st.TotalVotes, cand.Votes)
		Assert(st.TotalVotes.GreaterThanEqual(big.Zero()))
	}
	return nil
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

func (st *State) subFromVotingRecord(
	s adt.Store,
	voter *Voter,
	candAddr addr.Address,
	votes abi.TokenAmount,
	cur abi.ChainEpoch,
) (abi.TokenAmount, error) {

	votingRecords, err := adt.AsMap(s, voter.VotingRecords)
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to load voting records: %w", err)
	}
	record, found, err := getVotingRecord(votingRecords, candAddr)
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to get record for %s: %w", candAddr, err)
	}
	if !found {
		return abi.NewTokenAmount(0), xerrors.Errorf("no votes for %s", candAddr)
	}
	if record.Votes.LessThan(votes) {
		votes = record.Votes
	}

	// update voting records
	record.Votes = big.Sub(record.Votes, votes)
	record.RescindingVotes = big.Add(record.RescindingVotes, votes)
	record.LastRescindEpoch = cur
	err = setVotingRecord(votingRecords, candAddr, record)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}
	voter.VotingRecords, err = votingRecords.Root()
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to flush voting records: %w", err)
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
			return cand, nil
		}
		cand = &Candidate{
			Votes:      big.Add(votes, cand.Votes),
			BlockEpoch: cand.BlockEpoch,
		}
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
func (st *State) addVotingRecord(s adt.Store, voter *Voter, candAddr addr.Address, votes abi.TokenAmount) error {

	votingRecords, err := adt.AsMap(s, voter.VotingRecords)
	if err != nil {
		return xerrors.Errorf("failed to load voting records: %w", err)
	}

	// set or update voting records
	record, found, err := getVotingRecord(votingRecords, candAddr)
	if err != nil {
		return xerrors.Errorf("failed to get voting record for %s: %w", candAddr, err)
	}
	if found {
		record = &VotingRecord{
			Votes:            big.Add(record.Votes, votes),
			RescindingVotes:  record.RescindingVotes,
			LastRescindEpoch: record.LastRescindEpoch,
		}
	} else {
		record = &VotingRecord{
			Votes:            votes,
			RescindingVotes:  big.Zero(),
			LastRescindEpoch: abi.ChainEpoch(0),
		}
	}

	err = setVotingRecord(votingRecords, candAddr, record)
	if err != nil {
		return xerrors.Errorf("failed to put voting record for %s: %w", candAddr, err)
	}
	voter.VotingRecords, err = votingRecords.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush voting records: %w", err)
	}
	return nil
}

func (st *State) settle(s adt.Store, voter *Voter, candidates *adt.Map, cur abi.ChainEpoch) error {

	votingRecords, err := adt.AsMap(s, voter.VotingRecords)
	if err != nil {
		return xerrors.Errorf("failed to load voting records: %w", err)
	}

	blockedCands := make(map[abi.ChainEpoch][]*Candidate)
	blockedVotes := make(map[abi.ChainEpoch]abi.TokenAmount)
	totalVotes := big.Zero()
	var record VotingRecord
	err = votingRecords.ForEach(&record, func(key string) error {
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
				blockedVotes[cand.BlockEpoch] = record.Votes
			} else {
				blockedVotes[cand.BlockEpoch] = big.Add(blockedVotes[cand.BlockEpoch], record.Votes)
			}
		}
		totalVotes = big.Add(totalVotes, record.Votes)
		return nil
	})
	if err != nil {
		return err
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
		voter.UnclaimedFunds = big.Add(voter.UnclaimedFunds, big.Mul(totalVotes, deltaEarningsPerVote))
		voter.SettleCumEarningsPerVote = sameEpoch[0].BlockCumEarningsPerVote
		totalVotes = big.Sub(totalVotes, blockedVotes[sameEpoch[0].BlockEpoch])
		Assert(totalVotes.GreaterThanEqual(big.Zero()))
	}
	deltaEarningsPerVote := big.Sub(st.CumEarningsPerVote, voter.SettleCumEarningsPerVote)
	voter.UnclaimedFunds = big.Add(voter.UnclaimedFunds, big.Mul(totalVotes, deltaEarningsPerVote))
	voter.SettleEpoch = cur
	voter.SettleCumEarningsPerVote = st.CumEarningsPerVote
	return nil
}

func (st *State) claimUnlockedVotes(s adt.Store, voter *Voter, cur abi.ChainEpoch) (abi.TokenAmount, error) {

	votingRecords, err := adt.AsMap(s, voter.VotingRecords)
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to load voting records: %w", err)
	}

	deletes := make([]addr.Address, 0)
	updates := make(map[addr.Address]*VotingRecord)
	totalUnlocked := big.Zero()

	var old VotingRecord
	err = votingRecords.ForEach(&old, func(key string) error {
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
		updates[candAddr] = &VotingRecord{
			Votes:            old.Votes,
			RescindingVotes:  big.Zero(),
			LastRescindEpoch: old.LastRescindEpoch,
		}
		return nil
	})
	if err != nil {
		return abi.NewTokenAmount(0), err
	}
	if totalUnlocked.IsZero() {
		return abi.NewTokenAmount(0), nil
	}

	for _, candAddr := range deletes {
		err := votingRecords.Delete(abi.AddrKey(candAddr))
		if err != nil {
			return abi.NewTokenAmount(0), errors.Wrapf(err, "failed to delete voting record")
		}
	}
	for candAddr, newRecord := range updates {
		err := setVotingRecord(votingRecords, candAddr, newRecord)
		if err != nil {
			return abi.NewTokenAmount(0), err
		}
	}

	voter.VotingRecords, err = votingRecords.Root()
	if err != nil {
		return abi.NewTokenAmount(0), errors.Wrapf(err, "failed to flush voting records: %w")
	}
	return totalUnlocked, nil
}

func setCandidate(candidates *adt.Map, candAddr addr.Address, cand *Candidate) error {
	Assert(cand.Votes.GreaterThan(big.Zero()))
	if err := candidates.Put(abi.AddrKey(candAddr), cand); err != nil {
		return errors.Wrapf(err, "failed to put candidate for %s votes %v", candAddr, cand)
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

func newEmptyVoter(s adt.Store) (*Voter, error) {
	records, err := adt.MakeEmptyMap(s).Root()
	if err != nil {
		return nil, err
	}
	return &Voter{
		SettleEpoch:              abi.ChainEpoch(0),
		SettleCumEarningsPerVote: abi.NewTokenAmount(0),
		UnclaimedFunds:           abi.NewTokenAmount(0),
		VotingRecords:            records,
	}, nil
}

func setVoter(voters *adt.Map, voterAddr addr.Address, voter *Voter) error {
	if err := voters.Put(abi.AddrKey(voterAddr), voter); err != nil {
		return errors.Wrapf(err, "failed to put voter for %s", voterAddr)
	}
	return nil
}

func getVoter(voters *adt.Map, voterAddr addr.Address) (*Voter, bool, error) {
	var voter Voter
	found, err := voters.Get(abi.AddrKey(voterAddr), &voter)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to get voter for %v", voterAddr)
	}
	if !found {
		return nil, false, nil
	}
	return &voter, true, nil
}

func setVotingRecord(votingRecords *adt.Map, candAddr addr.Address, record *VotingRecord) error {
	if err := votingRecords.Put(abi.AddrKey(candAddr), record); err != nil {
		return errors.Wrapf(err, "failed to put voting record for candidate %s", candAddr)
	}
	return nil
}

func getVotingRecord(votingRecords *adt.Map, candAddr addr.Address) (*VotingRecord, bool, error) {
	var record VotingRecord
	found, err := votingRecords.Get(abi.AddrKey(candAddr), &record)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to get voting record for candidate %v", candAddr)
	}
	if !found {
		return nil, false, nil
	}
	return &record, true, nil
}
