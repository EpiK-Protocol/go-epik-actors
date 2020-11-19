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
	TotalVotes abi.TokenAmount

	// Total votes revoked but not withdrawn.
	TotalRevokingVotes abi.TokenAmount

	// Information each candidate.
	Candidates cid.Cid // Map, HAMT[Candidate ID-Address]Candidate

	// Information for each voter.
	Voters cid.Cid // Map, HAMT [Voter ID-Address]Voter
}

type Candidate struct {
	// If true, this candidate cannot be voted for but revoke votes.
	Closed bool

	// Number of votes currently received.
	Votes abi.TokenAmount
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

func (st *State) registerCandidates(candidates *adt.Map, candAddrs []addr.Address) error {
	for _, candAddr := range candAddrs {
		_, ok, err := getCandidate(candidates, candAddr)
		if err != nil {
			return err
		}
		if ok {
			return xerrors.Errorf("duplicate candidate %s", candAddr)
		}
		err = setCandidate(candidates, candAddr, &Candidate{Votes: big.Zero(), Closed: false})
		if err != nil {
			return err
		}
	}
	return nil
}

func (st *State) revoke(s adt.Store, candidates, voters *adt.Map,
	voterAddr, candAddr addr.Address, votes abi.TokenAmount, cur abi.ChainEpoch) error {

	{
		oldVoter, found, err := getVoter(voters, voterAddr)
		if err != nil {
			return err
		}
		if !found {
			return xerrors.Errorf("%v has no votes", voterAddr)
		}
		voterRecords, err := adt.AsMap(s, oldVoter.VotingRecords)
		if err != nil {
			return xerrors.Errorf("failed to load voting records of %s: %w", voterAddr, err)
		}
		oldRecord, found, err := getVotingRecord(voterRecords, candAddr)
		if err != nil {
			return xerrors.Errorf("failed to get voting record for %s: %w", candAddr, err)
		}
		if !found {
			return xerrors.Errorf("%v has no votes for %v", voterAddr, candAddr)
		}
		if oldRecord.Votes.LessThan(votes) {
			return xerrors.Errorf("not enough votes to revoke")
		}

		// update voting records
		newRecord := &VotingRecord{
			Votes:             big.Sub(oldRecord.Votes, votes),
			RevokingVotes:     big.Add(oldRecord.RevokingVotes, votes),
			LastRevokingEpoch: cur,
		}
		err = setVotingRecord(voterRecords, candAddr, newRecord)
		if err != nil {
			return err
		}
		newVoter := &Voter{}
		newVoter.VotingRecords, err = voterRecords.Root()
		err = setVoter(voters, voterAddr, newVoter)
		if err != nil {
			return err
		}
	}

	oldCand, found, err := getCandidate(candidates, candAddr)
	if err != nil {
		return err
	}
	AssertMsg(found, "candidate %v not exist", candAddr)
	AssertMsg(oldCand.Votes.GreaterThanEqual(votes), "candidate %v has no enough votes %v", candAddr, oldCand.Votes)

	newCand := &Candidate{
		Closed: oldCand.Closed,
		Votes:  big.Sub(oldCand.Votes, votes),
	}
	return setCandidate(candidates, candAddr, newCand)
}

func (st *State) vote(s adt.Store, candidates, voters *adt.Map,
	voterAddr, candAddr addr.Address, votes abi.TokenAmount) error {

	oldCand, ok, err := getCandidate(candidates, candAddr)
	if err != nil {
		return err
	}
	if !ok {
		return xerrors.Errorf("candidate %v not exist", candAddr)
	}
	if oldCand.Closed {
		return xerrors.Errorf("vote for candidate %v not allowed", candAddr)
	}
	newCand := &Candidate{
		Votes: big.Add(votes, oldCand.Votes),
	}
	err = setCandidate(candidates, candAddr, newCand)
	if err != nil {
		return err
	}

	// set or update voter
	newVoter := &Voter{}
	var voterRecords *adt.Map

	oldVoter, found, err := getVoter(voters, voterAddr)
	if err != nil {
		return err
	}
	if found {
		voterRecords, err = adt.AsMap(s, oldVoter.VotingRecords)
		if err != nil {
			return xerrors.Errorf("failed to load voting records of %s: %w", voterAddr, err)
		}
	} else {
		voterRecords = adt.MakeEmptyMap(s)
	}

	var newRecord *VotingRecord
	oldRecord, found, err := getVotingRecord(voterRecords, candAddr)
	if err != nil {
		return xerrors.Errorf("failed to get voting record of %s: %w", voterAddr, err)
	}
	if found {
		newRecord = &VotingRecord{
			Votes:             big.Add(oldRecord.Votes, votes),
			RevokingVotes:     oldRecord.RevokingVotes,
			LastRevokingEpoch: oldRecord.LastRevokingEpoch,
		}
	} else {
		newRecord = &VotingRecord{
			Votes:             votes,
			RevokingVotes:     big.Zero(),
			LastRevokingEpoch: abi.ChainEpoch(0),
		}
	}

	err = setVotingRecord(voterRecords, candAddr, newRecord)
	if err != nil {
		return xerrors.Errorf("failed to put voting record of %s: %w", voterAddr, err)
	}
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
