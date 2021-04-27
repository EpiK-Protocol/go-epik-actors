package expertfund_test

import (
	"context"
	"testing"

	abi "github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expertfund"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/filecoin-project/specs-actors/v2/support/ipld"
	tutil "github.com/filecoin-project/specs-actors/v2/support/testing"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func TestDeposit(t *testing.T) {
	t.Run("deposit", func(t *testing.T) {
		// harness := constructStateHarness(t)

		// harness.s.Deposit()
	})
}

func TestPutAndGetPool(t *testing.T) {
	t.Run("put/get pool", func(t *testing.T) {
		harness := constructStateHarness(t)

		// initial pool
		pool, err := harness.s.GetPool(harness.store)
		require.NoError(t, err)
		require.True(t, pool.AccPerShare.IsZero())
		require.True(t, pool.CurrentTotalDataSize == 0)
		require.True(t, pool.LastRewardBalance.IsZero())

		// save new pool
		newPool := &expertfund.PoolInfo{
			AccPerShare:          abi.NewTokenAmount(100),
			CurrentTotalDataSize: 100,
			LastRewardBalance:    abi.NewTokenAmount(200),
		}
		err = harness.s.SavePool(harness.store, newPool)
		require.NoError(t, err)

		pool, err = harness.s.GetPool(harness.store)
		require.NoError(t, err)
		require.True(t, pool.AccPerShare.Equals(big.NewInt(100)))
		require.True(t, pool.CurrentTotalDataSize == 100)
		require.True(t, pool.LastRewardBalance.Equals(big.NewInt(200)))
	})
}

func TestPutAndGetDataInfos(t *testing.T) {
	expert1 := tutil.NewIDAddr(t, 100)
	expert2 := tutil.NewIDAddr(t, 101)

	t.Run("get", func(t *testing.T) {
		harness := constructStateHarness(t)

		dbp, err := adt.AsMap(harness.store, harness.s.PieceInfos, builtin.DefaultHamtBitwidth)
		require.NoError(t, err)

		keys, err := dbp.CollectKeys()
		require.NoError(t, err)
		require.True(t, len(keys) == 0)

		// get nil
		m, _, err := harness.s.GetPieceInfos(harness.store)
		require.NoError(t, err)
		require.True(t, len(m) == 0)

		piece1 := tutil.MakeCID("rand1", &market.PieceCIDPrefix)

		// get not exist
		m, _, err = harness.s.GetPieceInfos(harness.store, piece1)
		require.Contains(t, err.Error(), "piece not found")
	})

	t.Run("put absent", func(t *testing.T) {
		harness := constructStateHarness(t)

		piece1 := tutil.MakeCID("rand1", &market.PieceCIDPrefix)

		// put
		err := harness.s.PutPieceInfos(harness.store, true, map[cid.Cid]*expertfund.PieceInfo{
			piece1: {expert1, expertfund.DefaultDataStoreThreshold},
		})
		require.NoError(t, err)

		m, _, err := harness.s.GetPieceInfos(harness.store, piece1)
		require.NoError(t, err)
		require.True(t, m[piece1] == expert1)

		// re-put
		err = harness.s.PutPieceInfos(harness.store, true, map[cid.Cid]*expertfund.PieceInfo{
			piece1: {expert1, expertfund.DefaultDataStoreThreshold},
		})
		require.Contains(t, err.Error(), "already exists")
	})

	t.Run("put not absent", func(t *testing.T) {
		harness := constructStateHarness(t)

		piece1 := tutil.MakeCID("rand1", &market.PieceCIDPrefix)
		piece2 := tutil.MakeCID("rand2", &market.PieceCIDPrefix)

		err := harness.s.PutPieceInfos(harness.store, false, map[cid.Cid]*expertfund.PieceInfo{
			piece1: {expert1, expertfund.DefaultDataStoreThreshold},
			piece2: {expert2, expertfund.DefaultDataStoreThreshold},
		})
		require.NoError(t, err)

		// override
		err = harness.s.PutPieceInfos(harness.store, false, map[cid.Cid]*expertfund.PieceInfo{
			piece1: {expert2, expertfund.DefaultDataStoreThreshold},
		})
		require.NoError(t, err)

		m, _, err := harness.s.GetPieceInfos(harness.store, piece1, piece2)
		require.NoError(t, err)
		require.True(t, m[piece1] == expert2 && m[piece2] == expert2)
	})
}

// func TestPutAndGetExperts(t *testing.T) {
// 	expert1 := tutil.NewIDAddr(t, 100)
// 	expert2 := tutil.NewIDAddr(t, 101)

// 	t.Run("get initial", func(t *testing.T) {
// 		harness := constructStateHarness(t)
// 	})
// }

func TestDisqualifiedExperts(t *testing.T) {
	expert1 := tutil.NewIDAddr(t, 100)
	expert2 := tutil.NewIDAddr(t, 101)

	t.Run("get initial", func(t *testing.T) {
		harness := constructStateHarness(t)

		tracked, err := adt.AsMap(harness.store, harness.s.DisqualifiedExperts, builtin.DefaultHamtBitwidth)
		require.NoError(t, err)
		keys, err := tracked.CollectKeys()
		require.NoError(t, err)
		require.True(t, len(keys) == 0)

		expertAddrs, err := harness.s.ListDisqualifiedExperts(harness.store)
		require.NoError(t, err)
		require.True(t, len(expertAddrs) == 0)
	})

	t.Run("add", func(t *testing.T) {
		harness := constructStateHarness(t)

		// add expert1, expert2
		err := harness.s.PutDisqualifiedExpertIfAbsent(harness.store, expert1, &expertfund.DisqualifiedExpertInfo{100})
		require.NoError(t, err)
		err = harness.s.PutDisqualifiedExpertIfAbsent(harness.store, expert2, &expertfund.DisqualifiedExpertInfo{200})
		require.NoError(t, err)
		expertEpochs, err := harness.s.ListDisqualifiedExperts(harness.store)
		require.NoError(t, err)
		require.True(t, len(expertEpochs) == 2 && expertEpochs[expert1] == 100 && expertEpochs[expert2] == 200)

		// re-add expert1
		err = harness.s.PutDisqualifiedExpertIfAbsent(harness.store, expert1, &expertfund.DisqualifiedExpertInfo{110})
		require.NoError(t, err)
		expertEpochs, err = harness.s.ListDisqualifiedExperts(harness.store)
		require.NoError(t, err)
		require.True(t, len(expertEpochs) == 2 && expertEpochs[expert1] == 100 && expertEpochs[expert2] == 200)
	})

	t.Run("delete", func(t *testing.T) {
		harness := constructStateHarness(t)

		err := harness.s.PutDisqualifiedExpertIfAbsent(harness.store, expert1, &expertfund.DisqualifiedExpertInfo{100})
		require.NoError(t, err)
		err = harness.s.PutDisqualifiedExpertIfAbsent(harness.store, expert2, &expertfund.DisqualifiedExpertInfo{200})
		require.NoError(t, err)

		// delete expert3(not added)
		expert3 := tutil.NewIDAddr(t, 200)
		err = harness.s.DeleteDisqualifiedExpertInfo(harness.store, expert3)
		require.NoError(t, err)
		expertEpochs, err := harness.s.ListDisqualifiedExperts(harness.store)
		require.NoError(t, err)
		require.True(t, len(expertEpochs) == 2 && expertEpochs[expert1] == 100 && expertEpochs[expert2] == 200)

		// delete expert2(added)
		err = harness.s.DeleteDisqualifiedExpertInfo(harness.store, expert2)
		require.NoError(t, err)
		expertEpochs, err = harness.s.ListDisqualifiedExperts(harness.store)
		require.NoError(t, err)
		require.True(t, len(expertEpochs) == 1 && expertEpochs[expert1] == 100)
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
		CurrentTotalDataSize: 0,
		AccPerShare:          abi.NewTokenAmount(0),
		LastRewardBalance:    abi.NewTokenAmount(0),
	}
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
