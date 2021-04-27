package expert_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/rand"
	"strconv"
	"testing"

	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"

	"github.com/dchest/blake2b"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	builtin "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	expert "github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/miner"
	mock "github.com/filecoin-project/specs-actors/v2/support/mock"
	tutil "github.com/filecoin-project/specs-actors/v2/support/testing"
	"github.com/stretchr/testify/require"
	cbg "github.com/whyrusleeping/cbor-gen"
)

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
	applicant := tutil.NewIDAddr(t, 101)
	receiver := tutil.NewIDAddr(t, 1000)
	builder := mock.NewBuilder(context.Background(), receiver).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(blake2b.Sum256).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("construct foundation", func(t *testing.T) {
		rt := builder.Build(t)
		params := expert.ConstructorParams{
			Owner:    owner,
			Proposer: owner,
			Type:     builtin.ExpertFoundation,
		}

		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)

		ret := rt.Call(actor.Constructor, &params)
		require.Nil(t, ret)
		rt.Verify()

		st := getState(rt)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.Equal(t, params.Owner, info.Owner)
		require.Equal(t, builtin.ExpertFoundation, info.Type)
	})

	t.Run("construct normal with 99 EPK", func(t *testing.T) {
		rt := builder.Build(t)
		rt.SetBalance(expert.ExpertApplyCost)
		rt.SetReceived(expert.ExpertApplyCost)
		params := expert.ConstructorParams{
			Owner:    owner,
			Proposer: owner,
			Type:     builtin.ExpertNormal,
		}

		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
		rt.ExpectSend(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, expert.ExpertApplyCost, nil, exitcode.Ok)
		rt.Call(actor.Constructor, &params)
		rt.Verify()

		require.True(t, rt.Balance().Sign() == 0)

		st := getState(rt)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.Equal(t, params.Owner, info.Owner)
		require.Equal(t, builtin.ExpertNormal, info.Type)
	})

	t.Run("should fail with not enough funds", func(t *testing.T) {
		rt := builder.Build(t)
		amt := big.Sub(expert.ExpertApplyCost, big.NewInt(1))
		rt.SetBalance(expert.ExpertApplyCost)
		rt.SetReceived(amt)
		params := expert.ConstructorParams{
			Owner:    owner,
			Proposer: owner,
			Type:     builtin.ExpertNormal,
		}

		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "fund for expert proposal not enough", func() {
			rt.Call(actor.Constructor, &params)
		})
	})

	t.Run("should fail with more funds", func(t *testing.T) {
		rt := builder.Build(t)
		amt := big.Add(expert.ExpertApplyCost, big.NewInt(1))
		rt.SetBalance(amt)
		rt.SetReceived(amt)
		params := expert.ConstructorParams{
			Owner:    owner,
			Proposer: applicant,
			Type:     builtin.ExpertNormal,
		}

		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
		rt.ExpectSend(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, expert.ExpertApplyCost, nil, exitcode.Ok)
		rt.ExpectSend(applicant, builtin.MethodSend, nil, big.NewInt(1), nil, exitcode.Ok)
		rt.Call(actor.Constructor, &params)
		rt.Verify()

		require.True(t, rt.Balance().Sign() == 0)

		st := getState(rt)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.Equal(t, builtin.ExpertNormal, info.Type)
	})
}

// Tests for fetching and manipulating expert addresses.
func TestControlAddresses(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	actorProposer := tutil.NewIDAddr(t, 1000)
	actorAddr := tutil.NewIDAddr(t, 1001)

	actor := newHarness(t, actorAddr, owner, actorProposer)
	builder := mock.NewBuilder(context.Background(), actorAddr).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(fixedHasher(0)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("get addresses", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertNormal)

		o := actor.controlAddress(rt)
		require.Equal(t, owner, o)
	})
}

func TestImportData(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	actorProposer := tutil.NewIDAddr(t, 1000)
	actorAddr := tutil.NewIDAddr(t, 1001)

	actor := newHarness(t, actorAddr, owner, actorProposer)
	builder := mock.NewBuilder(context.Background(), actorAddr).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(fixedHasher(0)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("fail when unqualified", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertNormal)

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "unqualified expert "+actorAddr.String(), func() {
			rt.Call(actor.ImportData, newImportDataParams())
		})
	})

	t.Run("ok", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		// first
		params := newImportDataParams()
		checkedID := builtin.CheckedCID{CID: params.PieceID}

		rt.SetCaller(owner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(owner)
		rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.OnExpertImport, &checkedID, abi.NewTokenAmount(0), nil, exitcode.Ok)
		rt.Call(actor.ImportData, params)
		rt.Verify()

		st := getState(rt)
		ret := actor.getDatas(rt, &builtin.BatchPieceCIDParams{PieceCIDs: []builtin.CheckedCID{checkedID}})
		require.True(t, st.DataCount == 1 && len(ret.Infos) == 1 &&
			ret.Infos[0].PieceID == params.PieceID &&
			ret.Infos[0].RootID == params.RootID &&
			ret.Infos[0].PieceSize == params.PieceSize)

		// second
		params2 := newImportDataParams()
		checkedID2 := builtin.CheckedCID{CID: params2.PieceID}

		actor.importData(rt, params2)
		st = getState(rt)
		ret = actor.getDatas(rt, &builtin.BatchPieceCIDParams{PieceCIDs: []builtin.CheckedCID{checkedID, checkedID2}})
		require.True(t, st.DataCount == 2 && len(ret.Infos) == 2 &&
			ret.Infos[1].PieceID == params2.PieceID &&
			ret.Infos[1].RootID == params2.RootID &&
			ret.Infos[1].PieceSize == params2.PieceSize)
	})

	t.Run("fail when import duplicate data", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		params := newImportDataParams()
		actor.importData(rt, params)

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "duplicate data "+params.PieceID.String(), func() {
			actor.importData(rt, params)
		})
	})
}

func TestStoreData(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	actorProposer := tutil.NewIDAddr(t, 1000)
	actorAddr := tutil.NewIDAddr(t, 1001)

	actor := newHarness(t, actorAddr, owner, actorProposer)
	builder := mock.NewBuilder(context.Background(), actorAddr).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(fixedHasher(0)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("fail when data not imported", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "data not found", func() {
			rt.Call(actor.StoreData, newStoreDataParams(newImportDataParams()))
		})

		// piece 1 imported but piece 2 not
		params1 := newImportDataParams()
		params2 := newImportDataParams()
		actor.importData(rt, params1)

		rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "data not found "+params2.PieceID.String(), func() {
			rt.Call(actor.StoreData, newStoreDataParams(params1, params2))
		})
	})

	t.Run("ok", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		params1 := newImportDataParams()
		actor.importData(rt, params1)

		// first time
		ret := actor.storeData(rt, newStoreDataParams(params1))
		require.True(t, ret.Infos[0].Redundancy == 1 &&
			ret.Infos[0].PieceID == params1.PieceID &&
			ret.Infos[0].RootID == params1.RootID &&
			ret.Infos[0].PieceSize == params1.PieceSize)

		// second time
		params2 := newImportDataParams()
		actor.importData(rt, params2)
		ret = actor.storeData(rt, newStoreDataParams(params1, params2))
		require.True(t, ret.Infos[0].Redundancy == 2 && ret.Infos[1].Redundancy == 1 &&
			ret.Infos[1].PieceID == params2.PieceID &&
			ret.Infos[1].RootID == params2.RootID &&
			ret.Infos[1].PieceSize == params2.PieceSize)
	})
}

func TestNominate(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	newowner := tutil.NewIDAddr(t, 101)
	governor := tutil.NewIDAddr(t, 102)
	actorProposer := tutil.NewIDAddr(t, 1000)
	actorAddr := tutil.NewIDAddr(t, 1001)
	nominatedAddr := tutil.NewIDAddr(t, 1002)

	actor := newHarness(t, actorAddr, owner, actorProposer)
	builder := mock.NewBuilder(context.Background(), actor.receiver).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(fixedHasher(0)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("nominate without owner change(gov)", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertNormal)

		// nominator is unqualified
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "nominator is unqualified", func() {
			rt.Call(actor.Nominate, &nominatedAddr)
		})

		// caller not owner
		st := getState(rt)
		st.ExpertState = expert.ExpertStateQualified
		rt.ReplaceState(st)

		caller := tutil.NewIDAddr(t, 200)
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(owner)
		rt.SetAddressActorType(nominatedAddr, builtin.ExpertActorCodeID)
		rt.ExpectAbort(exitcode.SysErrForbidden, func() {
			rt.Call(actor.Nominate, &nominatedAddr)
		})

		// normal
		rt.SetCaller(owner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(owner)
		rt.SetAddressActorType(nominatedAddr, builtin.ExpertActorCodeID)
		rt.ExpectSend(nominatedAddr, builtin.MethodsExpert.OnNominated, nil, abi.NewTokenAmount(0), &builtin.Discard{}, exitcode.Ok)
		rt.Call(actor.Nominate, &nominatedAddr)
	})

	t.Run("nominate with owner change(gov)", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		rt.SetEpoch(100)
		actor.govChangeOwner(rt, governor, newowner)

		rt.SetEpoch(100 + expert.ActivateNewOwnerDelay)

		rt.SetCaller(newowner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(newowner)
		rt.ExpectSend(nominatedAddr, builtin.MethodsExpert.OnNominated, nil, abi.NewTokenAmount(0), &builtin.Discard{}, exitcode.Ok)
		rt.Call(actor.Nominate, &nominatedAddr)

		st := getState(rt)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, info.ApplyNewOwnerEpoch == -1)
	})
}

func TestOnNominated(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	expertAddr := tutil.NewIDAddr(t, 1000)
	nominatedAddr := tutil.NewIDAddr(t, 1001)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), nominatedAddr).
			WithActorType(owner, builtin.AccountActorCodeID).
			WithHasher(fixedHasher(0)).
			WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

		rt := builder.Build(t)
		actor := newHarness(t, nominatedAddr, owner, expertAddr)
		actor.constructAndVerify(rt, builtin.ExpertNormal)
		return rt, actor
	}

	t.Run("validate caller failed", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetCaller(owner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
		rt.ExpectAbort(exitcode.SysErrForbidden, func() {
			rt.Call(actor.OnNominated, nil)
		})
	})

	t.Run("foundation expert cannot be nominated", func(t *testing.T) {
		builder := mock.NewBuilder(context.Background(), nominatedAddr).
			WithActorType(owner, builtin.AccountActorCodeID).
			WithHasher(fixedHasher(0)).
			WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

		rt := builder.Build(t)
		actor := newHarness(t, nominatedAddr, owner, expertAddr)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		rt.SetCaller(expertAddr, builtin.ExpertActorCodeID)
		rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "foundation expert cannot be nominated", func() {
			rt.Call(actor.OnNominated, nil)
		})
	})

	t.Run("nominate expert with unexpected status", func(t *testing.T) {
		rt, actor := setupFunc()

		for _, state := range []expert.ExpertState{expert.ExpertStateBlocked, expert.ExpertStateQualified, expert.ExpertStateUnqualified} {
			st := getState(rt)
			st.ExpertState = state
			rt.ReplaceState(st)

			rt.SetCaller(expertAddr, builtin.ExpertActorCodeID)
			rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
			rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "nominate expert with invalid status", func() {
				rt.Call(actor.OnNominated, nil)
			})
		}
	})

	t.Run("ok", func(t *testing.T) {
		rt, actor := setupFunc()
		st := getState(rt)
		st.ExpertState = expert.ExpertStateRegistered
		rt.ReplaceState(st)

		rt.SetCaller(expertAddr, builtin.ExpertActorCodeID)
		rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
		rt.Call(actor.OnNominated, nil)
		rt.Verify()

		st = getState(rt)
		require.True(t, st.ExpertState == expert.ExpertStateUnqualified)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, info.Proposer == expertAddr)
	})
}

func TestOnBlocked(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	proposer := tutil.NewIDAddr(t, 1000)
	expertAddr := tutil.NewIDAddr(t, 1001)
	nominatedAddr := tutil.NewIDAddr(t, 1002)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), expertAddr).
			WithActorType(owner, builtin.AccountActorCodeID).
			WithActorType(proposer, builtin.ExpertActorCodeID).
			WithHasher(fixedHasher(0)).
			WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

		rt := builder.Build(t)
		actor := newHarness(t, expertAddr, owner, proposer)
		actor.constructAndVerify(rt, builtin.ExpertNormal)

		return rt, actor
	}

	t.Run("foundation expert cannot be blocked", func(t *testing.T) {
		builder := mock.NewBuilder(context.Background(), expertAddr).
			WithActorType(owner, builtin.AccountActorCodeID).
			WithHasher(fixedHasher(0)).
			WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

		rt := builder.Build(t)
		actor := newHarness(t, expertAddr, owner, proposer)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "foundation expert cannot be blocked", func() {
			actor.onBlocked(rt, proposer, true)
		})
	})

	t.Run("block expert with unexpected status", func(t *testing.T) {
		rt, actor := setupFunc()

		st := getState(rt)
		st.ExpertState = expert.ExpertStateBlocked
		rt.ReplaceState(st)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "try to block expert with invalid state", func() {
			actor.onBlocked(rt, proposer, true)
		})

		st = getState(rt)
		st.ExpertState = expert.ExpertStateRegistered
		rt.ReplaceState(st)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "try to block expert with invalid state", func() {
			actor.onBlocked(rt, proposer, true)
		})
	})

	t.Run("ok", func(t *testing.T) {
		rt, actor := setupFunc()

		st := getState(rt)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		info.Proposer = proposer
		err = st.SaveInfo(adt.AsStore(rt), info)
		require.NoError(t, err)
		st.ExpertState = expert.ExpertStateQualified
		st.CurrentVotes = abi.NewTokenAmount(1000)
		rt.ReplaceState(st)

		// nominate will success
		rt.SetAddressActorType(nominatedAddr, builtin.ExpertActorCodeID)
		actor.nominate(rt, nominatedAddr)

		rt.SetEpoch(101)
		ret := actor.onBlocked(rt, proposer, true)
		require.True(t, ret.ImplicatedExpert == proposer && ret.ImplicatedExpertVotesEnough)
		st = getState(rt)
		require.True(t, st.ExpertState == expert.ExpertStateBlocked && st.CurrentVotes.IsZero())

		// try nominate will failed
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "nominator is unqualified", func() {
			rt.Call(actor.Nominate, &nominatedAddr)
		})
	})
}

func TestOnImplicated(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	proposerOfProposer := tutil.NewIDAddr(t, 1000)
	proposer := tutil.NewIDAddr(t, 1001)
	nominatedAddr := tutil.NewIDAddr(t, 1002)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), proposer).
			WithActorType(owner, builtin.AccountActorCodeID).
			WithHasher(fixedHasher(0)).
			WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

		rt := builder.Build(t)
		actor := newHarness(t, proposer, owner, proposerOfProposer)
		actor.constructAndVerify(rt, builtin.ExpertNormal)
		return rt, actor
	}

	t.Run("no effect on fundation expert", func(t *testing.T) {
		rt, actor := setupFunc()

		st := getState(rt)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		info.Type = builtin.ExpertFoundation
		err = st.SaveInfo(adt.AsStore(rt), info)
		require.NoError(t, err)
		rt.ReplaceState(st)

		votesEnough := actor.onImplicated(rt, nominatedAddr)
		st = getState(rt)
		require.True(t, st.ImplicatedTimes == 0 && votesEnough)
	})

	t.Run("no effect on already blocked expert", func(t *testing.T) {
		rt, actor := setupFunc()

		st := getState(rt)
		st.ExpertState = expert.ExpertStateBlocked
		rt.ReplaceState(st)

		votesEnough := actor.onImplicated(rt, nominatedAddr)
		st = getState(rt)
		require.True(t, st.ImplicatedTimes == 0 && votesEnough)
	})

	t.Run("ok", func(t *testing.T) {
		rt, actor := setupFunc()

		st := getState(rt)
		st.ExpertState = expert.ExpertStateQualified
		st.CurrentVotes = big.Sub(big.Mul(big.NewInt(150000), builtin.TokenPrecision), big.NewInt(1))
		rt.ReplaceState(st)

		// first implication
		votesEnough := actor.onImplicated(rt, nominatedAddr)
		st = getState(rt)
		require.True(t, st.ImplicatedTimes == 1 &&
			st.ExpertState == expert.ExpertStateQualified &&
			votesEnough == true &&
			st.VoteThreshold().Equals(big.Mul(big.NewInt(125000), builtin.TokenPrecision)))

		// second implication
		votesEnough = actor.onImplicated(rt, nominatedAddr)
		st = getState(rt)
		require.True(t, st.ImplicatedTimes == 2 &&
			st.ExpertState == expert.ExpertStateUnqualified &&
			votesEnough == false &&
			st.VoteThreshold().Equals(big.Mul(big.NewInt(150000), builtin.TokenPrecision)))
	})
}

func TestChangeOwner(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	newowner := tutil.NewIDAddr(t, 101)
	govnewowner := tutil.NewIDAddr(t, 102)
	governor := tutil.NewIDAddr(t, 103)
	proposer := tutil.NewIDAddr(t, 1000)
	expertAddr := tutil.NewIDAddr(t, 1001)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), expertAddr).
			WithActorType(owner, builtin.AccountActorCodeID).
			WithActorType(newowner, builtin.MultisigActorCodeID).
			WithHasher(fixedHasher(0)).
			WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

		rt := builder.Build(t)
		actor := newHarness(t, expertAddr, owner, proposer)
		actor.constructAndVerify(rt, builtin.ExpertNormal)
		return rt, actor
	}

	t.Run("fail calling with old owner when gov change effect", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetEpoch(100)
		actor.govChangeOwner(rt, governor, govnewowner)

		st := getState(rt)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, owner == info.Owner && govnewowner == info.ApplyNewOwner && 100 == info.ApplyNewOwnerEpoch)

		rt.SetEpoch(100 + expert.ActivateNewOwnerDelay)
		rt.SetCaller(owner, builtin.MultisigActorCodeID)
		rt.ExpectValidateCallerAddr(govnewowner)
		rt.ExpectAbort(exitcode.SysErrForbidden, func() {
			rt.Call(actor.ChangeOwner, &newowner)
		})
	})

	t.Run("success calling with old owner before gov change effect", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetEpoch(100)
		actor.govChangeOwner(rt, governor, govnewowner)

		st := getState(rt)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, owner == info.Owner && govnewowner == info.ApplyNewOwner && 100 == info.ApplyNewOwnerEpoch)

		rt.SetEpoch(100 + expert.ActivateNewOwnerDelay - 1)
		rt.SetCaller(owner, builtin.MultisigActorCodeID)
		rt.ExpectValidateCallerAddr(owner)
		rt.Call(actor.ChangeOwner, &newowner)
		rt.Verify()

		st = getState(rt)
		info, err = st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, newowner == info.Owner && govnewowner == info.ApplyNewOwner && -1 == info.ApplyNewOwnerEpoch)
	})

	t.Run("success calling with new owner when gov change effect", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetEpoch(100)
		actor.govChangeOwner(rt, governor, govnewowner)

		st := getState(rt)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, owner == info.Owner && govnewowner == info.ApplyNewOwner && 100 == info.ApplyNewOwnerEpoch)

		rt.SetEpoch(100 + expert.ActivateNewOwnerDelay)
		rt.SetCaller(govnewowner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(govnewowner)
		rt.Call(actor.ChangeOwner, &newowner)
		rt.Verify()

		st = getState(rt)
		info, err = st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, newowner == info.Owner && govnewowner == info.ApplyNewOwner && -1 == info.ApplyNewOwnerEpoch)
	})
}

func TestGovChangeOwner(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	governor := tutil.NewIDAddr(t, 101)
	proposer := tutil.NewIDAddr(t, 1000)
	expertAddr := tutil.NewIDAddr(t, 1001)

	setupFunc := func(governor addr.Address) (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), expertAddr).
			WithActorType(owner, builtin.AccountActorCodeID).
			WithHasher(fixedHasher(0)).
			WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

		rt := builder.Build(t)
		actor := newHarness(t, expertAddr, owner, proposer)
		actor.constructAndVerify(rt, builtin.ExpertNormal)

		rt.SetCaller(governor, builtin.MultisigActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(builtin.GovernActorAddr, builtin.MethodsGovern.ValidateGranted, &builtin.ValidateGrantedParams{
			Caller: governor,
			Method: builtin.MethodsExpert.GovChangeOwner,
		}, big.Zero(), nil, exitcode.Ok)
		return rt, actor
	}

	t.Run("fail when empty address", func(t *testing.T) {
		rt, actor := setupFunc(governor)

		rt.SetCaller(governor, builtin.MultisigActorCodeID)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "empty address", func() {
			rt.Call(actor.GovChangeOwner, &addr.Undef)
		})
	})

	t.Run("fail when non-ID address", func(t *testing.T) {
		rt, actor := setupFunc(governor)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "owner address must be an ID address", func() {
			newowner := tutil.NewBLSAddr(t, 1)
			rt.Call(actor.GovChangeOwner, &newowner)
		})
	})

	t.Run("normal", func(t *testing.T) {
		rt, actor := setupFunc(governor)
		newowner := tutil.NewIDAddr(t, 200)

		rt.SetEpoch(100)
		rt.Call(actor.GovChangeOwner, &newowner)
		rt.Verify()

		st := getState(rt)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, owner == info.Owner && newowner == info.ApplyNewOwner && 100 == info.ApplyNewOwnerEpoch)

		// at ActivateNewOwnerDelay-1
		rt.SetEpoch(100 + expert.ActivateNewOwnerDelay - 1)
		actor.controlAddress(rt)
		st = getState(rt)
		info, err = st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, owner == info.Owner && newowner == info.ApplyNewOwner && 100 == info.ApplyNewOwnerEpoch)

		// at ActivateNewOwnerDelay
		rt.SetEpoch(100 + expert.ActivateNewOwnerDelay)
		actor.controlAddress(rt)
		st = getState(rt)
		info, err = st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, newowner == info.Owner && newowner == info.ApplyNewOwner && -1 == info.ApplyNewOwnerEpoch)
	})
}

func TestCheckState(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	actorProposer := tutil.NewIDAddr(t, 1000)
	actorAddr := tutil.NewIDAddr(t, 1001)

	actor := newHarness(t, actorAddr, owner, actorProposer)
	builder := mock.NewBuilder(context.Background(), actorAddr).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithHasher(fixedHasher(0)).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	testcases := []struct {
		name            string
		status          expert.ExpertState
		expectAllowVote bool
		expectQualified bool
	}{
		{"ExpertStateRegistered", expert.ExpertStateRegistered, false, false},
		{"ExpertStateQualified", expert.ExpertStateQualified, true, true},
		{"ExpertStateUnqualified", expert.ExpertStateUnqualified, true, false},
		{"ExpertStateBlocked", expert.ExpertStateBlocked, false, false},
	}

	for _, ts := range testcases {
		t.Run(ts.name, func(t *testing.T) {
			rt := builder.Build(t)
			actor.constructAndVerify(rt, builtin.ExpertNormal)

			st := getState(rt)
			st.ExpertState = ts.status
			rt.ReplaceState(st)

			rt.SetCaller(owner, builtin.AccountActorCodeID)
			rt.ExpectValidateCallerAny()
			ret := rt.Call(actor.CheckState, nil).(*builtin.CheckExpertStateReturn)
			rt.Verify()
			require.True(t, ret.AllowVote == ts.expectAllowVote && ret.Qualified == ts.expectQualified)
		})
	}
}

func TestOnVotesUpdated(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	actorProposer := tutil.NewIDAddr(t, 1000)
	actorAddr := tutil.NewIDAddr(t, 1001)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		actor := newHarness(t, actorAddr, owner, actorProposer)
		builder := mock.NewBuilder(context.Background(), actorAddr).
			WithActorType(owner, builtin.AccountActorCodeID).
			WithHasher(fixedHasher(0)).
			WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertNormal)
		return rt, actor
	}

	t.Run("foundation", func(t *testing.T) {
		rt, actor := setupFunc()

		st := getState(rt)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		info.Type = builtin.ExpertFoundation
		err = st.SaveInfo(adt.AsStore(rt), info)
		require.NoError(t, err)
		rt.ReplaceState(st)

		ret := actor.onVotesUpdated(rt, &builtin.OnExpertVotesUpdatedParams{
			Expert: actorAddr,
			Votes:  abi.NewTokenAmount(0),
		})
		require.True(t, ret.VotesEnough)
	})

	t.Run("unexpected expert states", func(t *testing.T) {
		rt, actor := setupFunc()

		for _, eState := range []expert.ExpertState{expert.ExpertStateBlocked, expert.ExpertStateRegistered} {
			st := getState(rt)
			st.ExpertState = eState
			rt.ReplaceState(st)

			rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "unexpected expert state", func() {
				actor.onVotesUpdated(rt, &builtin.OnExpertVotesUpdatedParams{
					Expert: actorAddr,
					Votes:  abi.NewTokenAmount(0),
				})
			})
		}
	})

	t.Run("other states", func(t *testing.T) {
		for _, eState := range []expert.ExpertState{expert.ExpertStateQualified, expert.ExpertStateUnqualified} {
			rt, actor := setupFunc()
			st := getState(rt)
			st.ExpertState = eState
			rt.ReplaceState(st)

			ret := actor.onVotesUpdated(rt, &builtin.OnExpertVotesUpdatedParams{
				Expert: actorAddr,
				Votes:  big.Sub(expert.ExpertVoteThreshold, abi.NewTokenAmount(1)),
			})
			st = getState(rt)
			require.True(t, ret.VotesEnough == false && st.ExpertState == expert.ExpertStateUnqualified)

			ret = actor.onVotesUpdated(rt, &builtin.OnExpertVotesUpdatedParams{
				Expert: actorAddr,
				Votes:  expert.ExpertVoteThreshold,
			})
			st = getState(rt)
			require.True(t, ret.VotesEnough == true && st.ExpertState == expert.ExpertStateQualified)
		}
	})
}

func newStoreDataParams(expertDataParams ...*expert.ImportDataParams) *builtin.BatchPieceCIDParams {
	var ret builtin.BatchPieceCIDParams
	for _, p := range expertDataParams {
		ret.PieceCIDs = append(ret.PieceCIDs, builtin.CheckedCID{CID: p.PieceID})
	}
	return &ret
}

func newImportDataParams() *expert.ImportDataParams {
	rd := rand.Intn(100) + 100
	rootID := tutil.MakeCID(strconv.Itoa(rd), &miner.SealedCIDPrefix)
	pieceID := tutil.MakeCID(strconv.Itoa(rd), &market.PieceCIDPrefix)
	return &expert.ImportDataParams{
		RootID:    rootID,
		PieceID:   pieceID,
		PieceSize: abi.PaddedPieceSize(rd) + abi.PaddedPieceSize(2<<10),
	}
}

type actorHarness struct {
	expert.Actor
	t testing.TB

	receiver addr.Address // The expert actor's own address
	owner    addr.Address
	proposer addr.Address
}

func newHarness(t testing.TB, receiver, owner, proposer addr.Address) *actorHarness {
	return &actorHarness{expert.Actor{}, t, receiver, owner, proposer}
}

func (h *actorHarness) constructAndVerify(rt *mock.Runtime, typ builtin.ExpertType) {
	params := expert.ConstructorParams{
		Owner:    h.owner,
		Proposer: h.owner,
		Type:     typ,
	}
	if typ == builtin.ExpertNormal {
		rt.SetBalance(expert.ExpertApplyCost)
		rt.SetReceived(expert.ExpertApplyCost)
		rt.ExpectSend(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, expert.ExpertApplyCost, nil, exitcode.Ok)
	}

	rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
	ret := rt.Call(h.Constructor, &params)
	require.Nil(h.t, ret)
	rt.Verify()
}

func (h *actorHarness) controlAddress(rt *mock.Runtime) addr.Address {
	rt.ExpectValidateCallerAny()
	owner := rt.Call(h.ControlAddress, nil).(*addr.Address)
	require.NotNil(h.t, owner)
	rt.Verify()
	return *owner
}

func (h *actorHarness) importData(rt *mock.Runtime, params *expert.ImportDataParams) {
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(h.owner)
	rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.OnExpertImport, &builtin.CheckedCID{
		CID: params.PieceID,
	}, abi.NewTokenAmount(0), nil, exitcode.Ok)
	rt.Call(h.ImportData, params)
	rt.Verify()
}

func (h *actorHarness) storeData(rt *mock.Runtime, params *builtin.BatchPieceCIDParams) *expert.GetDatasReturn {
	rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
	ret := rt.Call(h.StoreData, params).(*expert.GetDatasReturn)
	rt.Verify()
	return ret
}

func (h *actorHarness) getDatas(rt *mock.Runtime, params *builtin.BatchPieceCIDParams) *expert.GetDatasReturn {
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAny()
	ret := rt.Call(h.GetDatas, params).(*expert.GetDatasReturn)
	rt.Verify()
	return ret
}

func (h *actorHarness) nominate(rt *mock.Runtime, expertAddr addr.Address) {
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(h.owner)
	rt.ExpectSend(expertAddr, builtin.MethodsExpert.OnNominated, nil, big.Zero(), nil, exitcode.Ok)

	rt.Call(h.Nominate, &expertAddr)
	rt.Verify()
}

func (h *actorHarness) onNominate(rt *mock.Runtime) {
	rt.SetCaller(h.proposer, builtin.ExpertActorCodeID)
	rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
	rt.Call(h.OnNominated, nil)
	rt.Verify()
}

func (h *actorHarness) onBlocked(rt *mock.Runtime, proposer addr.Address, expectedVotesEnough bool) *expert.OnBlockedReturn {
	rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
	votesEnough := cbg.CborBool(expectedVotesEnough)
	rt.ExpectSend(proposer, builtin.MethodsExpert.OnImplicated, nil, big.Zero(), &votesEnough, exitcode.Ok)
	ret := rt.Call(h.OnBlocked, nil).(*expert.OnBlockedReturn)
	rt.Verify()
	return ret
}

// func (h *actorHarness) govBlock(rt *mock.Runtime, governor addr.Address) {
// 	rt.SetCaller(governor, builtin.AccountActorCodeID)
// 	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
// 	rt.ExpectSend(builtin.GovernActorAddr, builtin.MethodsGovern.ValidateGranted, &builtin.ValidateGrantedParams{
// 		Caller: governor,
// 		Method: builtin.MethodsExpert.GovBlock,
// 	}, big.Zero(), nil, exitcode.Ok)
// 	rt.ExpectSend(h.proposer, builtin.MethodsExpert.OnImplicated, nil, abi.NewTokenAmount(0), nil, exitcode.Ok)
// 	rt.ExpectSend(builtin.VoteFundActorAddr, builtin.MethodsVote.OnCandidateBlocked, nil, abi.NewTokenAmount(0), nil, exitcode.Ok)
// 	rt.Call(h.GovBlock, nil)
// 	rt.Verify()
// }

func (h *actorHarness) onImplicated(rt *mock.Runtime, nominatedAddr addr.Address) bool {
	rt.SetCaller(nominatedAddr, builtin.ExpertActorCodeID)
	rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
	ret := rt.Call(h.OnImplicated, nil).(*cbg.CborBool)
	rt.Verify()
	return bool(*ret)
}

func (h *actorHarness) onVotesUpdated(rt *mock.Runtime, params *builtin.OnExpertVotesUpdatedParams) *expert.OnVotesUpdatedReturn {
	rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
	ret := rt.Call(h.OnVotesUpdated, params).(*expert.OnVotesUpdatedReturn)
	rt.Verify()
	return ret
}

func (h *actorHarness) changeOwner(rt *mock.Runtime, oldOwner, newOwner addr.Address) {
	rt.SetCaller(oldOwner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(oldOwner)
	rt.Call(h.ChangeOwner, &newOwner)
	rt.Verify()
}

func (h *actorHarness) govChangeOwner(rt *mock.Runtime, governor, newOwner addr.Address) {
	rt.SetCaller(governor, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
	rt.ExpectSend(builtin.GovernActorAddr, builtin.MethodsGovern.ValidateGranted, &builtin.ValidateGrantedParams{
		Caller: governor,
		Method: builtin.MethodsExpert.GovChangeOwner,
	}, big.Zero(), nil, exitcode.Ok)
	rt.Call(h.GovChangeOwner, &newOwner)
	rt.Verify()
}

func (h *actorHarness) checkState(rt *mock.Runtime) *builtin.CheckExpertStateReturn {
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAny()
	ret := rt.Call(h.CheckState, nil).(*builtin.CheckExpertStateReturn)
	rt.Verify()
	return ret
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
