package vote

import (
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
	// Total remaining rewards.
	TotalRewards abi.TokenAmount

	// Total valid votes.
	TotalValidVotes abi.TokenAmount

	// Information each candidate.
	Candidates cid.Cid // Map, HAMT[Candidate ID-Address]Candidate

	// Information for each voter.
	Voters cid.Cid // Map, HAMT [Voter ID-Address]Voter
}

type Candidate struct {
	// Epoch at which this candidate was blocked.
	BlockedEpoch abi.ChainEpoch

	// Number of votes currently received.
	Votes abi.TokenAmount
}

func (c *Candidate) IsBlocked() bool {
	return c.BlockedEpoch > 0
}

type Voter struct {
	// Voting record for each candidate.
	VotingRecords cid.Cid // Map, HAMT [Candidate ID-Address]VotingRecord
}

type VotingRecord struct {
	// Number of valid votes for candidate.
	Votes abi.TokenAmount
	// Number of votes being revoked.
	RevokingVotes abi.TokenAmount
	// Epoch during which the last revoking called.
	LastRevokingEpoch abi.ChainEpoch
}

func ConstructState(emptyCandidateMapCid, emptyVoterMapCid cid.Cid) *State {
	return &State{
		Candidates: emptyCandidateMapCid,
		Voters:     emptyVoterMapCid,
	}
}

func (st *State) blockCandidates(candidates *adt.Map, candAddrs map[addr.Address]struct{}, cur abi.ChainEpoch) error {
	for candAddr := range candAddrs {
		cand, found, err := getCandidate(candidates, candAddr)
		if err != nil {
			return err
		}
		if !found || // no votes yet.
			cand.IsBlocked() {
			return nil
		}
		err = setCandidate(candidates, candAddr, &Candidate{Votes: cand.Votes, BlockedEpoch: cur})
		if err != nil {
			return err
		}
		st.TotalValidVotes = big.Sub(st.TotalValidVotes, cand.Votes)
	}
	Assert(st.TotalValidVotes.GreaterThanEqual(big.Zero()))
	return nil
}

// Returns if candidate is blocked
func (st *State) revokeFromCandidate(
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
	if cand.Votes.LessThan(votes) {
		return nil, xerrors.Errorf("insufficient votes to revoke from %s", candAddr)
	}

	cand = &Candidate{
		BlockedEpoch: cand.BlockedEpoch,
		Votes:        big.Sub(cand.Votes, votes),
	}
	err = setCandidate(candidates, candAddr, cand)
	if err != nil {
		return nil, err
	}

	return cand, nil
}

func (st *State) updateVotingRecordForRevoking(s adt.Store, voters *adt.Map,
	voterAddr, candAddr addr.Address, votes abi.TokenAmount, cur abi.ChainEpoch) error {

	oldVoter, found, err := getVoter(voters, voterAddr)
	if err != nil {
		return err
	}
	if !found {
		return xerrors.Errorf("voter %v not exist", voterAddr)
	}
	votingRecords, err := adt.AsMap(s, oldVoter.VotingRecords)
	if err != nil {
		return xerrors.Errorf("failed to load voting records of %s: %w", voterAddr, err)
	}
	oldRecord, found, err := getVotingRecord(votingRecords, candAddr)
	if err != nil {
		return xerrors.Errorf("failed to get record for %s: %w", candAddr, err)
	}
	if !found {
		return xerrors.Errorf("record not exist for %s", candAddr)
	}
	if oldRecord.Votes.LessThan(votes) {
		return xerrors.Errorf("insufficient votes in record")
	}

	// update voting records
	newRecord := &VotingRecord{
		Votes:             big.Sub(oldRecord.Votes, votes),
		RevokingVotes:     big.Add(oldRecord.RevokingVotes, votes),
		LastRevokingEpoch: cur,
	}
	err = setVotingRecord(votingRecords, candAddr, newRecord)
	if err != nil {
		return err
	}
	newVoter := &Voter{}
	newVoter.VotingRecords, err = votingRecords.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush voting records: %w", err)
	}
	return setVoter(voters, voterAddr, newVoter)
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
			return nil, xerrors.Errorf("cannot vote for blocked candidate")
		}
		cand = &Candidate{
			Votes:        big.Add(votes, cand.Votes),
			BlockedEpoch: cand.BlockedEpoch,
		}
	} else {
		cand = &Candidate{
			Votes:        votes,
			BlockedEpoch: abi.ChainEpoch(0),
		}
	}
	err = setCandidate(candidates, candAddr, cand)
	if err != nil {
		return nil, err
	}
	return cand, nil
}

// Assuming this candidate is eligible.
func (st *State) addVotingRecord(s adt.Store, voters *adt.Map,
	voterAddr, candAddr addr.Address, votes abi.TokenAmount) error {

	// set or update voter
	oldVoter, found, err := getVoter(voters, voterAddr)
	if err != nil {
		return err
	}

	var voterRecords *adt.Map
	if found {
		voterRecords, err = adt.AsMap(s, oldVoter.VotingRecords)
		if err != nil {
			return xerrors.Errorf("failed to load voting records of %s: %w", voterAddr, err)
		}
	} else {
		voterRecords = adt.MakeEmptyMap(s)
	}

	// set or update voting records
	record, found, err := getVotingRecord(voterRecords, candAddr)
	if err != nil {
		return xerrors.Errorf("failed to get voting record for %s: %w", candAddr, err)
	}
	if found {
		record = &VotingRecord{
			Votes:             big.Add(record.Votes, votes),
			RevokingVotes:     record.RevokingVotes,
			LastRevokingEpoch: record.LastRevokingEpoch,
		}
	} else {
		record = &VotingRecord{
			Votes:             votes,
			RevokingVotes:     big.Zero(),
			LastRevokingEpoch: abi.ChainEpoch(0),
		}
	}

	err = setVotingRecord(voterRecords, candAddr, record)
	if err != nil {
		return xerrors.Errorf("failed to put voting record for %s: %w", candAddr, err)
	}
	newVoter := &Voter{}
	newVoter.VotingRecords, err = voterRecords.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush voting records of %s for %s: %w", voterAddr, candAddr, err)
	}
	return setVoter(voters, voterAddr, newVoter)
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

func setVotingRecord(voterRecords *adt.Map, candAddr addr.Address, record *VotingRecord) error {
	if err := voterRecords.Put(abi.AddrKey(candAddr), record); err != nil {
		return errors.Wrapf(err, "failed to put voting record for candidate %s", candAddr)
	}
	return nil
}

func deleteVoter(voters *adt.Map, voterAddr addr.Address) error {
	err := voters.Delete(abi.AddrKey(voterAddr))
	if err != nil {
		return errors.Wrapf(err, "failed to delete voter %s", voterAddr)
	}
	return nil
}

func updateVotingRecords(voterRecords *adt.Map, deletes []addr.Address, updates map[addr.Address]*VotingRecord) error {
	for _, candAddr := range deletes {
		err := voterRecords.Delete(abi.AddrKey(candAddr))
		if err != nil {
			return errors.Wrapf(err, "failed to delete voting record for candidate %s", candAddr)
		}
	}
	for candAddr, newRecord := range updates {
		err := setVotingRecord(voterRecords, candAddr, newRecord)
		if err != nil {
			return err
		}
	}
	return nil
}

func getVotingRecord(voterRecords *adt.Map, candAddr addr.Address) (*VotingRecord, bool, error) {
	var record VotingRecord
	found, err := voterRecords.Get(abi.AddrKey(candAddr), &record)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to get voting record for candidate %v", candAddr)
	}
	if !found {
		return nil, false, nil
	}
	return &record, true, nil
}
