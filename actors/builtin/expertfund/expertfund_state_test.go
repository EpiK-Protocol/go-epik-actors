package expertfund_test

import (
	"context"
	"testing"

	abi "github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expertfund"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/filecoin-project/specs-actors/v2/support/ipld"
	"github.com/stretchr/testify/require"
)

func TestDeposit(t *testing.T) {
	t.Run("deposit", func(t *testing.T) {
		// harness := constructStateHarness(t)

		// harness.s.Deposit()
	})
}

type stateHarness struct {
	t testing.TB

	s     *expertfund.State
	store adt.Store
}

func constructStateHarness(t *testing.T) *stateHarness {
	// store init
	store := ipld.NewADTStore(context.Background())

	info := &expertfund.PoolInfo{
		LastRewardBlock: abi.ChainEpoch(0),
		AccPerShare:     abi.NewTokenAmount(0)}
	infoCid, err := store.Put(context.Background(), info)
	require.NoError(t, err)

	state, err := expertfund.ConstructState(store, infoCid)
	require.NoError(t, err)
	return &stateHarness{
		t:     t,
		s:     state,
		store: store,
	}
}
