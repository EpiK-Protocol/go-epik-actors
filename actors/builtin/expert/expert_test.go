package expert_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"

	"github.com/dchest/blake2b"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
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
			Owner: owner,
		}

		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)

		ret := rt.Call(actor.Constructor, &params)

		assert.Nil(t, ret)
		rt.Verify()

		var st expert.State
		rt.GetState(&st)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		assert.Equal(t, params.Owner, info.Owner)
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

	t.Run("store data", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		pieceID := tutil.MakeCID("1", &miner.SealedCIDPrefix)
		actor.importData(rt, newExpertDataParams(pieceID))

		minerAddr := tutil.NewIDAddr(t, 101)
		actor.storeData(rt, minerAddr, newExpertDataParams(pieceID))
	})

	t.Run("get data", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		pieceID := tutil.MakeCID("1", &miner.SealedCIDPrefix)
		actor.importData(rt, newExpertDataParams(pieceID))
		actor.GetData(rt, newExpertDataParams(pieceID))
	})
}

func TestNominate(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	actorAddr := tutil.NewIDAddr(t, 1000)
	actor := newHarness(t, actorAddr, owner)
	builder := mock.NewBuilder(context.Background(), actor.receiver).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(fixedHasher(0)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("Nominate", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		actor.Nominate(rt, &expert.NominateExpertParams{
			Expert: actorAddr,
		})
	})
}

func TestBlock(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	actorAddr := tutil.NewIDAddr(t, 1000)
	actor := newHarness(t, actorAddr, owner)
	builder := mock.NewBuilder(context.Background(), actor.receiver).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(fixedHasher(0)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("Block", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		actor.Block(rt)
	})
}

func TestFoundationChange(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	actorAddr := tutil.NewIDAddr(t, 1000)
	actor := newHarness(t, actorAddr, owner)
	builder := mock.NewBuilder(context.Background(), actor.receiver).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(fixedHasher(0)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("FoundationChange", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		actor.FoundationChange(rt, &expert.FoundationChangeParams{
			Owner: tutil.NewIDAddr(t, 101),
		})
	})
}

func TestVote(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	actorAddr := tutil.NewIDAddr(t, 1000)
	actor := newHarness(t, actorAddr, owner)
	builder := mock.NewBuilder(context.Background(), actor.receiver).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(fixedHasher(0)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("Vote", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		actor.Vote(rt, &expert.ExpertVoteParams{
			Amount: abi.NewTokenAmount(1),
		})
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

func (h *actorHarness) storeData(rt *mock.Runtime, miner addr.Address, params *expert.ExpertDataParams) {
	rt.SetCaller(miner, builtin.StorageMinerActorCodeID)
	rt.ExpectValidateCallerType(builtin.StorageMinerActorCodeID)

	rt.Call(h.a.StoreData, params)
	rt.Verify()
}

func (h *actorHarness) GetData(rt *mock.Runtime, params *expert.ExpertDataParams) {
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAny()

	rt.Call(h.a.GetData, params)
	rt.Verify()
}

func (h *actorHarness) Nominate(rt *mock.Runtime, params *expert.NominateExpertParams) {

	{
		param := abi.EmptyValue{}
		rt.ExpectSend(params.Expert, builtin.MethodsExpert.NominateUpdate, &param, big.Zero(), nil, exitcode.Ok)
	}

	{
		cdcParams := builtin.NotifyUpdate{
			Expert:  params.Expert,
			PieceID: cid.Undef,
		}
		rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.NotifyUpdate, &cdcParams, big.Zero(), nil, exitcode.Ok)
	}

	rt.ExpectValidateCallerAddr(h.owner)
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.Call(h.a.Nominate, params)
	rt.Verify()
}

func (h *actorHarness) Block(rt *mock.Runtime) {

	{
		param := abi.EmptyValue{}
		rt.ExpectSend(h.receiver, builtin.MethodsExpert.BlockUpdate, &param, big.Zero(), nil, exitcode.Ok)
	}

	{
		cdcParams := builtin.NotifyUpdate{
			Expert:  h.receiver,
			PieceID: cid.Undef,
		}
		rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.NotifyUpdate, &cdcParams, big.Zero(), nil, exitcode.Ok)
	}

	rt.ExpectValidateCallerAddr(builtin.GovernActorAddr)
	rt.SetCaller(builtin.GovernActorAddr, builtin.GovernActorCodeID)
	param := abi.EmptyValue{}
	rt.Call(h.a.Block, &param)
	rt.Verify()
}

func (h *actorHarness) FoundationChange(rt *mock.Runtime, params *expert.FoundationChangeParams) {

	rt.ExpectValidateCallerAddr(builtin.GovernActorAddr)
	rt.SetCaller(builtin.GovernActorAddr, builtin.GovernActorCodeID)
	rt.Call(h.a.FoundationChange, params)
	rt.Verify()
}

func (h *actorHarness) Vote(rt *mock.Runtime, params *expert.ExpertVoteParams) {

	{
		cdcParams := builtin.NotifyUpdate{
			Expert:  h.receiver,
			PieceID: cid.Undef,
		}
		rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.NotifyUpdate, &cdcParams, big.Zero(), nil, exitcode.Ok)
	}

	rt.ExpectValidateCallerAddr(builtin.VoteFundActorAddr)
	rt.SetCaller(builtin.VoteFundActorAddr, builtin.VoteFundActorCodeID)
	rt.Call(h.a.Vote, params)
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
