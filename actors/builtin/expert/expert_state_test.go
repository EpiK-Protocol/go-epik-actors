package expert_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
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

		pieceID := tutils.MakeCID("1", &miner.SealedCIDPrefix)
		data := newDataOnChainInfo(pieceID)
		harness.putData(data)
		assert.Equal(t, data, harness.getData(pieceID))

		pieceID = tutils.MakeCID("2", &miner.SealedCIDPrefix)
		data = newDataOnChainInfo(pieceID)
		harness.putData(data)
		assert.Equal(t, data, harness.getData(pieceID))

		harness.deleteData(pieceID)
		assert.False(t, harness.hasData(pieceID))
	})

	t.Run("Delete nonexistent value returns an error", func(t *testing.T) {
		harness := constructStateHarness(t)

		pieceID := tutils.MakeCID("1", &miner.SealedCIDPrefix)
		err := harness.s.DeleteData(harness.store, pieceID.String())
		assert.Error(t, err)
	})

	t.Run("Get nonexistent value returns false", func(t *testing.T) {
		harness := constructStateHarness(t)

		pieceID := tutils.MakeCID("1", &miner.SealedCIDPrefix)
		assert.False(t, harness.hasData(pieceID))
	})
}

func TestChangeOwner(t *testing.T) {
	t.Run("save, get", func(t *testing.T) {
		harness := constructStateHarness(t)

		change, err := harness.s.GetOwnerChange(harness.store)
		assert.NoError(t, err)

		err = harness.s.ApplyOwnerChange(harness.store, change.ApplyEpoch, change.ApplyOwner)
		assert.NoError(t, err)

		newChange, err := harness.s.GetOwnerChange(harness.store)
		assert.NoError(t, err)

		assert.Equal(t, change, newChange)
	})

	t.Run("auto update", func(t *testing.T) {
		harness := constructStateHarness(t)

		owner := tutils.NewBLSAddr(t, 2)
		err := harness.s.ApplyOwnerChange(harness.store, abi.ChainEpoch(100), owner)
		assert.NoError(t, err)

		err = harness.s.AutoUpdateOwnerChange(harness.store, abi.ChainEpoch(1000))
		assert.NoError(t, err)

		info, err := harness.s.GetInfo(harness.store)
		assert.NoError(t, err)

		assert.NotEqual(t, owner, info.Owner)

		err = harness.s.AutoUpdateOwnerChange(harness.store, abi.ChainEpoch(1000)+expert.ExpertVoteCheckPeriod)
		assert.NoError(t, err)

		info, err = harness.s.GetInfo(harness.store)
		assert.NoError(t, err)

		assert.Equal(t, owner, info.Owner)

	})
}

func TestValidate(t *testing.T) {
	t.Run("validate", func(t *testing.T) {
		harness := constructStateHarness(t)

		info, err := harness.s.GetInfo(harness.store)
		assert.NoError(t, err)

		err = harness.s.Validate(harness.store, 1)
		assert.NoError(t, err)

		info.Type = expert.ExpertNormal
		err = harness.s.SaveInfo(harness.store, info)
		assert.NoError(t, err)

		err = harness.s.Validate(harness.store, 1)
		assert.Error(t, err)

		harness.s.VoteAmount = expert.ExpertVoteThreshold
		err = harness.s.Validate(harness.store, 1)
		assert.NoError(t, err)

		harness.s.Status = expert.ExpertStateBlocked
		err = harness.s.Validate(harness.store, 1)
		assert.Error(t, err)

		harness.s.Status = expert.ExpertStateImplicated
		err = harness.s.Validate(harness.store, 1)
		assert.Error(t, err)

		harness.s.Status = expert.ExpertStateImplicated
		harness.s.LostEpoch = abi.ChainEpoch(0)
		err = harness.s.Validate(harness.store, 1)
		assert.NoError(t, err)

		harness.s.Status = expert.ExpertStateImplicated
		harness.s.LostEpoch = abi.ChainEpoch(0)
		err = harness.s.Validate(harness.store, expert.ExpertVoteCheckPeriod+1)
		assert.Error(t, err)

		harness.s.Status = expert.ExpertStateBlocked
		err = harness.s.Validate(harness.store, expert.ExpertVoteCheckPeriod+1)
		assert.Error(t, err)
	})
}

type stateHarness struct {
	t testing.TB

	s     *expert.State
	store adt.Store
}

func (h *stateHarness) putData(data *expert.DataOnChainInfo) {
	err := h.s.PutData(h.store, data)
	require.NoError(h.t, err)
}

func (h *stateHarness) getData(pieceID cid.Cid) *expert.DataOnChainInfo {
	data, _, err := h.s.GetData(h.store, pieceID.String())
	require.NoError(h.t, err)
	return data
}

func (h *stateHarness) deleteData(pieceID cid.Cid) {
	err := h.s.DeleteData(h.store, pieceID.String())
	require.NoError(h.t, err)
}

func (h *stateHarness) hasData(pieceID cid.Cid) bool {
	_, found, err := h.s.GetData(h.store, pieceID.String())
	require.NoError(h.t, err)
	return found
}

func constructStateHarness(t *testing.T) *stateHarness {
	// store init
	store := ipld.NewADTStore(context.Background())
	emptyMap, err := adt.MakeEmptyMap(store).Root()
	require.NoError(t, err)

	// state field init
	owner := tutils.NewBLSAddr(t, 1)

	info := &expert.ExpertInfo{
		Owner:           owner,
		Type:            expert.ExpertFoundation,
		ApplicationHash: "aHash",
		Proposer:        owner,
	}
	infoCid, err := store.Put(context.Background(), info)
	require.NoError(t, err)

	eState := expert.ExpertStateRegistered
	if info.Type == expert.ExpertFoundation {
		eState = expert.ExpertStateNormal
	}

	changeCid, err := store.Put(context.Background(), &expert.PendingOwnerChange{
		ApplyOwner: owner,
		ApplyEpoch: abi.ChainEpoch(-1),
	})
	require.NoError(t, err)
	state := expert.ConstructState(infoCid, emptyMap, eState, changeCid)

	return &stateHarness{
		t:     t,
		s:     state,
		store: store,
	}
}

func newDataOnChainInfo(pieceID cid.Cid) *expert.DataOnChainInfo {
	return &expert.DataOnChainInfo{
		PieceID: pieceID.String(),
	}
}
