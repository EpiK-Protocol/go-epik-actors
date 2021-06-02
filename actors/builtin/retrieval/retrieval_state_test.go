package retrieval_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/retrieval"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/filecoin-project/specs-actors/v2/support/ipld"
	tutils "github.com/filecoin-project/specs-actors/v2/support/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPledge(t *testing.T) {

	t.Run("Pledge", func(t *testing.T) {
		harness := constructStateHarness(t)

		pledger := tutils.NewBLSAddr(t, 1)
		target := tutils.NewBLSAddr(t, 2)
		amount := abi.NewTokenAmount(10000)

		err := harness.s.Pledge(harness.store, pledger, target, amount)
		assert.NoError(t, err, "pledge failed")

	})

	t.Run("Pledge withdraw", func(t *testing.T) {
		harness := constructStateHarness(t)

		pledger := tutils.NewBLSAddr(t, 1)
		target := tutils.NewBLSAddr(t, 2)
		amount := abi.NewTokenAmount(10000)

		err := harness.s.Pledge(harness.store, pledger, target, amount)
		assert.NoError(t, err, "pledge failed")

		code, err := harness.s.ApplyForWithdraw(harness.store, 1, pledger, target, amount)
		assert.NoError(t, err, "pledge apply withdraw failed")
		require.True(t, code == exitcode.Ok)

	})

	t.Run("Pledge withdraw out of amount", func(t *testing.T) {
		harness := constructStateHarness(t)

		pledger := tutils.NewBLSAddr(t, 1)
		target := tutils.NewBLSAddr(t, 2)
		amount := abi.NewTokenAmount(10000)

		err := harness.s.Pledge(harness.store, pledger, target, amount)
		assert.NoError(t, err, "pledge failed")

		code, err := harness.s.ApplyForWithdraw(harness.store, 1, pledger, target, big.Add(amount, amount))
		assert.Error(t, err, "pledge apply withdraw failed")
		require.True(t, code == exitcode.ErrIllegalState)

	})

	t.Run("Pledge withdraw", func(t *testing.T) {
		harness := constructStateHarness(t)

		pledger := tutils.NewBLSAddr(t, 1)
		target1 := tutils.NewBLSAddr(t, 2)
		target2 := tutils.NewBLSAddr(t, 2)
		amount1 := abi.NewTokenAmount(10000)
		amount2 := abi.NewTokenAmount(10000)

		err := harness.s.Pledge(harness.store, pledger, target1, amount1)
		assert.NoError(t, err, "pledge failed")
		err = harness.s.Pledge(harness.store, pledger, target2, amount2)
		assert.NoError(t, err, "pledge failed")

		code, err := harness.s.ApplyForWithdraw(harness.store, 1, pledger, target2, amount2)
		assert.NoError(t, err, "pledge apply withdraw failed")
		require.True(t, code == exitcode.Ok)
	})
}

func constructStateHarness(t *testing.T) *stateHarness {
	// store init
	store := ipld.NewADTStore(context.Background())

	state, err := retrieval.ConstructState(store)
	require.NoError(t, err)

	return &stateHarness{
		t:     t,
		s:     state,
		store: store,
	}
}

type stateHarness struct {
	t testing.TB

	s     *retrieval.State
	store adt.Store
}
