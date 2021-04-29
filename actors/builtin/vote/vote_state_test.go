package vote_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/vote"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/filecoin-project/specs-actors/v2/support/ipld"
	"github.com/stretchr/testify/require"

	tutils "github.com/filecoin-project/specs-actors/v2/support/testing"
)

func TestConstruct(t *testing.T) {
	harness := constructStateHarness(t)
	require.Equal(t, abi.NewTokenAmount(0), harness.s.PrevEpochEarningsPerVote)
	require.Equal(t, abi.NewTokenAmount(0), harness.s.CurrEpochEffectiveVotes)
	require.Equal(t, abi.NewTokenAmount(0), harness.s.CurrEpochRewards)
	require.Equal(t, abi.NewTokenAmount(0), harness.s.LastRewardBalance)
	require.Equal(t, abi.NewTokenAmount(0), harness.s.FallbackDebt)
	require.Equal(t, abi.NewTokenAmount(0), harness.s.TotalVotes)
	require.Equal(t, abi.ChainEpoch(0), harness.s.PrevEpoch)
	require.Equal(t, abi.ChainEpoch(0), harness.s.CurrEpoch)
	require.True(t, harness.s.FallbackReceiver != address.Undef)
}

func TestBlockCandidates(t *testing.T) {
	harness := constructStateHarness(t)
	harness.setCumEarningsPerVote(abi.NewTokenAmount(100))

	t.Run("candidate not found", func(t *testing.T) {
		_, err := harness.s.BlockCandidates(harness.store, 1, big.Zero(), tutils.NewBLSAddr(t, 2))
		require.Error(t, err)
		require.Contains(t, err.Error(), "candidate not found")
	})
}

type stateHarness struct {
	t testing.TB

	s     *vote.State
	store adt.Store

	fallback address.Address
}

func (h *stateHarness) setCumEarningsPerVote(val abi.TokenAmount) {
	h.s.PrevEpochEarningsPerVote = val
}

func (h *stateHarness) blockCandidates(curr abi.ChainEpoch, currBal abi.TokenAmount, addrs ...address.Address) {
	n, err := h.s.BlockCandidates(h.store, curr, currBal, addrs...)
	require.NoError(h.t, err)
	require.True(h.t, n == len(addrs))
}

func constructStateHarness(t *testing.T) *stateHarness {
	store := ipld.NewADTStore(context.Background())
	fallback := tutils.NewIDAddr(t, 1)
	state, err := vote.ConstructState(store, fallback)
	require.NoError(t, err)

	return &stateHarness{
		t:        t,
		s:        state,
		store:    store,
		fallback: fallback,
	}
}
