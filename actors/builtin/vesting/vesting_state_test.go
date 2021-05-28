package vesting_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	abi "github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/vesting"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/specs-actors/v2/support/ipld"
	tutils "github.com/filecoin-project/specs-actors/v2/support/testing"
)

func TestConstruct(t *testing.T) {
	harness := constructStateHarness(t)
	require.Equal(t, abi.NewTokenAmount(0), harness.s.LockedFunds)

	m, err := adt.AsMap(harness.store, harness.s.CoinbaseVestings, builtin.DefaultHamtBitwidth)
	require.NoError(t, err)
	keys, err := m.CollectKeys()
	require.NoError(t, err)
	require.True(t, len(keys) == 0)
}

func TestVestingFunds(t *testing.T) {

	coinbase1 := tutils.NewIDAddr(t, 100)
	coinbase2 := tutils.NewIDAddr(t, 101)

	t.Run("and and withdraw", func(t *testing.T) {
		harness := constructStateHarness(t)

		vfs := harness.loadVestingFunds(coinbase2)
		require.True(t, vfs.UnlockedBalance.IsZero())
		require.True(t, len(vfs.Funds) == 0)

		// add coinbase1
		harness.addLockedFunds(coinbase1, abi.ChainEpoch(100), abi.NewTokenAmount(1))
		harness.addLockedFunds(coinbase1, abi.ChainEpoch(110), abi.NewTokenAmount(100))
		harness.addLockedFunds(coinbase1, abi.ChainEpoch(100), abi.NewTokenAmount(15))

		vfs = harness.loadVestingFunds(coinbase1)

		require.True(t, vfs.UnlockedBalance.IsZero())
		require.True(t, len(vfs.Funds) == 7)

		total := abi.NewTokenAmount(0)
		for _, vf := range vfs.Funds {
			total = big.Add(total, vf.Amount)
		}
		require.True(t, total.Int64() == 116)

		// withdraw coinbase2
		_, amount := harness.withdrawVestedFunds(coinbase2, 100000, abi.NewTokenAmount(100000))
		require.True(t, amount.IsZero())

		// withdraw coinbase1 but immature
		_, amount = harness.withdrawVestedFunds(coinbase1, 24480, abi.NewTokenAmount(100000))
		require.True(t, amount.IsZero())

		// withdraw coinbase1 with first vesting mature
		vfs, amount = harness.withdrawVestedFunds(coinbase1, 24481, abi.NewTokenAmount(100000))
		require.True(t, amount.Int64() == 23)
		require.True(t, vfs.UnlockedBalance.IsZero())
		total = abi.NewTokenAmount(0)
		for _, vf := range vfs.Funds {
			total = big.Add(total, vf.Amount)
		}
		require.True(t, total.Int64() == 93)

		// save vfs
		err := harness.s.SaveVestingFunds(harness.store, coinbase1, vfs)
		require.NoError(t, err)
		vfs = harness.loadVestingFunds(coinbase1)
		require.True(t, vfs.UnlockedBalance.IsZero())
		require.True(t, len(vfs.Funds) == 6)
	})
}

func TestMinerCumulation(t *testing.T) {
	miner1 := tutils.NewIDAddr(t, 100)
	miner2 := tutils.NewIDAddr(t, 101)

	harness := constructStateHarness(t)

	// no cumulation
	cum, err := harness.s.GetMinerCumulation(harness.store, miner1)
	require.NoError(t, err)
	require.True(t, cum.IsZero())

	err = harness.s.AddMinerCumulation(harness.store, miner1, abi.NewTokenAmount(100))
	require.NoError(t, err)
	err = harness.s.AddMinerCumulation(harness.store, miner2, abi.NewTokenAmount(222))
	require.NoError(t, err)
	err = harness.s.AddMinerCumulation(harness.store, miner1, abi.NewTokenAmount(300))
	require.NoError(t, err)

	cum, err = harness.s.GetMinerCumulation(harness.store, miner1)
	require.NoError(t, err)
	require.True(t, cum.Int64() == 400)

	cum, err = harness.s.GetMinerCumulation(harness.store, miner2)
	require.NoError(t, err)
	require.True(t, cum.Int64() == 222)
}

type stateHarness struct {
	t testing.TB

	s     *vesting.State
	store adt.Store
}

func constructStateHarness(t *testing.T) *stateHarness {
	store := ipld.NewADTStore(context.Background())
	state, err := vesting.ConstructState(store)
	require.NoError(t, err)

	return &stateHarness{
		t:     t,
		s:     state,
		store: store,
	}
}

func (h *stateHarness) addLockedFunds(coinbase address.Address, currEpoch abi.ChainEpoch, amount abi.TokenAmount) {
	vfs, _, err := h.s.LoadVestingFunds(h.store, coinbase)
	require.NoError(h.t, err)
	err = h.s.AddLockedFunds(vfs, currEpoch, amount)
	require.NoError(h.t, err)
	err = h.s.SaveVestingFunds(h.store, coinbase, vfs)
	require.NoError(h.t, err)
}

func (h *stateHarness) loadVestingFunds(coinbase address.Address) *vesting.VestingFunds {
	vfs, _, err := h.s.LoadVestingFunds(h.store, coinbase)
	require.NoError(h.t, err)
	return vfs
}

func (h *stateHarness) withdrawVestedFunds(coinbase address.Address, currEpoch abi.ChainEpoch, requestedAmount abi.TokenAmount) (*vesting.VestingFunds, abi.TokenAmount) {
	vfs, _, err := h.s.LoadVestingFunds(h.store, coinbase)
	require.NoError(h.t, err)

	amount, err := h.s.WithdrawVestedFunds(vfs, currEpoch, requestedAmount)
	require.NoError(h.t, err)
	return vfs, amount
}
