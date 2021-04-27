package expert_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/filecoin-project/specs-actors/v2/support/ipld"
	tutils "github.com/filecoin-project/specs-actors/v2/support/testing"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfo(t *testing.T) {
	t.Run("save, get info", func(t *testing.T) {
		harness := constructStateHarness(t)

		info, err := harness.s.GetInfo(harness.store)
		assert.NoError(t, err)

		err = harness.s.SaveInfo(harness.store, info)
		assert.NoError(t, err)

		newInfo, err := harness.s.GetInfo(harness.store)
		assert.NoError(t, err)

		assert.Equal(t, info, newInfo)
	})
}

func TestData(t *testing.T) {
	t.Run("Put, get and delete", func(t *testing.T) {
		harness := constructStateHarness(t)

		rootID1 := tutils.MakeCID("1", &miner.SealedCIDPrefix)
		pieceID1 := tutils.MakeCID("1", &market.PieceCIDPrefix)
		data := newDataOnChainInfo(rootID1, pieceID1)
		harness.putDatas(data)
		assert.Equal(t, data, harness.getDatas(true, pieceID1)[0])

		rootID2 := tutils.MakeCID("1", &miner.SealedCIDPrefix)
		pieceID2 := tutils.MakeCID("2", &market.PieceCIDPrefix)
		data = newDataOnChainInfo(rootID2, pieceID2)
		harness.putDatas(data)
		assert.Equal(t, data, harness.getDatas(true, pieceID2)[0])

		harness.deleteData(pieceID2)
		assert.False(t, harness.hasData(pieceID2))
	})

	t.Run("Delete nonexistent value returns an error", func(t *testing.T) {
		harness := constructStateHarness(t)

		pieceID := tutils.MakeCID("1", &miner.SealedCIDPrefix)
		harness.deleteData(pieceID)
	})

	t.Run("Get nonexistent value returns false", func(t *testing.T) {
		harness := constructStateHarness(t)

		pieceID := tutils.MakeCID("1", &miner.SealedCIDPrefix)
		assert.False(t, harness.hasData(pieceID))
	})
}

type stateHarness struct {
	t testing.TB

	s     *expert.State
	store adt.Store
}

func (h *stateHarness) putDatas(infos ...*expert.DataOnChainInfo) {
	err := h.s.PutDatas(h.store, infos...)
	require.NoError(h.t, err)
}

func (h *stateHarness) getDatas(mustPresent bool, pieceIDs ...cid.Cid) []*expert.DataOnChainInfo {
	datas, err := h.s.GetDatas(h.store, mustPresent, pieceIDs...)
	require.NoError(h.t, err)
	return datas
}

func (h *stateHarness) deleteData(pieceID cid.Cid) {
	err := h.s.DeleteData(h.store, pieceID)
	require.NoError(h.t, err)
}

func (h *stateHarness) hasData(pieceID cid.Cid) bool {
	datas, err := h.s.GetDatas(h.store, false, pieceID)
	require.NoError(h.t, err)
	return len(datas) == 1
}

func constructStateHarness(t *testing.T) *stateHarness {
	// store init
	store := ipld.NewADTStore(context.Background())

	// state field init
	owner := tutils.NewBLSAddr(t, 1)

	info := &expert.ExpertInfo{
		Owner:              owner,
		Type:               builtin.ExpertFoundation,
		ApplicationHash:    "aHash",
		Proposer:           owner,
		ApplyNewOwner:      owner,
		ApplyNewOwnerEpoch: -1,
	}
	infoCid, err := store.Put(context.Background(), info)
	require.NoError(t, err)

	eState := expert.ExpertStateRegistered
	if info.Type == builtin.ExpertFoundation {
		eState = expert.ExpertStateQualified
	}

	state, err := expert.ConstructState(store, infoCid, eState)
	require.NoError(t, err)

	return &stateHarness{
		t:     t,
		s:     state,
		store: store,
	}
}

func newDataOnChainInfo(rootID, pieceID cid.Cid) *expert.DataOnChainInfo {
	return &expert.DataOnChainInfo{
		RootID:  rootID,
		PieceID: pieceID,
	}
}
