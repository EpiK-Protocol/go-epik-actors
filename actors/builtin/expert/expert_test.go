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
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/miner"
	mock "github.com/filecoin-project/specs-actors/v2/support/mock"
	tutil "github.com/filecoin-project/specs-actors/v2/support/testing"
	"github.com/stretchr/testify/require"
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

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "expert is unqualified", func() {
			rt.Call(actor.ImportData, newExpertDataParams())
		})
	})

	t.Run("ok", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		// first
		params := newExpertDataParams()
		rt.SetCaller(owner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(owner)
		rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.OnExpertImport, &builtin.OnExpertImportParams{
			PieceID: params.PieceID,
		}, abi.NewTokenAmount(0), nil, exitcode.Ok)
		rt.Call(actor.ImportData, params)
		rt.Verify()

		st := getState(rt)
		info := actor.getData(rt, params)
		require.True(t, st.DataCount == 1 &&
			info.PieceID == params.PieceID.String() &&
			info.RootID == params.RootID &&
			info.PieceSize == params.PieceSize)

		// second
		params2 := newExpertDataParams()
		actor.importData(rt, params2)
		st = getState(rt)
		info = actor.getData(rt, params2)
		require.True(t, st.DataCount == 2 &&
			info.PieceID == params2.PieceID.String() &&
			info.RootID == params2.RootID &&
			info.PieceSize == params2.PieceSize)
	})

	t.Run("fail when import duplicate data", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		params := newExpertDataParams()
		actor.importData(rt, params)

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "duplicate expert import", func() {
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

	t.Run("fail when unqualified", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertNormal)

		rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "expert is unqualified", func() {
			rt.Call(actor.StoreData, newExpertDataParams())
		})
	})

	t.Run("fail when data not imported", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "data not imported", func() {
			rt.Call(actor.StoreData, newExpertDataParams())
		})
	})

	t.Run("ok", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		params := newExpertDataParams()
		actor.importData(rt, params)

		// first time
		info := actor.storeData(rt, params)
		require.True(t, info.Redundancy == 1 &&
			info.PieceID == params.PieceID.String() &&
			info.RootID == params.RootID &&
			info.PieceSize == params.PieceSize)
		// second time
		info = actor.storeData(rt, params)
		require.True(t, info.Redundancy == 2 &&
			info.PieceID == params.PieceID.String() &&
			info.RootID == params.RootID &&
			info.PieceSize == params.PieceSize)
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
			rt.Call(actor.Nominate, &expert.NominateExpertParams{
				Expert: nominatedAddr,
			})
		})

		// caller not owner
		st := getState(rt)
		st.Status = expert.ExpertStateNormal
		rt.ReplaceState(st)

		caller := tutil.NewIDAddr(t, 200)
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(owner)
		rt.ExpectAbort(exitcode.SysErrForbidden, func() {
			rt.Call(actor.Nominate, &expert.NominateExpertParams{
				Expert: nominatedAddr,
			})
		})

		// normal
		rt.SetCaller(owner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(owner)
		rt.ExpectSend(nominatedAddr, builtin.MethodsExpert.OnNominated, nil, abi.NewTokenAmount(0), &builtin.Discard{}, exitcode.Ok)
		rt.Call(actor.Nominate, &expert.NominateExpertParams{
			Expert: nominatedAddr,
		})
	})

	t.Run("nominate with owner change(gov)", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		rt.SetEpoch(100)
		actor.govChangeOwner(rt, governor, newowner)

		rt.SetEpoch(100 + expert.NewOwnerActivateDelay)

		rt.SetCaller(newowner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(newowner)
		rt.ExpectSend(nominatedAddr, builtin.MethodsExpert.OnNominated, nil, abi.NewTokenAmount(0), &builtin.Discard{}, exitcode.Ok)
		rt.Call(actor.Nominate, &expert.NominateExpertParams{
			Expert: nominatedAddr,
		})

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

		for _, state := range []expert.ExpertState{expert.ExpertStateBlocked, expert.ExpertStateNominated, expert.ExpertStateNormal} {
			st := getState(rt)
			st.Status = state
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
		st.Status = expert.ExpertStateDisqualified
		rt.ReplaceState(st)

		rt.SetCaller(expertAddr, builtin.ExpertActorCodeID)
		rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
		rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.TrackNewNominated, nil, abi.NewTokenAmount(0), nil, exitcode.Ok)
		rt.Call(actor.OnNominated, nil)
		rt.Verify()

		st = getState(rt)
		require.True(t, st.LostEpoch == expert.NoLostEpoch && st.Status == expert.ExpertStateNominated)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, info.Proposer == expertAddr)
	})
}

func TestGovBlock(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	governor := tutil.NewIDAddr(t, 101)
	proposer := tutil.NewIDAddr(t, 1000)
	expertAddr := tutil.NewIDAddr(t, 1001)
	nominatedAddr := tutil.NewIDAddr(t, 1002)

	setupFunc := func(governor addr.Address) (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), expertAddr).
			WithActorType(owner, builtin.AccountActorCodeID).
			WithHasher(fixedHasher(0)).
			WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

		rt := builder.Build(t)
		actor := newHarness(t, expertAddr, owner, proposer)
		actor.constructAndVerify(rt, builtin.ExpertNormal)

		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(builtin.GovernActorAddr, builtin.MethodsGovern.ValidateGranted, &builtin.ValidateGrantedParams{
			Caller: governor,
			Method: builtin.MethodsExpert.GovBlock,
		}, big.Zero(), nil, exitcode.Ok)
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

		rt.SetCaller(governor, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(builtin.GovernActorAddr, builtin.MethodsGovern.ValidateGranted, &builtin.ValidateGrantedParams{
			Caller: governor,
			Method: builtin.MethodsExpert.GovBlock,
		}, big.Zero(), nil, exitcode.Ok)

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "foundation expert cannot be blocked", func() {
			rt.Call(actor.GovBlock, nil)
		})
	})

	t.Run("block expert with unexpected status", func(t *testing.T) {
		rt, actor := setupFunc(governor)

		st := getState(rt)
		st.Status = expert.ExpertStateBlocked
		rt.ReplaceState(st)
		rt.SetCaller(governor, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "expert already blocked", func() {
			rt.Call(actor.GovBlock, nil)
		})

		st = getState(rt)
		st.Status = expert.ExpertStateRegistered
		rt.ReplaceState(st)
		rt.SetCaller(governor, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(builtin.GovernActorAddr, builtin.MethodsGovern.ValidateGranted, &builtin.ValidateGrantedParams{
			Caller: governor,
			Method: builtin.MethodsExpert.GovBlock,
		}, big.Zero(), nil, exitcode.Ok)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "expert not nominated", func() {
			rt.Call(actor.GovBlock, nil)
		})
	})

	t.Run("ok", func(t *testing.T) {
		rt, actor := setupFunc(governor)

		st := getState(rt)
		info, err := st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		info.Proposer = proposer
		err = st.SaveInfo(adt.AsStore(rt), info)
		require.NoError(t, err)
		st.Status = expert.ExpertStateNormal
		rt.ReplaceState(st)

		rt.SetEpoch(101)

		rt.SetCaller(governor, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(proposer, builtin.MethodsExpert.OnImplicated, nil, abi.NewTokenAmount(0), nil, exitcode.Ok)
		rt.ExpectSend(builtin.VoteFundActorAddr, builtin.MethodsVote.OnCandidateBlocked, nil, abi.NewTokenAmount(0), nil, exitcode.Ok)
		rt.Call(actor.GovBlock, nil)
		rt.Verify()

		st = getState(rt)
		require.True(t, st.LostEpoch == 101 && st.Status == expert.ExpertStateBlocked)

		// try nominate will failed
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "nominator is unqualified", func() {
			rt.Call(actor.Nominate, &expert.NominateExpertParams{
				Expert: nominatedAddr,
			})
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

		rt.SetCaller(nominatedAddr, builtin.ExpertActorCodeID)
		rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
		rt.Call(actor.OnImplicated, nil)
		rt.Verify()

		st = getState(rt)
		require.True(t, st.ImplicatedTimes == 0)
	})

	t.Run("ok", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetCaller(nominatedAddr, builtin.ExpertActorCodeID)
		rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
		rt.Call(actor.OnImplicated, nil)
		rt.Verify()

		st := getState(rt)
		require.True(t, st.ImplicatedTimes == 1)
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

		rt.SetEpoch(100 + expert.NewOwnerActivateDelay)
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

		rt.SetEpoch(100 + expert.NewOwnerActivateDelay - 1)
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

		rt.SetEpoch(100 + expert.NewOwnerActivateDelay)
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

		// at NewOwnerActivateDelay-1
		rt.SetEpoch(100 + expert.NewOwnerActivateDelay - 1)
		actor.controlAddress(rt)
		st = getState(rt)
		info, err = st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, owner == info.Owner && newowner == info.ApplyNewOwner && 100 == info.ApplyNewOwnerEpoch)

		// at NewOwnerActivateDelay
		rt.SetEpoch(100 + expert.NewOwnerActivateDelay)
		actor.controlAddress(rt)
		st = getState(rt)
		info, err = st.GetInfo(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, newowner == info.Owner && newowner == info.ApplyNewOwner && -1 == info.ApplyNewOwnerEpoch)
	})
}

func TestOnTrackUpdate(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	governor := tutil.NewIDAddr(t, 101)
	actorProposer := tutil.NewIDAddr(t, 1000)
	actorAddr := tutil.NewIDAddr(t, 1001)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), actorAddr).
			WithActorType(owner, builtin.AccountActorCodeID).
			WithHasher(fixedHasher(0)).
			WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

		rt := builder.Build(t)
		actor := newHarness(t, actorAddr, owner, actorProposer)
		actor.constructAndVerify(rt, builtin.ExpertNormal)
		return rt, actor
	}

	t.Run("unexpected type (foundation)", func(t *testing.T) {
		builder := mock.NewBuilder(context.Background(), actorAddr).
			WithActorType(owner, builtin.AccountActorCodeID).
			WithHasher(fixedHasher(0)).
			WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

		rt := builder.Build(t)
		actor := newHarness(t, actorAddr, owner, actorProposer)
		actor.constructAndVerify(rt, builtin.ExpertFoundation)

		rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
		rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "unexpected expert status or type", func() {
			rt.Call(actor.OnTrackUpdate, &expert.OnTrackUpdateParams{Votes: big.Zero()})
		})
	})

	t.Run("unexpected status", func(t *testing.T) {
		rt, actor := setupFunc()

		for _, state := range []expert.ExpertState{expert.ExpertStateDisqualified, expert.ExpertStateRegistered} {
			st := getState(rt)
			st.Status = state
			rt.ReplaceState(st)

			rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
			rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
			rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "unexpected expert status or type", func() {
				rt.Call(actor.OnTrackUpdate, &expert.OnTrackUpdateParams{Votes: big.Zero()})
			})
		}
	})

	t.Run("expert is blocked", func(t *testing.T) {
		rt, actor := setupFunc()
		st := getState(rt)
		st.Status = expert.ExpertStateBlocked
		rt.ReplaceState(st)

		ret := actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: big.Zero()})
		require.True(t, ret.ResetMe == true && ret.UntrackMe == true)
	})

	t.Run("nominate without enough votes for 3 days", func(t *testing.T) {
		rt, actor := setupFunc()

		st := getState(rt)
		require.True(t, st.Status == expert.ExpertStateRegistered && st.LostEpoch == expert.NoLostEpoch)

		rt.SetEpoch(100)
		actor.onNominate(rt)
		st = getState(rt)
		require.True(t, st.Status == expert.ExpertStateNominated && st.LostEpoch == expert.NoLostEpoch)

		ret := actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: big.Sub(expert.ExpertVoteThreshold, big.NewInt(1))})
		st = getState(rt)
		require.True(t, ret.ResetMe == false && ret.UntrackMe == false)
		require.True(t, st.Status == expert.ExpertStateNominated && st.LostEpoch == 100)

		// status changes to disqualified
		rt.SetEpoch(100 + expert.ExpertVoteCheckPeriod)
		ret = actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: big.Sub(expert.ExpertVoteThreshold, big.NewInt(1))})
		st = getState(rt)
		require.True(t, ret.ResetMe == true && ret.UntrackMe == true &&
			st.Status == expert.ExpertStateDisqualified && st.LostEpoch == 100)

		// nominate again
		rt.SetEpoch(200 + expert.ExpertVoteCheckPeriod)
		actor.onNominate(rt)
		st = getState(rt)
		require.True(t, st.Status == expert.ExpertStateNominated && st.LostEpoch == expert.NoLostEpoch)
	})

	t.Run("nominate with enough votes", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetEpoch(100)
		actor.onNominate(rt)

		rt.SetEpoch(120)
		ret := actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: expert.ExpertVoteThreshold})
		st := getState(rt)
		require.True(t, ret.ResetMe == false && ret.UntrackMe == false)
		require.True(t, st.Status == expert.ExpertStateNormal && st.LostEpoch == expert.NoLostEpoch)

		rt.SetEpoch(130)
		ret = actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: big.Sub(expert.ExpertVoteThreshold, big.NewInt(1))})
		st = getState(rt)
		require.True(t, ret.ResetMe == false && ret.UntrackMe == false)
		require.True(t, st.Status == expert.ExpertStateNormal && st.LostEpoch == 130)

		rt.SetEpoch(129 + expert.ExpertVoteCheckPeriod)
		ret = actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: big.NewInt(1)})
		st = getState(rt)
		require.True(t, ret.ResetMe == false && ret.UntrackMe == false &&
			st.Status == expert.ExpertStateNormal && st.LostEpoch == 130)

		rt.SetEpoch(130 + expert.ExpertVoteCheckPeriod)
		ret = actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: big.Zero()})
		st = getState(rt)
		require.True(t, ret.ResetMe == true && ret.UntrackMe == true &&
			st.Status == expert.ExpertStateDisqualified && st.LostEpoch == 130)

		actor.govBlock(rt, governor)
		ret = actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: big.Zero()})
		st = getState(rt)
		require.True(t, ret.ResetMe == true && ret.UntrackMe == true && st.Status == expert.ExpertStateBlocked)
	})

	t.Run("lose qualification for implication", func(t *testing.T) {
		rt, actor := setupFunc()
		nominatedExpert := tutil.NewIDAddr(t, 2000)

		rt.SetEpoch(100)
		actor.onNominate(rt) // nominated

		rt.SetEpoch(120)
		ret := actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: expert.ExpertVoteThreshold}) // normal, NoLostEpoch

		rt.SetEpoch(130)
		actor.onImplicated(rt, nominatedExpert)
		ret = actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: expert.ExpertVoteThreshold})
		st := getState(rt)
		require.True(t, ret.ResetMe == false && ret.UntrackMe == false &&
			st.Status == expert.ExpertStateNormal && st.LostEpoch == 130)

		rt.SetEpoch(129 + expert.ExpertVoteCheckPeriod)
		ret = actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: big.Sub(big.Add(expert.ExpertVoteThreshold, expert.ExpertVoteThresholdAddition), big.NewInt(1))})
		st = getState(rt)
		require.True(t, ret.ResetMe == false && ret.UntrackMe == false && st.Status == expert.ExpertStateNormal && st.LostEpoch == 130)

		// rt.SetEpoch(130 + expert.ExpertVoteCheckPeriod)
		// ret = actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: big.Add(expert.ExpertVoteThreshold, expert.ExpertVoteThresholdAddition)})
		// st = getState(rt)
		// require.True(t, ret.ResetMe == false && ret.UntrackMe == false && st.Status == expert.ExpertStateNormal && st.LostEpoch == expert.NoLostEpoch)

		rt.SetEpoch(130 + expert.ExpertVoteCheckPeriod)
		ret = actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: big.Sub(big.Add(expert.ExpertVoteThreshold, expert.ExpertVoteThresholdAddition), big.NewInt(1))})
		st = getState(rt)
		require.True(t, ret.ResetMe == true && ret.UntrackMe == true && st.Status == expert.ExpertStateDisqualified && st.LostEpoch == 130)
	})

	t.Run("not enought to enough", func(t *testing.T) {
		rt, actor := setupFunc()

		st := getState(rt)

		rt.SetEpoch(100)
		actor.onNominate(rt) // nominated, NoLostEpoch
		ret := actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: big.Sub(expert.ExpertVoteThreshold, big.NewInt(1))})
		st = getState(rt)
		require.True(t, ret.ResetMe == false && ret.UntrackMe == false && st.Status == expert.ExpertStateNominated && st.LostEpoch == 100)

		rt.SetEpoch(120)
		ret = actor.onTrackUpdate(rt, &expert.OnTrackUpdateParams{Votes: expert.ExpertVoteThreshold})
		st = getState(rt)
		require.True(t, ret.ResetMe == false && ret.UntrackMe == false && st.Status == expert.ExpertStateNormal && st.LostEpoch == expert.NoLostEpoch)
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
		{"ExpertStateNominated", expert.ExpertStateNominated, true, false},
		{"ExpertStateNormal", expert.ExpertStateNormal, true, true},
		{"ExpertStateBlocked", expert.ExpertStateBlocked, false, false},
		{"ExpertStateDisqualified", expert.ExpertStateDisqualified, false, false},
	}

	for _, ts := range testcases {
		t.Run(ts.name, func(t *testing.T) {
			rt := builder.Build(t)
			actor.constructAndVerify(rt, builtin.ExpertNormal)

			st := getState(rt)
			st.Status = ts.status
			rt.ReplaceState(st)

			rt.SetCaller(owner, builtin.AccountActorCodeID)
			rt.ExpectValidateCallerAny()
			ret := rt.Call(actor.CheckState, nil).(*expert.CheckStateReturn)
			rt.Verify()
			require.True(t, ret.AllowVote == ts.expectAllowVote && ret.Qualified == ts.expectQualified)
		})
	}
}

func newExpertDataParams() *expert.ExpertDataParams {
	rd := rand.Intn(100) + 100
	pieceID := tutil.MakeCID(strconv.Itoa(rd), &miner.SealedCIDPrefix)
	return &expert.ExpertDataParams{
		RootID:    "root|" + pieceID.String(),
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

func (h *actorHarness) controlAddress(rt *mock.Runtime) (owner addr.Address) {
	rt.ExpectValidateCallerAny()
	ret := rt.Call(h.ControlAddress, nil).(*builtin.ExpertControlAddressReturn)
	require.NotNil(h.t, ret)
	rt.Verify()
	return ret.Owner
}

func (h *actorHarness) importData(rt *mock.Runtime, params *expert.ExpertDataParams) {
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(h.owner)
	rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.OnExpertImport, &builtin.OnExpertImportParams{
		PieceID: params.PieceID,
	}, abi.NewTokenAmount(0), nil, exitcode.Ok)
	rt.Call(h.ImportData, params)
	rt.Verify()
}

func (h *actorHarness) storeData(rt *mock.Runtime, params *expert.ExpertDataParams) *expert.DataOnChainInfo {
	rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
	ret := rt.Call(h.StoreData, params).(*expert.DataOnChainInfo)
	rt.Verify()
	return ret
}

func (h *actorHarness) getData(rt *mock.Runtime, params *expert.ExpertDataParams) *expert.DataOnChainInfo {
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAny()
	ret := rt.Call(h.GetData, params).(*expert.DataOnChainInfo)
	rt.Verify()
	return ret
}

func (h *actorHarness) nominate(rt *mock.Runtime, params *expert.NominateExpertParams) {
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(h.owner)
	rt.ExpectSend(params.Expert, builtin.MethodsExpert.OnNominated, nil, big.Zero(), nil, exitcode.Ok)

	rt.Call(h.Nominate, params)
	rt.Verify()
}

func (h *actorHarness) onNominate(rt *mock.Runtime) {
	rt.SetCaller(h.proposer, builtin.ExpertActorCodeID)
	rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
	rt.ExpectSend(builtin.ExpertFundActorAddr, builtin.MethodsExpertFunds.TrackNewNominated, nil, abi.NewTokenAmount(0), nil, exitcode.Ok)
	rt.Call(h.OnNominated, nil)
	rt.Verify()
}

func (h *actorHarness) govBlock(rt *mock.Runtime, governor addr.Address) {
	rt.SetCaller(governor, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
	rt.ExpectSend(builtin.GovernActorAddr, builtin.MethodsGovern.ValidateGranted, &builtin.ValidateGrantedParams{
		Caller: governor,
		Method: builtin.MethodsExpert.GovBlock,
	}, big.Zero(), nil, exitcode.Ok)
	rt.ExpectSend(h.proposer, builtin.MethodsExpert.OnImplicated, nil, abi.NewTokenAmount(0), nil, exitcode.Ok)
	rt.ExpectSend(builtin.VoteFundActorAddr, builtin.MethodsVote.OnCandidateBlocked, nil, abi.NewTokenAmount(0), nil, exitcode.Ok)
	rt.Call(h.GovBlock, nil)
	rt.Verify()
}

func (h *actorHarness) onImplicated(rt *mock.Runtime, nominatedAddr addr.Address) {
	st := getState(rt)
	before := st.ImplicatedTimes
	rt.SetCaller(nominatedAddr, builtin.ExpertActorCodeID)
	rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
	rt.Call(h.OnImplicated, nil)
	rt.Verify()
	require.True(h.t, getState(rt).ImplicatedTimes-before == 1)
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

func (h *actorHarness) onTrackUpdate(rt *mock.Runtime, params *expert.OnTrackUpdateParams) *expert.OnTrackUpdateReturn {
	rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.ExpertFundActorAddr)
	ret := rt.Call(h.OnTrackUpdate, params).(*expert.OnTrackUpdateReturn)
	rt.Verify()
	return ret
}

func (h *actorHarness) checkState(rt *mock.Runtime) *expert.CheckStateReturn {
	rt.SetCaller(h.owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAny()
	ret := rt.Call(h.CheckState, nil).(*expert.CheckStateReturn)
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
