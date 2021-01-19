package vote_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/vote"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/filecoin-project/specs-actors/v2/support/ipld"
	"github.com/stretchr/testify/require"

	tutils "github.com/filecoin-project/specs-actors/v2/support/testing"
)

func TestConstruct(t *testing.T) {
	harness := constructStateHarness(t)
	require.Equal(t, abi.NewTokenAmount(0), harness.s.CumEarningsPerVote)
	require.Equal(t, abi.NewTokenAmount(0), harness.s.TotalVotes)
	require.Equal(t, abi.NewTokenAmount(0), harness.s.UnownedFunds)
	require.True(t, harness.s.FallbackReceiver != address.Undef)
}

func TestBlockCandidates(t *testing.T) {
	harness := constructStateHarness(t)
	harness.setCumEarningsPerVote(abi.NewTokenAmount(100))

	t.Run("candidate not found", func(t *testing.T) {
		candidates, err := adt.AsMap(harness.store, harness.s.Candidates, builtin.DefaultHamtBitwidth)
		require.NoError(t, err)

		_, err = harness.s.BlockCandidates(candidates, map[address.Address]struct{}{
			tutils.NewBLSAddr(t, 2): {},
		}, abi.ChainEpoch(1))
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
	h.s.CumEarningsPerVote = val
}

func (h *stateHarness) blockCandidates(curr abi.ChainEpoch, addrs ...address.Address) {
	m := make(map[address.Address]struct{})
	for _, addr := range addrs {
		m[addr] = struct{}{}
	}
	candidates, err := adt.AsMap(h.store, h.s.Candidates, builtin.DefaultHamtBitwidth)
	require.NoError(h.t, err)

	n, err := h.s.BlockCandidates(candidates, m, curr)
	require.NoError(h.t, err)
	require.True(h.t, n == len(addrs))

	h.s.Candidates, err = candidates.Root()
	require.NoError(h.t, err)
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
