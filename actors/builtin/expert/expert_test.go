package expert_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/dchest/blake2b"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	builtin "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	expert "github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/miner"
	mock "github.com/filecoin-project/specs-actors/v2/support/mock"
	tutil "github.com/filecoin-project/specs-actors/v2/support/testing"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testPid abi.PeerID
var testMultiaddrs []abi.Multiaddrs

func init() {
	testPid = abi.PeerID("peerID")

	testMultiaddrs = []abi.Multiaddrs{
		{1},
		{2},
	}
}

func getState(rt *mock.Runtime) *expert.State {
	var st expert.State
	rt.GetState(&st)
	return &st
}

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, expert.Actor{})
}

func TestConstruction(t *testing.T) {
	actor := expert.Actor{}

	owner := tutil.NewIDAddr(t, 100)
	receiver := tutil.NewIDAddr(t, 1000)
	builder := mock.NewBuilder(context.Background(), receiver).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(blake2b.Sum256).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("simple construction", func(t *testing.T) {
		rt := builder.Build(t)
		params := expert.ConstructorParams{
			Owner:      owner,
			PeerId:     testPid,
			Multiaddrs: testMultiaddrs,
		}

		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)

		ret := rt.Call(actor.Constructor, &params)

		assert.Nil(t, ret)
		rt.Verify()

		var st expert.State
		rt.GetState(&st)
		assert.Equal(t, params.Owner, st.Info.Owner)
		assert.Equal(t, params.PeerId, st.Info.PeerId)
		assert.Equal(t, params.Multiaddrs, st.Info.Multiaddrs)
	})
}

// Tests for fetching and manipulating expert addresses.
func TestControlAddresses(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	actor := newHarness(t, tutil.NewIDAddr(t, 1000), owner)
	builder := mock.NewBuilder(context.Background(), actor.receiver).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(fixedHasher(0)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("get addresses", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		o := actor.controlAddress(rt)
		assert.Equal(t, owner, o)
	})
}

func TestExpertData(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	actor := newHarness(t, tutil.NewIDAddr(t, 1000), owner)
	builder := mock.NewBuilder(context.Background(), actor.receiver).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(fixedHasher(0)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("import data", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		pieceID := tutil.MakeCID("1", &miner.SealedCIDPrefix)
		actor.importData(rt, newExpertDataParams(pieceID))
	})

	t.Run("check data", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		pieceID := tutil.MakeCID("1", &miner.SealedCIDPrefix)
		actor.importData(rt, newExpertDataParams(pieceID))
		actor.checkData(rt, newExpertDataParams(pieceID))
	})
}

func newExpertDataParams(pieceID cid.Cid) *expert.ExpertDataParams {
	return &expert.ExpertDataParams{
		PieceID: pieceID,
	}
}

type actorHarness struct {
	a expert.Actor
	t testing.TB

	receiver addr.Address // The expert actor's own address
	owner    addr.Address
}

func newHarness(t testing.TB, receiver, owner addr.Address) *actorHarness {
	return &actorHarness{expert.Actor{}, t, receiver, owner}
}

func (h *actorHarness) constructAndVerify(rt *mock.Runtime) {
	params := expert.ConstructorParams{
		Owner:  h.owner,
		PeerId: testPid,
	}

	rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
	ret := rt.Call(h.a.Constructor, &params)
	assert.Nil(h.t, ret)
	rt.Verify()
}

func (h *actorHarness) controlAddress(rt *mock.Runtime) (owner addr.Address) {
	rt.ExpectValidateCallerAny()
	ret := rt.Call(h.a.ControlAddress, nil).(*expert.GetControlAddressReturn)
	require.NotNil(h.t, ret)
	rt.Verify()
	return ret.Owner
}

func (h *actorHarness) importData(rt *mock.Runtime, params *expert.ExpertDataParams) {
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(h.owner)

	rt.Call(h.a.ImportData, params)
	rt.Verify()
}

func (h *actorHarness) checkData(rt *mock.Runtime, params *expert.ExpertDataParams) {
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAny()

	rt.Call(h.a.CheckData, params)
	rt.Verify()
}

// Returns a fake hashing function that always arranges the first 8 bytes of the digest to be the binary
// encoding of a target uint64.
func fixedHasher(target uint64) func([]byte) [32]byte {
	return func(_ []byte) [32]byte {
		var buf bytes.Buffer
		err := binary.Write(&buf, binary.BigEndian, target)
		if err != nil {
			panic(err)
		}
		var digest [32]byte
		copy(digest[:], buf.Bytes())
		return digest
	}
}
