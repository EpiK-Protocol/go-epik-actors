package vote

import (
	"sort"

	"github.com/filecoin-project/go-address"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

type State struct {

	// Information for each candidate.
	Candidates cid.Cid // Map, HAMT[Candidate ID-Address]Candidate

	// Information for each voter.
	Voters cid.Cid // Map, HAMT [Voter ID-Address]Voter

	TotalVotes abi.TokenAmount // All unwithdrawn votes, including active, blocked and rescinded

	// pool info
	CurrEpoch                abi.ChainEpoch
	CurrEpochEffectiveVotes  abi.TokenAmount // current epoch
	CurrEpochRewards         abi.TokenAmount // current epoch
	PrevEpoch                abi.ChainEpoch
	PrevEpochEarningsPerVote abi.TokenAmount // Cumulative earnings per vote(atto) since genesis. Updated when epoch changed
	LastRewardBalance        abi.TokenAmount // Block rewards balance (rt.Balance - TotalVotes)

	// Fallback rewards receiver when no votes
	FallbackReceiver address.Address
	FallbackDebt     abi.TokenAmount
}

type Candidate struct {
	// Epoch in which this candidate was firstly blocked.
	BlockEpoch abi.ChainEpoch

	// PrevEpochEarningsPerVote in epoch just previous to BlockEpoch.
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
	// PrevEpochEarningsPerVote in epoch just previous to LastSettleEpoch.
	SettleCumEarningsPerVote abi.TokenAmount

	// Withdrawable rewards since last withdrawal.
	Withdrawable abi.TokenAmount

	// Tally for each candidate.
	Tally     cid.Cid // Map, HAMT [Candidate ID-Address]VotesInfo
	PrevTally cid.Cid // Updated to Tally when epoch changed
}

type VotesInfo struct {
	// Number of valid votes(atto) for candidate.
	Votes abi.TokenAmount
	// Number of votes being rescinded.
	RescindingVotes abi.TokenAmount
	// Epoch during which the last rescind called.
	LastRescindEpoch abi.ChainEpoch
}

func ConstructState(store adt.Store, fallback address.Address) (*State, error) {
	if fallback.Protocol() != addr.ID {
		return nil, xerrors.New("fallback not a ID-Address")
	}

	emptyCandidatesMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty candidate map: %w", err)
	}

	emptyVotesMapCid, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, xerrors.Errorf("failed to create empty vote map: %w", err)
	}

	return &State{
		Candidates: emptyCandidatesMapCid,
		Voters:     emptyVotesMapCid,
		TotalVotes: abi.NewTokenAmount(0),

		FallbackReceiver: fallback,
		FallbackDebt:     abi.NewTokenAmount(0),

		CurrEpochEffectiveVotes:  abi.NewTokenAmount(0),
		CurrEpochRewards:         abi.NewTokenAmount(0),
		PrevEpochEarningsPerVote: abi.NewTokenAmount(0),
		LastRewardBalance:        abi.NewTokenAmount(0),
	}, nil
}

func (st *State) BlockCandidates(store adt.Store, currEpoch abi.ChainEpoch, currRewardBalance abi.TokenAmount, candAddrs ...addr.Address) (int, error) {

	err := st.UpdatePool(store, currEpoch, currRewardBalance)
	if err != nil {
		return 0, xerrors.Errorf("failed to update pool: %w", err)
	}

	candidates, err := adt.AsMap(store, st.Candidates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return 0, xerrors.Errorf("failed to load candidates: %w", err)
	}

	blocked := 0
	for _, candAddr := range candAddrs {
		cand, found, err := getCandidate(candidates, candAddr)
		if err != nil {
			return 0, err
		}
		if !found {
			return 0, xerrors.Errorf("candidate not found: %s", candAddr)
		}

		if cand.IsBlocked() {
			continue
		}

		cand.BlockEpoch = currEpoch
		cand.BlockCumEarningsPerVote = st.PrevEpochEarningsPerVote
		err = setCandidate(candidates, candAddr, cand)
		if err != nil {
			return 0, err
		}
		st.CurrEpochEffectiveVotes = big.Sub(st.CurrEpochEffectiveVotes, cand.Votes)
		if st.CurrEpochEffectiveVotes.LessThan(big.Zero()) {
			return 0, xerrors.Errorf("negative total votes %v after sub %v for blocking", st.CurrEpochEffectiveVotes, cand.Votes)
		}

		blocked++
	}

	st.Candidates, err = candidates.Root()
	if err != nil {
		return 0, xerrors.Errorf("failed to flush candidates: %w", err)
	}

	return blocked, nil
}

func (st *State) RescindVotes(
	store adt.Store,
	voter *Voter,
	candAddr addr.Address,
	votes abi.TokenAmount,
	currEpoch abi.ChainEpoch,
	rewardBalance abi.TokenAmount,
) (abi.TokenAmount, error) {

	err := st.Settle(store, voter, currEpoch, rewardBalance)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}

	tally, err := adt.AsMap(store, voter.Tally, builtin.DefaultHamtBitwidth)
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to load tally: %w", err)
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
	info.LastRescindEpoch = currEpoch
	err = setVotesInfo(tally, candAddr, info)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}
	voter.Tally, err = tally.Root()
	if err != nil {
		return abi.NewTokenAmount(0), xerrors.Errorf("failed to flush tally: %w", err)
	}
	return votes, nil
}

// Assuming this candidate is eligible.
func (st *State) AddVotes(store adt.Store, voter *Voter, candAddr addr.Address, votes abi.TokenAmount, currEpoch abi.ChainEpoch, rewardBalance abi.TokenAmount) error {

	err := st.Settle(store, voter, currEpoch, rewardBalance)
	if err != nil {
		return err
	}

	tally, err := adt.AsMap(store, voter.Tally, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load tally: %w", err)
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
		return xerrors.Errorf("failed to flush tally: %w", err)
	}
	return nil
}

// NOTE this method is only for test!
func (st *State) EstimateSettleAll(s adt.Store, currEpoch abi.ChainEpoch, currBalance abi.TokenAmount) (map[addr.Address]abi.TokenAmount, error) {
	voters, err := adt.AsMap(s, st.Voters, builtin.DefaultHamtBitwidth)
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

		err = st.Settle(s, &voter, currEpoch, currBalance)
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

// Arguments currEpoch and currRewardBalance must match current state
func (st *State) EstimateSettle(store adt.Store, voterAddr addr.Address, currEpoch abi.ChainEpoch, currRewardBalance abi.TokenAmount) (*Voter, error) {
	voter, found, err := st.GetVoter(store, voterAddr)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, xerrors.Errorf("voter not found")
	}

	err = st.Settle(store, voter, currEpoch, currRewardBalance)
	if err != nil {
		return nil, err
	}

	return voter, nil
}

func (st *State) Settle(store adt.Store, voter *Voter, currEpoch abi.ChainEpoch, currRewardBalance abi.TokenAmount) error {

	err := st.UpdatePool(store, currEpoch, currRewardBalance)
	if err != nil {
		return xerrors.Errorf("failed to update pool: %w", err)
	}

	if voter.SettleEpoch == currEpoch {
		// already settled
		return nil
	}

	candidates, err := adt.AsMap(store, st.Candidates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load candidates: %w", err)
	}

	voter.PrevTally = voter.Tally
	prevTally, err := adt.AsMap(store, voter.PrevTally, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load tally: %w", err)
	}

	blockedCands := make(map[abi.ChainEpoch][]*Candidate)
	blockedVotes := make(map[abi.ChainEpoch]abi.TokenAmount)
	prevTotalVotes := big.Zero()
	var info VotesInfo
	err = prevTally.ForEach(&info, func(key string) error {
		candAddr, err := addr.NewFromBytes([]byte(key))
		if err != nil {
			return err
		}
		cand, found, err := getCandidate(candidates, candAddr)
		if err != nil {
			return err
		}
		if !found {
			return xerrors.Errorf("candidate not found %s", candAddr)
		}

		if cand.IsBlocked() && cand.BlockEpoch < currEpoch {
			if cand.BlockedBefore(voter.SettleEpoch) {
				// invalid votes since last settlement
				return nil
			}
			blockedCands[cand.BlockEpoch] = append(blockedCands[cand.BlockEpoch], cand)
			if _, ok := blockedVotes[cand.BlockEpoch]; !ok {
				blockedVotes[cand.BlockEpoch] = info.Votes
			} else {
				blockedVotes[cand.BlockEpoch] = big.Add(blockedVotes[cand.BlockEpoch], info.Votes)
			}
		}
		prevTotalVotes = big.Add(prevTotalVotes, info.Votes)
		return nil
	})
	if err != nil {
		return xerrors.Errorf("failed to count valid votes in tally: %w", err)
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
		if deltaEarningsPerVote.LessThan(big.Zero()) {
			return xerrors.Errorf("negative delta earnigs %v after sub1 %v", deltaEarningsPerVote, voter.SettleCumEarningsPerVote)
		}

		voter.Withdrawable = big.Add(voter.Withdrawable, big.Div(big.Mul(prevTotalVotes, deltaEarningsPerVote), Multiplier1E12))
		voter.SettleCumEarningsPerVote = sameEpoch[0].BlockCumEarningsPerVote

		prevTotalVotes = big.Sub(prevTotalVotes, blockedVotes[sameEpoch[0].BlockEpoch])
		if prevTotalVotes.LessThan(big.Zero()) {
			return xerrors.Errorf("negative total votes %v after sub %v, blocked at %d", prevTotalVotes, blockedVotes[sameEpoch[0].BlockEpoch], sameEpoch[0].BlockEpoch)
		}
	}

	deltaEarningsPerVote := big.Sub(st.PrevEpochEarningsPerVote, voter.SettleCumEarningsPerVote)
	if deltaEarningsPerVote.LessThan(big.Zero()) {
		return xerrors.Errorf("negative delta earnings %v after sub2 %v", deltaEarningsPerVote, voter.SettleCumEarningsPerVote)
	}

	voter.Withdrawable = big.Add(voter.Withdrawable, big.Div(big.Mul(prevTotalVotes, deltaEarningsPerVote), Multiplier1E12))
	voter.SettleEpoch = currEpoch
	voter.SettleCumEarningsPerVote = st.PrevEpochEarningsPerVote
	return nil
}

func (st *State) UpdatePool(store adt.Store, currEpoch abi.ChainEpoch, currRewardBalance abi.TokenAmount) error {

	if currRewardBalance.LessThan(st.LastRewardBalance) {
		return xerrors.Errorf("unexpected current balance %s less than state.LastRewardBalance %s", currRewardBalance, st.LastRewardBalance)
	}
	deltaRewards := big.Sub(currRewardBalance, st.LastRewardBalance)

	if currEpoch < st.CurrEpoch {
		return xerrors.Errorf("unexpected rt.CurrEpoch %d less than pool.CurrEpoch", currEpoch, st.CurrEpoch)
	}
	if currEpoch > st.CurrEpoch {
		if st.CurrEpochEffectiveVotes.IsZero() {
			st.FallbackDebt = big.Add(st.FallbackDebt, st.CurrEpochRewards)
		} else {
			deltaPerVote := big.Div(big.Mul(st.CurrEpochRewards, Multiplier1E12), st.CurrEpochEffectiveVotes)
			st.PrevEpochEarningsPerVote = big.Add(st.PrevEpochEarningsPerVote, deltaPerVote)
		}

		st.PrevEpoch = st.CurrEpoch
		st.CurrEpoch = currEpoch
		st.CurrEpochRewards = deltaRewards
	} else {
		st.CurrEpochRewards = big.Add(st.CurrEpochRewards, deltaRewards)
	}
	st.LastRewardBalance = currRewardBalance
	return nil
}

func (st *State) WithdrawUnlockedVotes(store adt.Store, voter *Voter, currEpoch abi.ChainEpoch, rewardBalance abi.TokenAmount) (
	unlocked abi.TokenAmount,
	isVoterEmpty bool,
	err error,
) {

	err = st.Settle(store, voter, currEpoch, rewardBalance)
	if err != nil {
		return big.Zero(), false, err
	}

	tally, err := adt.AsMap(store, voter.Tally, builtin.DefaultHamtBitwidth)
	if err != nil {
		return abi.NewTokenAmount(0), false, xerrors.Errorf("failed to load tally: %w", err)
	}

	deletes := make([]addr.Address, 0)
	updates := make(map[addr.Address]*VotesInfo)
	totalUnlocked := big.Zero()

	count := 0
	var old VotesInfo
	err = tally.ForEach(&old, func(key string) error {
		count++
		if old.RescindingVotes.IsZero() || currEpoch <= old.LastRescindEpoch+RescindingUnlockDelay {
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
			return abi.NewTokenAmount(0), false, xerrors.Errorf("failed to delete tally item: %w", err)
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
		return abi.NewTokenAmount(0), false, xerrors.Errorf("failed to flush tally: %w", err)
	}
	return totalUnlocked, false, nil
}

func (st *State) GetVoter(store adt.Store, voterAddr addr.Address) (*Voter, bool, error) {
	voters, err := adt.AsMap(store, st.Voters, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, false, err
	}

	var voter Voter
	found, err := voters.Get(abi.AddrKey(voterAddr), &voter)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to get voter %s: %w", voterAddr, err)
	}
	return &voter, found, err
}

func (st *State) PutVoter(store adt.Store, voterAddr addr.Address, voter *Voter) error {
	voters, err := adt.AsMap(store, st.Voters, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load voters: %w", err)
	}

	err = voters.Put(abi.AddrKey(voterAddr), voter)
	if err != nil {
		return xerrors.Errorf("failed to put voter: %w", err)
	}

	st.Voters, err = voters.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush voters: %w", err)
	}
	return nil
}

func (st *State) DeleteVoter(store adt.Store, voterAddr addr.Address) error {
	voters, err := adt.AsMap(store, st.Voters, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load voters: %w", err)
	}

	if err := voters.Delete(abi.AddrKey(voterAddr)); err != nil {
		return xerrors.Errorf("failed to delete voter %s: %w", voterAddr, err)
	}

	st.Voters, err = voters.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush voters: %w", err)
	}
	return nil
}

func newVoter(store adt.Store, currEpoch abi.ChainEpoch, currCumEarningsPerVote abi.TokenAmount) (*Voter, error) {
	tally, err := adt.StoreEmptyMap(store, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, err
	}
	return &Voter{
		SettleEpoch:              currEpoch,
		SettleCumEarningsPerVote: currCumEarningsPerVote,
		Withdrawable:             abi.NewTokenAmount(0),
		Tally:                    tally,
		PrevTally:                tally,
	}, nil
}

func newCandidate() *Candidate {
	return &Candidate{
		Votes:                   abi.NewTokenAmount(0),
		BlockCumEarningsPerVote: abi.NewTokenAmount(0),
		BlockEpoch:              abi.ChainEpoch(0),
	}
}

func (st *State) ListVotesInfo(store adt.Store, voterAddr addr.Address) (map[addr.Address]VotesInfo, error) {
	voter, found, err := st.GetVoter(store, voterAddr)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, xerrors.Errorf("voter not found %s", voterAddr)
	}

	tally, err := adt.AsMap(store, voter.Tally, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, err
	}

	ret := make(map[addr.Address]VotesInfo)
	var out VotesInfo
	err = tally.ForEach(&out, func(k string) error {
		cand, err := addr.NewFromBytes([]byte(k))
		if err != nil {
			return err
		}
		ret[cand] = out
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func setCandidate(candidates *adt.Map, candAddr addr.Address, cand *Candidate) error {
	if cand.Votes.LessThan(big.Zero()) {
		return xerrors.Errorf("negative votes %v of candidate %s to put", cand.Votes, candAddr)
	}

	// Should not delete even if candidate has no votes, for it may be inspect in settle.
	if err := candidates.Put(abi.AddrKey(candAddr), cand); err != nil {
		return xerrors.Errorf("failed to put candidate %s: %w", candAddr, err)
	}
	return nil
}

func getCandidate(candidates *adt.Map, candAddr addr.Address) (*Candidate, bool, error) {
	var out Candidate
	found, err := candidates.Get(abi.AddrKey(candAddr), &out)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to get candidate for %v: %w", candAddr, err)
	}
	return &out, found, nil
}

func (st *State) GetCandidate(store adt.Store, candAddr addr.Address) (*Candidate, bool, error) {
	candidates, err := adt.AsMap(store, st.Candidates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to load candidates: %w", err)
	}
	return getCandidate(candidates, candAddr)
}

func (st *State) PutCandidate(store adt.Store, candAddr addr.Address, cand *Candidate) error {
	candidates, err := adt.AsMap(store, st.Candidates, builtin.DefaultHamtBitwidth)
	if err != nil {
		return xerrors.Errorf("failed to load candidates: %w", err)
	}
	if err = setCandidate(candidates, candAddr, cand); err != nil {
		return err
	}
	st.Candidates, err = candidates.Root()
	if err != nil {
		return xerrors.Errorf("failed to flush candidates: %w", err)
	}
	return nil
}

func setVotesInfo(tally *adt.Map, candAddr addr.Address, info *VotesInfo) error {
	if err := tally.Put(abi.AddrKey(candAddr), info); err != nil {
		return xerrors.Errorf("failed to put tally item for candidate %s: %w", candAddr, err)
	}
	return nil
}

func getVotesInfo(tally *adt.Map, candAddr addr.Address) (*VotesInfo, bool, error) {
	var info VotesInfo
	found, err := tally.Get(abi.AddrKey(candAddr), &info)
	if err != nil {
		return nil, false, xerrors.Errorf("failed to get tally item for candidate %v: %w", candAddr, err)
	}
	if !found {
		return nil, false, nil
	}
	return &info, true, nil
}
