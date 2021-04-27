package expertfund_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	expert "github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expertfund"
	initact "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/filecoin-project/specs-actors/v2/support/mock"
	tutil "github.com/filecoin-project/specs-actors/v2/support/testing"
	"github.com/stretchr/testify/require"
)

func getState(rt *mock.Runtime) *expertfund.State {
	var st expertfund.State
	rt.GetState(&st)
	return &st
}

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, expertfund.Actor{})
}

func TestConstruction(t *testing.T) {
	actor := actorHarness{expertfund.Actor{}, t}
	receiver := tutil.NewIDAddr(t, 100)
	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("construction", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		actor.checkState(rt)
	})
}

func TestApplyForExpert(t *testing.T) {
	actor := actorHarness{expertfund.Actor{}, t}
	receiver := tutil.NewIDAddr(t, 100)
	applicant1 := tutil.NewIDAddr(t, 101)
	applicant2 := tutil.NewIDAddr(t, 102)
	expert1 := tutil.NewIDAddr(t, 1000)
	expert2 := tutil.NewIDAddr(t, 1001)
	uniqueAddr1 := tutil.NewActorAddr(t, "expert1")
	uniqueAddr2 := tutil.NewActorAddr(t, "expert2")
	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("apply for experts", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		rt.SetBalance(abi.NewTokenAmount(198))
		// first is foundation expert
		rt.SetReceived(abi.NewTokenAmount(99))
		rt.SetCaller(applicant1, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)

		ctorParams := expert.ConstructorParams{
			Owner:    applicant1,
			Proposer: applicant1,
			Type:     builtin.ExpertFoundation,
		}
		ctorParamBuf := new(bytes.Buffer)
		err := ctorParams.MarshalCBOR(ctorParamBuf)
		require.NoError(t, err)
		rt.ExpectSend(builtin.InitActorAddr, builtin.MethodsInit.Exec,
			&initact.ExecParams{
				CodeCID:           builtin.ExpertActorCodeID,
				ConstructorParams: ctorParamBuf.Bytes(),
			}, abi.NewTokenAmount(99), &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1}, exitcode.Ok)

		ret1 := (rt.Call(actor.ApplyForExpert, &expertfund.ApplyForExpertParams{Owner: applicant1})).(*expertfund.ApplyForExpertReturn)
		rt.Verify()

		sum := actor.checkState(rt)
		st := getState(rt)
		require.True(t, sum.ExpertsCount == 1 && ret1.IDAddress == expert1)
		_, err = st.GetExpert(adt.AsStore(rt), ret1.IDAddress)
		require.NoError(t, err)

		// second is normal expert
		rt.SetReceived(abi.NewTokenAmount(99))
		rt.SetCaller(applicant2, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)

		ctorParams = expert.ConstructorParams{
			Owner:    applicant2,
			Proposer: applicant2,
			Type:     builtin.ExpertNormal,
		}
		ctorParamBuf = new(bytes.Buffer)
		err = ctorParams.MarshalCBOR(ctorParamBuf)
		require.NoError(t, err)
		rt.ExpectSend(builtin.InitActorAddr, builtin.MethodsInit.Exec,
			&initact.ExecParams{
				CodeCID:           builtin.ExpertActorCodeID,
				ConstructorParams: ctorParamBuf.Bytes(),
			}, abi.NewTokenAmount(99), &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2}, exitcode.Ok)

		ret2 := (rt.Call(actor.ApplyForExpert, &expertfund.ApplyForExpertParams{Owner: applicant2})).(*expertfund.ApplyForExpertReturn)
		rt.Verify()

		sum = actor.checkState(rt)
		st = getState(rt)
		require.True(t, sum.ExpertsCount == 2 && ret2.IDAddress == expert2)
		_, err = st.GetExpert(adt.AsStore(rt), ret2.IDAddress)
		require.NoError(t, err)
	})
}

func TestOnExpertImport(t *testing.T) {
	receiver := tutil.NewIDAddr(t, 100)
	expert1 := tutil.NewIDAddr(t, 1000)
	expert2 := tutil.NewIDAddr(t, 1001)

	actor := actorHarness{expertfund.Actor{}, t}
	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("must put absent data", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		// piece 1
		pieceID1 := tutil.MakeCID("1", &miner.SealedCIDPrefix)
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})
		st := getState(rt)
		pieceToExpert, pieceToThreshold, err := st.GetPieceInfos(adt.AsStore(rt), pieceID1)
		require.NoError(t, err)
		require.True(t, pieceToExpert[pieceID1] == expert1 && pieceToThreshold[pieceID1] == st.DataStoreThreshold)

		// piece 2
		pieceID2 := tutil.MakeCID("2", &miner.SealedCIDPrefix)
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID2})
		st = getState(rt)
		pieceToExpert2, _, err := st.GetPieceInfos(adt.AsStore(rt), pieceID2)
		require.NoError(t, err)
		require.True(t, pieceToExpert2[pieceID2] == expert2)

		// not found
		_, _, err = st.GetPieceInfos(adt.AsStore(rt), tutil.MakeCID("3", &miner.SealedCIDPrefix))
		require.True(t, strings.Contains(err.Error(), "piece not found"))

		// re-put piece 1 with expert 1
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "already exists "+pieceID1.String(), func() {
			actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})
		})
		// re-put piece 1 with expert 2
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "already exists "+pieceID1.String(), func() {
			actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID1})
		})
	})
}

func TestGetData(t *testing.T) {
	receiver := tutil.NewIDAddr(t, 100)
	expert1 := tutil.NewIDAddr(t, 1000)

	actor := actorHarness{expertfund.Actor{}, t}
	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("get existing data", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		rootID := tutil.MakeCID("1", &miner.SealedCIDPrefix)
		pieceID := tutil.MakeCID("1", &market.PieceCIDPrefix)
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID})

		di := actor.getData(rt, expert1, &builtin.CheckedCID{CID: pieceID}, &expert.DataOnChainInfo{PieceID: pieceID, RootID: rootID})
		require.True(t, di.Expert == expert1 && di.Data.PieceID == pieceID && di.Data.RootID == rootID)
	})

	t.Run("get nonexistent data", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		rootID := tutil.MakeCID("1", &miner.SealedCIDPrefix)
		pieceID := tutil.MakeCID("1", &market.PieceCIDPrefix)

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "piece not found", func() {
			actor.getData(rt, expert1, &builtin.CheckedCID{CID: pieceID}, &expert.DataOnChainInfo{PieceID: pieceID, RootID: rootID})
		})
	})

	t.Run("call expert actor error", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		rootID := tutil.MakeCID("1", &miner.SealedCIDPrefix)
		pieceID := tutil.MakeCID("1", &market.PieceCIDPrefix)
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID})

		rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
		rt.ExpectValidateCallerAny()
		params := &builtin.BatchPieceCIDParams{PieceCIDs: []builtin.CheckedCID{{CID: pieceID}}}
		expRet := &expert.GetDatasReturn{
			Infos: []*expert.DataOnChainInfo{{PieceID: pieceID, RootID: rootID}},
		}
		rt.ExpectSend(expert1, builtin.MethodsExpert.GetDatas, params, abi.NewTokenAmount(0), expRet, exitcode.ErrForbidden)

		rt.ExpectAbort(exitcode.ErrForbidden, func() {
			rt.Call(actor.GetData, &builtin.CheckedCID{CID: pieceID})
		})
	})
}

// func TestBlockExpert(t *testing.T) {
// 	owner := tutil.NewIDAddr(t, 100)
// 	receiver := tutil.NewIDAddr(t, 100)
// 	governor := tutil.NewIDAddr(t, 101)
// 	proposer := tutil.NewIDAddr(t, 1001)
// 	expertAddr := tutil.NewIDAddr(t, 1002)

// 	setupFunc := func() (*mock.Runtime, *actorHarness) {
// 		actor := actorHarness{expertfund.Actor{}, t}
// 		builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
// 		rt := builder.Build(t)
// 		actor.constructAndVerify(rt)
// 		return rt, &actor
// 	}

// 	t.Run("foundation expert cannot be blocked", func(t *testing.T) {
// 		rt, actor := setupFunc()

// 		actor.applyForExpert(rt, owner)
// 	})

// 	t.Run("block expert with unexpected status", func(t *testing.T) {
// 		rt, actor := setupFunc(governor)

// 		st := getState(rt)
// 		st.Status = expert.ExpertStateBlocked
// 		rt.ReplaceState(st)
// 		rt.SetCaller(governor, builtin.AccountActorCodeID)
// 		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
// 		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "expert already blocked", func() {
// 			rt.Call(actor.GovBlock, nil)
// 		})

// 		st = getState(rt)
// 		st.Status = expert.ExpertStateRegistered
// 		rt.ReplaceState(st)
// 		rt.SetCaller(governor, builtin.AccountActorCodeID)
// 		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
// 		rt.ExpectSend(builtin.GovernActorAddr, builtin.MethodsGovern.ValidateGranted, &builtin.ValidateGrantedParams{
// 			Caller: governor,
// 			Method: builtin.MethodsExpert.GovBlock,
// 		}, big.Zero(), nil, exitcode.Ok)
// 		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "expert not nominated", func() {
// 			rt.Call(actor.GovBlock, nil)
// 		})
// 	})

// 	t.Run("ok", func(t *testing.T) {
// 		rt, actor := setupFunc(governor)

// 		st := getState(rt)
// 		info, err := st.GetInfo(adt.AsStore(rt))
// 		require.NoError(t, err)
// 		info.Proposer = proposer
// 		err = st.SaveInfo(adt.AsStore(rt), info)
// 		require.NoError(t, err)
// 		st.Status = expert.ExpertStateNormal
// 		rt.ReplaceState(st)

// 		rt.SetEpoch(101)

// 		rt.SetCaller(governor, builtin.AccountActorCodeID)
// 		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
// 		rt.ExpectSend(proposer, builtin.MethodsExpert.OnImplicated, nil, abi.NewTokenAmount(0), nil, exitcode.Ok)
// 		rt.ExpectSend(builtin.VoteFundActorAddr, builtin.MethodsVote.OnCandidateBlocked, nil, abi.NewTokenAmount(0), nil, exitcode.Ok)
// 		rt.Call(actor.GovBlock, nil)
// 		rt.Verify()

// 		st = getState(rt)
// 		require.True(t, st.LostEpoch == 101 && st.Status == expert.ExpertStateBlocked)

// 		// try nominate will failed
// 		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "nominator is unqualified", func() {
// 			rt.Call(actor.Nominate, &nominatedAddr)
// 		})
// 	})
// }

func TestChangeThreshold(t *testing.T) {

	actor := actorHarness{expertfund.Actor{}, t}
	receiver := tutil.NewIDAddr(t, 100)
	governor := tutil.NewIDAddr(t, 101)
	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("ok", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		st := getState(rt)
		require.True(t, st.DataStoreThreshold == expertfund.DefaultDataStoreThreshold)

		rt.SetEpoch(101)

		rt.SetCaller(governor, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
		rt.ExpectSend(builtin.GovernActorAddr, builtin.MethodsGovern.ValidateGranted, &builtin.ValidateGrantedParams{
			Caller: governor,
			Method: builtin.MethodsExpertFunds.ChangeThreshold,
		}, big.Zero(), nil, exitcode.Ok)
		rt.Call(actor.ChangeThreshold, &expertfund.ChangeThresholdParams{expertfund.DefaultDataStoreThreshold + 1})
		rt.Verify()

		st = getState(rt)
		require.True(t, st.DataStoreThreshold == expertfund.DefaultDataStoreThreshold+1)
	})
}

func TestBatchCheckData(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	receiver := tutil.NewIDAddr(t, 100)
	expert1 := tutil.NewIDAddr(t, 1000)
	uniqueAddr1 := tutil.NewActorAddr(t, "expert1")
	expert2 := tutil.NewIDAddr(t, 1001)
	uniqueAddr2 := tutil.NewActorAddr(t, "expert2")

	pieceID1 := tutil.MakeCID("1", &market.PieceCIDPrefix)
	pieceID2 := tutil.MakeCID("2", &market.PieceCIDPrefix)
	pieceID3 := tutil.MakeCID("3", &market.PieceCIDPrefix)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		actor := actorHarness{expertfund.Actor{}, t}
		builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
		rt := builder.Build(t)
		actor.constructAndVerify(rt)
		return rt, &actor
	}

	t.Run("piece size mismatched", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1})
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})

		rt.SetCaller(owner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAny()
		rt.ExpectSend(expert1, builtin.MethodsExpert.GetDatas, &builtin.BatchPieceCIDParams{PieceCIDs: []builtin.CheckedCID{{CID: pieceID1}}},
			big.Zero(), &expert.GetDatasReturn{
				Infos: []*expert.DataOnChainInfo{{
					RootID:    pieceID1,
					PieceID:   pieceID1,
					PieceSize: 100,
				}},
			}, exitcode.Ok)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "piece size mismatched", func() {
			rt.Call(actor.BatchCheckData, &expertfund.BatchCheckDataParams{
				CheckedPieces: []expertfund.CheckedPiece{{
					PieceCID:  pieceID1,
					PieceSize: 99,
				}},
			})
		})
	})

	t.Run("piece not found", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1})
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})

		rt.SetCaller(owner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAny()
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "piece not found", func() {
			rt.Call(actor.BatchCheckData, &expertfund.BatchCheckDataParams{
				CheckedPieces: []expertfund.CheckedPiece{{
					PieceCID:  pieceID2,
					PieceSize: 100,
				}},
			})
		})
	})

	t.Run("duplicate", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1})
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})

		rt.SetCaller(owner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAny()
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "duplicate piece", func() {
			rt.Call(actor.BatchCheckData, &expertfund.BatchCheckDataParams{
				CheckedPieces: []expertfund.CheckedPiece{{
					PieceCID:  pieceID1,
					PieceSize: 100,
				}, {
					PieceCID:  pieceID1,
					PieceSize: 100,
				}},
			})
		})
	})

	t.Run("ok", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1})
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})

		rt.SetCaller(owner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAny()
		rt.ExpectSend(expert1, builtin.MethodsExpert.GetDatas, &builtin.BatchPieceCIDParams{PieceCIDs: []builtin.CheckedCID{{CID: pieceID1}}},
			big.Zero(), &expert.GetDatasReturn{
				Infos: []*expert.DataOnChainInfo{{
					RootID:    pieceID1,
					PieceID:   pieceID1,
					PieceSize: 100,
				}},
			}, exitcode.Ok)
		rt.Call(actor.BatchCheckData, &expertfund.BatchCheckDataParams{
			CheckedPieces: []expertfund.CheckedPiece{{
				PieceCID:  pieceID1,
				PieceSize: 100,
			}},
		})
	})

	t.Run("multi files ok", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1})
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2})
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID2})
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID3})

		rt.SetCaller(owner, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAny()
		rt.ExpectSend(expert1, builtin.MethodsExpert.GetDatas, &builtin.BatchPieceCIDParams{PieceCIDs: []builtin.CheckedCID{{CID: pieceID1}, {CID: pieceID2}}},
			big.Zero(), &expert.GetDatasReturn{
				Infos: []*expert.DataOnChainInfo{{
					RootID:    pieceID1,
					PieceID:   pieceID1,
					PieceSize: 100,
				}, {
					RootID:    pieceID2,
					PieceID:   pieceID2,
					PieceSize: 200,
				}},
			}, exitcode.Ok)
		rt.ExpectSend(expert2, builtin.MethodsExpert.GetDatas, &builtin.BatchPieceCIDParams{PieceCIDs: []builtin.CheckedCID{{CID: pieceID3}}},
			big.Zero(), &expert.GetDatasReturn{
				Infos: []*expert.DataOnChainInfo{{
					RootID:    pieceID3,
					PieceID:   pieceID3,
					PieceSize: 300,
				}},
			}, exitcode.Ok)
		rt.Call(actor.BatchCheckData, &expertfund.BatchCheckDataParams{
			CheckedPieces: []expertfund.CheckedPiece{{
				PieceCID:  pieceID1,
				PieceSize: 100,
			}, {
				PieceCID:  pieceID2,
				PieceSize: 200,
			}, {
				PieceCID:  pieceID3,
				PieceSize: 300,
			}},
		})
	})
}

func TestBatchStoreData(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	receiver := tutil.NewIDAddr(t, 100)
	expert1 := tutil.NewIDAddr(t, 1000)
	uniqueAddr1 := tutil.NewActorAddr(t, "expert1")
	expert2 := tutil.NewIDAddr(t, 1001)
	uniqueAddr2 := tutil.NewActorAddr(t, "expert2")

	pieceID1 := tutil.MakeCID("1", &market.PieceCIDPrefix)
	pieceID2 := tutil.MakeCID("2", &market.PieceCIDPrefix)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		actor := actorHarness{expertfund.Actor{}, t}
		builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
		rt := builder.Build(t)
		actor.constructAndVerify(rt)
		return rt, &actor
	}

	t.Run("store files from foundation expert", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1}) // foundation, active
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true))
		actor.expectPool(rt, 0, 0, 0, 0, 0, 0)

		rt.SetEpoch(100)
		rt.SetBalance(abi.NewTokenAmount(1234))
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})

		// expert2, DataStoreThreshold - 1
		st := getState(rt)
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert1: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert1: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  100,
					Redundancy: st.DataStoreThreshold - 1,
				}}},
			},
		})
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true))
		actor.expectPool(rt, 0, 0, 0, 0, 0, 0)

		// expert2, DataStoreThreshold
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert1: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert1: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  100,
					Redundancy: st.DataStoreThreshold,
				}}},
			},
		})
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10)) // Sqrt(PieceSize)
		actor.expectPool(rt, 0, 0, 0, 100, 0, 10)

		// expert2, DataStoreThreshold+1
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert1: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert1: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  100,
					Redundancy: st.DataStoreThreshold + 1,
				}}},
			},
		})
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10))
		actor.expectPool(rt, 0, 0, 0, 100, 0, 10) // no deposit triggered
	})

	t.Run("no deposit when beyond threshold", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1}) // foundation, active
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2})     // inactive
		actor.expectExpert(rt, expert2, newExpectExpertInfo())

		// activate expert2
		rt.SetEpoch(100)
		actor.onExpertVotesUpdated(rt, expert2, true)
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true))
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID1})
		actor.expectPool(rt, 0, 0, 0, 100, 0, 0)

		// expert2, DataStoreThreshold + 1
		st := getState(rt)
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert2: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert2: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  400,
					Redundancy: st.DataStoreThreshold + 1,
				}}},
			},
		})
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true))
		actor.expectPool(rt, 0, 0, 0, 100, 0, 0)
	})

	t.Run("no deposit when store files from inactive expert", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1}) // foundation, active
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2})     // inactive
		actor.expectExpert(rt, expert2, newExpectExpertInfo())

		rt.SetEpoch(100)
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID1})

		// expert2, DataStoreThreshold
		st := getState(rt)
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert2: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert2: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  100,
					Redundancy: st.DataStoreThreshold,
				}}},
			},
		})
		actor.expectExpert(rt, expert2, newExpectExpertInfo())
		actor.expectPool(rt, 0, 0, 0, 0, 0, 0)
	})

	t.Run("both foundation and normal", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1}) // foundation, active
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2})     // inactive

		actor.onExpertVotesUpdated(rt, expert2, true) // activate expert2
		// import files
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID2})

		// store peice1
		rt.SetEpoch(100)
		rt.SetBalance(abi.NewTokenAmount(100))
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert1: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert1: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  400,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(20))
		actor.expectPool(rt, 0, 0, 0, 100, 0, 20)

		// store peice2
		rt.SetEpoch(200)
		rt.SetBalance(abi.NewTokenAmount(350))
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert2: {{CID: pieceID2}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert2: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID2,
					PieceID:    pieceID2,
					PieceSize:  100,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.expectPool(rt, 175*1e11, 350, 100, 200, 20, 30)
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(10).withRewardDebt(175))
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(20))
	})
}

func TestClaim(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	receiver := tutil.NewIDAddr(t, 100)
	expert1 := tutil.NewIDAddr(t, 1000)
	uniqueAddr1 := tutil.NewActorAddr(t, "expert1")
	expert2 := tutil.NewIDAddr(t, 1001)
	uniqueAddr2 := tutil.NewActorAddr(t, "expert2")

	pieceID1 := tutil.MakeCID("1", &market.PieceCIDPrefix)
	pieceID2 := tutil.MakeCID("2", &market.PieceCIDPrefix)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		actor := actorHarness{expertfund.Actor{}, t}
		builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
		rt := builder.Build(t)
		actor.constructAndVerify(rt)
		return rt, &actor
	}

	t.Run("multi pieces in different epochs", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1}) // foundation, active
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2})     // inactive

		actor.onExpertVotesUpdated(rt, expert2, true) // activate expert2
		// import files
		rt.SetEpoch(100)
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID2})

		// store peice1
		rt.SetEpoch(150)
		rt.SetBalance(abi.NewTokenAmount(100))
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert1: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert1: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  400,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(20))
		actor.expectPool(rt, 0, 0, 0, 150, 0, 20)

		// store peice2
		rt.SetEpoch(200)
		rt.SetBalance(abi.NewTokenAmount(350))
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert2: {{CID: pieceID2}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert2: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID2,
					PieceID:    pieceID2,
					PieceSize:  100,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(10).withRewardDebt(175))
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(20))
		actor.expectPool(rt, 175*1e11, 350, 150, 200, 20, 30)

		// first claim
		start1 := abi.ChainEpoch(210)
		rt.SetEpoch(start1)
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(100000), big.Zero())
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).
			withDatasize(20).
			withRewardDebt(350).
			withLockedFunds(350).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(210): abi.NewTokenAmount(350),
			}))
		actor.expectPool(rt, 175*1e11, 350, 200, 210, 30, 30)

		rt.SetEpoch(220)
		actor.claim(rt, expert2, owner, abi.NewTokenAmount(100000), big.Zero())
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(10).withRewardDebt(175))
		actor.expectPool(rt, 175*1e11, 350, 210, 220, 30, 30)

		// immature
		rt.SetEpoch(start1 + expertfund.RewardVestingDelay)
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(100000), big.Zero())
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).
			withDatasize(20).
			withRewardDebt(350).
			withLockedFunds(350).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(210): abi.NewTokenAmount(350),
			}))
		// mature
		rt.SetEpoch(start1 + expertfund.RewardVestingDelay + 1)
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(100000), abi.NewTokenAmount(350))
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).
			withDatasize(20).
			withRewardDebt(350))
		actor.expectPool(rt, 175*1e11, 0, int64(start1+expertfund.RewardVestingDelay), int64(start1+expertfund.RewardVestingDelay)+1, 30, 30)
	})

	t.Run("multi claims in same epoch", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1}) // foundation, active
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2})     // inactive
		actor.onExpertVotesUpdated(rt, expert2, true)                                                                                  // activate expert2

		// import files
		rt.SetEpoch(100)
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID2})

		// store peice1 and piece2
		rt.SetEpoch(150)
		rt.SetBalance(abi.NewTokenAmount(300))
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert1: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert1: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  400,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert2: {{CID: pieceID2}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert2: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID2,
					PieceID:    pieceID2,
					PieceSize:  100,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(20))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(10))
		actor.expectPool(rt, 0, 0, 0, 150, 0, 30)

		rt.SetEpoch(150)
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(100000), big.Zero())
		actor.expectPool(rt, 0, 0, 0, 150, 0, 30)

		// claim expert1, expert2 at 151
		rt.SetEpoch(151)
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(100000), big.Zero())
		actor.claim(rt, expert2, owner, abi.NewTokenAmount(100000), big.Zero())
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(20).withRewardDebt(200).withLockedFunds(200).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(151): abi.NewTokenAmount(200),
			}))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(10).withRewardDebt(100).withLockedFunds(100).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(151): abi.NewTokenAmount(100),
			}))
		actor.expectPool(rt, 100*1e11, 300, 150, 151, 30, 30)

		// re-claim expert1 at 151 + expertfund.RewardVestingDelay
		rt.SetEpoch(151 + expertfund.RewardVestingDelay)
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(100000), big.Zero())
		actor.claim(rt, expert2, owner, abi.NewTokenAmount(100000), big.Zero())
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(20).withRewardDebt(200).withLockedFunds(200).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(151): abi.NewTokenAmount(200),
			}))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(10).withRewardDebt(100).withLockedFunds(100).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(151): abi.NewTokenAmount(100),
			}))
		actor.expectPool(rt, 100*1e11, 300, 151, 151+int64(expertfund.RewardVestingDelay), 30, 30)

		// re-claim expert1 at 151 + expertfund.RewardVestingDelay + 1
		rt.SetEpoch(151 + expertfund.RewardVestingDelay + 1)
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(100000), abi.NewTokenAmount(200))
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(20).withRewardDebt(200))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(10).withRewardDebt(100).withLockedFunds(100).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(151): abi.NewTokenAmount(100),
			}))
		actor.expectPool(rt, 100*1e11, 100, 151+int64(expertfund.RewardVestingDelay), 152+int64(expertfund.RewardVestingDelay), 30, 30)
		require.True(t, rt.Balance().Equals(big.NewInt(100)))

		// re-claim expert2 at 151 + expertfund.RewardVestingDelay + 100
		rt.SetEpoch(151 + expertfund.RewardVestingDelay + 100)
		actor.claim(rt, expert2, owner, abi.NewTokenAmount(100000), abi.NewTokenAmount(100))
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(20).withRewardDebt(200))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(10).withRewardDebt(100))
		actor.expectPool(rt, 100*1e11, 0, 152+int64(expertfund.RewardVestingDelay), 251+int64(expertfund.RewardVestingDelay), 30, 30)
		require.True(t, rt.Balance().Equals(big.NewInt(0)))
	})

	t.Run("multi claims in same epoch", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1})
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})

		// store data
		rt.SetEpoch(100)
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert1: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert1: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  100,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.expectPool(rt, 0, 0, 0, 100, 0, 10)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10))

		// first claim
		rt.SetEpoch(200)
		rt.SetBalance(abi.NewTokenAmount(300))
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(10000), big.Zero())
		actor.expectDisqualifiedExpert(rt, expert1, false, -1)
		actor.expectPool(rt, 300*1e11, 300, 100, 200, 10, 10)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10).withRewardDebt(300).withLockedFunds(300).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(200): abi.NewTokenAmount(300),
			}))
		// second claim
		rt.SetBalance(abi.NewTokenAmount(500))
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(10000), big.Zero())
		actor.expectDisqualifiedExpert(rt, expert1, false, -1)
		actor.expectPool(rt, 500*1e11, 500, 100, 200, 10, 10)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10).withRewardDebt(500).withLockedFunds(500).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(200): abi.NewTokenAmount(500),
			}))
	})
}

func TestOnExpertVotesUpdated(t *testing.T) {
	invalidEpoch := abi.ChainEpoch(-1)
	owner := tutil.NewIDAddr(t, 100)
	receiver := tutil.NewIDAddr(t, 100)
	expert1 := tutil.NewIDAddr(t, 1000)
	uniqueAddr1 := tutil.NewActorAddr(t, "expert1")
	expert2 := tutil.NewIDAddr(t, 1001)
	uniqueAddr2 := tutil.NewActorAddr(t, "expert2")

	pieceID1 := tutil.MakeCID("1", &market.PieceCIDPrefix)
	pieceID2 := tutil.MakeCID("2", &market.PieceCIDPrefix)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		actor := actorHarness{expertfund.Actor{}, t}
		builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
		rt := builder.Build(t)
		actor.constructAndVerify(rt)
		return rt, &actor
	}

	t.Run("votes change without data", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1})
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2})

		st := getState(rt)
		_, found, err := st.GetDisqualifiedExpertInfo(adt.AsStore(rt), expert2)
		require.NoError(t, err)
		require.False(t, found)

		// new expert, no enough votes, also no record in DisqualifiedExperts
		rt.SetEpoch(100)
		actor.onExpertVotesUpdated(rt, expert2, false)
		actor.expectDisqualifiedExpert(rt, expert2, false, invalidEpoch)

		// enough
		rt.SetEpoch(200)
		actor.onExpertVotesUpdated(rt, expert2, true)
		actor.expectDisqualifiedExpert(rt, expert2, false, invalidEpoch)

		rt.SetEpoch(300)
		actor.onExpertVotesUpdated(rt, expert2, false)
		actor.expectDisqualifiedExpert(rt, expert2, true, 300)

		rt.SetEpoch(400)
		actor.onExpertVotesUpdated(rt, expert2, false)
		actor.expectDisqualifiedExpert(rt, expert2, true, 300)

		rt.SetEpoch(500)
		actor.onExpertVotesUpdated(rt, expert2, true)
		actor.expectDisqualifiedExpert(rt, expert2, false, invalidEpoch)
	})

	t.Run("re-activate in 3 days", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1})
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2})

		actor.onExpertVotesUpdated(rt, expert2, true)
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID2})

		// store data
		rt.SetEpoch(100)
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert1: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert1: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  100,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert2: {{CID: pieceID2}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert2: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID2,
					PieceID:    pieceID2,
					PieceSize:  400,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.expectPool(rt, 0, 0, 0, 100, 0, 30)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(20))

		// deactivate expert2 for no enough votes
		rt.SetEpoch(200)
		rt.SetBalance(abi.NewTokenAmount(300))
		actor.onExpertVotesUpdated(rt, expert2, false)
		actor.expectDisqualifiedExpert(rt, expert1, false, invalidEpoch)
		actor.expectDisqualifiedExpert(rt, expert2, true, 200)
		actor.expectPool(rt, 100*1e11, 300, 100, 200, 30, 10)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(false).withDatasize(20).withLockedFunds(200).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(200): abi.NewTokenAmount(200),
			}))

		// activate expert2 in 3 days
		deadline := 200 + expertfund.ClearExpertContributionDelay
		rt.SetEpoch(deadline)
		rt.SetBalance(abi.NewTokenAmount(400))
		actor.onExpertVotesUpdated(rt, expert2, true)
		actor.expectDisqualifiedExpert(rt, expert1, false, invalidEpoch)
		actor.expectDisqualifiedExpert(rt, expert2, false, invalidEpoch)
		actor.expectPool(rt, 200*1e11, 400, 200, int64(deadline), 10, 30)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(20).withRewardDebt(400).withLockedFunds(200).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(200): abi.NewTokenAmount(200),
			}))

		rt.SetEpoch(deadline + 1)
		rt.SetBalance(abi.NewTokenAmount(550))
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(10000), big.Zero())
		actor.expectPool(rt, 250*1e11, 550, int64(deadline), int64(deadline)+1, 30, 30)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10).withRewardDebt(250).withLockedFunds(250).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(deadline + 1): abi.NewTokenAmount(250),
			}))
		actor.claim(rt, expert2, owner, abi.NewTokenAmount(10000), big.Zero())
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(20).withRewardDebt(500).withLockedFunds(300).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(200):          abi.NewTokenAmount(200),
				abi.ChainEpoch(deadline + 1): abi.NewTokenAmount(100),
			}))
	})

	t.Run("re-activate after 3 days", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1})
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2})

		actor.onExpertVotesUpdated(rt, expert2, true)
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID2})

		// store data
		rt.SetEpoch(100)
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert1: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert1: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  100,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert2: {{CID: pieceID2}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert2: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID2,
					PieceID:    pieceID2,
					PieceSize:  400,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.expectPool(rt, 0, 0, 0, 100, 0, 30)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(20))

		// deactivate expert2 for no enough votes
		rt.SetEpoch(200)
		rt.SetBalance(abi.NewTokenAmount(300))
		actor.onExpertVotesUpdated(rt, expert2, false)
		actor.expectDisqualifiedExpert(rt, expert1, false, invalidEpoch)
		actor.expectDisqualifiedExpert(rt, expert2, true, 200)
		actor.expectPool(rt, 100*1e11, 300, 100, 200, 30, 10)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(false).withDatasize(20).withLockedFunds(200).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(200): abi.NewTokenAmount(200),
			}))

		// activate expert2 in 3 days
		afterDeadline := 200 + expertfund.ClearExpertContributionDelay + 1
		rt.SetEpoch(afterDeadline)
		rt.SetBalance(abi.NewTokenAmount(400))
		actor.onExpertVotesUpdated(rt, expert2, true)
		actor.expectDisqualifiedExpert(rt, expert1, false, invalidEpoch)
		actor.expectDisqualifiedExpert(rt, expert2, false, invalidEpoch)
		actor.expectPool(rt, 200*1e11, 400, 200, int64(afterDeadline), 10, 10)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withLockedFunds(200). // RewardDebt cleared
														withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(200): abi.NewTokenAmount(200),
			}))

		rt.SetEpoch(afterDeadline + 1)
		rt.SetBalance(abi.NewTokenAmount(550))
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(10000), big.Zero())
		actor.expectPool(rt, 350*1e11, 550, int64(afterDeadline), int64(afterDeadline)+1, 10, 10)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10).withRewardDebt(350).withLockedFunds(350).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(afterDeadline + 1): abi.NewTokenAmount(350),
			}))
		actor.claim(rt, expert2, owner, abi.NewTokenAmount(10000), big.Zero())
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withLockedFunds(200).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(200): abi.NewTokenAmount(200),
			}))
	})
}

func TestBlockExpert(t *testing.T) {
	owner := tutil.NewIDAddr(t, 100)
	receiver := tutil.NewIDAddr(t, 100)
	expert1 := tutil.NewIDAddr(t, 1000)
	uniqueAddr1 := tutil.NewActorAddr(t, "expert1")
	expert2 := tutil.NewIDAddr(t, 1001)
	uniqueAddr2 := tutil.NewActorAddr(t, "expert2")
	expert3 := tutil.NewIDAddr(t, 1002)
	uniqueAddr3 := tutil.NewActorAddr(t, "expert3")

	pieceID1 := tutil.MakeCID("1", &market.PieceCIDPrefix)
	pieceID2 := tutil.MakeCID("2", &market.PieceCIDPrefix)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		actor := actorHarness{expertfund.Actor{}, t}
		builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
		rt := builder.Build(t)
		actor.constructAndVerify(rt)
		return rt, &actor
	}

	t.Run("burn vesting funds, proposer has enough votes", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1})
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2})
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert3, RobustAddress: uniqueAddr3})

		actor.onExpertVotesUpdated(rt, expert2, true)
		actor.onExpertVotesUpdated(rt, expert3, true)
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID1})
		actor.onExpertImport(rt, expert3, &builtin.CheckedCID{CID: pieceID2})

		// store data
		rt.SetEpoch(100)
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert2: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert2: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  100,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert3: {{CID: pieceID2}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert3: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID2,
					PieceID:    pieceID2,
					PieceSize:  400,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.expectPool(rt, 0, 0, 0, 100, 0, 30)
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(10))
		actor.expectExpert(rt, expert3, newExpectExpertInfo().withActive(true).withDatasize(20))
		actor.expectDisqualifiedExpert(rt, expert2, false, -1)
		actor.expectDisqualifiedExpert(rt, expert3, false, -1)

		// block expert3
		rt.SetEpoch(200)
		rt.SetBalance(abi.NewTokenAmount(300))
		actor.blockExpert(rt, owner, expert3, &expert.OnBlockedReturn{
			ImplicatedExpert:            expert2,
			ImplicatedExpertVotesEnough: true,
		}, abi.NewTokenAmount(200))
		require.True(t, rt.Balance().Equals(abi.NewTokenAmount(100)))
		actor.expectPool(rt, 100*1e11, 100, 100, 200, 30, 10)
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(10))
		actor.expectExpert(rt, expert3, newExpectExpertInfo().withActive(false).withDatasize(20))

		// re-block expert3
		rt.SetEpoch(300)
		actor.blockExpert(rt, owner, expert3, &expert.OnBlockedReturn{
			ImplicatedExpert:            expert2,
			ImplicatedExpertVotesEnough: true,
		}, abi.NewTokenAmount(0))
		require.True(t, rt.Balance().Equals(abi.NewTokenAmount(100)))
		actor.expectPool(rt, 100*1e11, 100, 200, 300, 10, 10)
	})

	t.Run("burn vesting funds, proposer has no enough votes", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1})
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2})
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert3, RobustAddress: uniqueAddr3})

		actor.onExpertVotesUpdated(rt, expert2, true)
		actor.onExpertVotesUpdated(rt, expert3, true)
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID1})
		actor.onExpertImport(rt, expert3, &builtin.CheckedCID{CID: pieceID2})

		// store data
		rt.SetEpoch(100)
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert2: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert2: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  100,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert3: {{CID: pieceID2}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert3: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID2,
					PieceID:    pieceID2,
					PieceSize:  400,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.expectPool(rt, 0, 0, 0, 100, 0, 30)
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(10))
		actor.expectExpert(rt, expert3, newExpectExpertInfo().withActive(true).withDatasize(20))
		actor.expectDisqualifiedExpert(rt, expert2, false, -1)
		actor.expectDisqualifiedExpert(rt, expert3, false, -1)

		// block expert3
		rt.SetEpoch(200)
		rt.SetBalance(abi.NewTokenAmount(300))
		actor.blockExpert(rt, owner, expert3, &expert.OnBlockedReturn{
			ImplicatedExpert:            expert2,
			ImplicatedExpertVotesEnough: false,
		}, abi.NewTokenAmount(200))
		require.True(t, rt.Balance().Equals(abi.NewTokenAmount(100)))
		actor.expectPool(rt, 100*1e11, 100, 100, 200, 30, 0)
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(false).withDatasize(10).withLockedFunds(100).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(200): abi.NewTokenAmount(100),
			}))
		actor.expectExpert(rt, expert3, newExpectExpertInfo().withActive(false).withDatasize(20))
		actor.expectDisqualifiedExpert(rt, expert2, true, 200)
		actor.expectDisqualifiedExpert(rt, expert3, true, 200)
	})

	t.Run("unlocked funds not burned", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1})
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2})

		actor.onExpertVotesUpdated(rt, expert2, true)
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID2})

		// store data
		rt.SetEpoch(100)
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert1: {{CID: pieceID1}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert1: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID1,
					PieceID:    pieceID1,
					PieceSize:  100,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.batchStoreData(rt, &batchStoreDataConf{
			expectStoreDataParams: map[address.Address][]builtin.CheckedCID{
				expert2: {{CID: pieceID2}},
			},
			expectStoreDataReturn: map[address.Address]*expert.GetDatasReturn{
				expert2: {Infos: []*expert.DataOnChainInfo{{
					RootID:     pieceID2,
					PieceID:    pieceID2,
					PieceSize:  400,
					Redundancy: expertfund.DefaultDataStoreThreshold,
				}}},
			},
		})
		actor.expectPool(rt, 0, 0, 0, 100, 0, 30)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(20))
		actor.expectDisqualifiedExpert(rt, expert1, false, -1)
		actor.expectDisqualifiedExpert(rt, expert2, false, -1)

		rt.SetEpoch(200)
		rt.SetBalance(abi.NewTokenAmount(300))
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(10000), big.Zero())
		actor.claim(rt, expert2, owner, abi.NewTokenAmount(10000), big.Zero())

		rt.SetEpoch(200 + expertfund.RewardVestingDelay)
		rt.SetBalance(abi.NewTokenAmount(900))
		actor.claim(rt, expert1, owner, abi.NewTokenAmount(10000), big.Zero())
		actor.claim(rt, expert2, owner, abi.NewTokenAmount(10000), big.Zero())

		actor.expectPool(rt, 300*1e11, 900, 200, int64(200+expertfund.RewardVestingDelay), 30, 30)
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10).
			withLockedFunds(300).withRewardDebt(300).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(200): abi.NewTokenAmount(100),
				abi.ChainEpoch(200 + expertfund.RewardVestingDelay): abi.NewTokenAmount(200),
			}))
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(true).withDatasize(20).
			withLockedFunds(600).withRewardDebt(600).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(200): abi.NewTokenAmount(200),
				abi.ChainEpoch(200 + expertfund.RewardVestingDelay): abi.NewTokenAmount(400),
			}))

		// block expert2
		rt.SetEpoch(200 + expertfund.RewardVestingDelay + 1)
		rt.SetBalance(abi.NewTokenAmount(900))
		actor.blockExpert(rt, owner, expert2, &expert.OnBlockedReturn{
			ImplicatedExpert:            expert1,
			ImplicatedExpertVotesEnough: true,
		}, abi.NewTokenAmount(400))
		require.True(t, rt.Balance().Equals(abi.NewTokenAmount(500)))
		actor.expectPool(rt, 300*1e11, 500, int64(200+expertfund.RewardVestingDelay), int64(200+expertfund.RewardVestingDelay+1), 30, 10)
		actor.expectExpert(rt, expert2, newExpectExpertInfo().withActive(false).withDatasize(20).withUnlockedFunds(200).withRewardDebt(600))
		actor.expectExpert(rt, expert1, newExpectExpertInfo().withActive(true).withDatasize(10).
			withLockedFunds(300).withRewardDebt(300).
			withVestings(map[abi.ChainEpoch]abi.TokenAmount{
				abi.ChainEpoch(200): abi.NewTokenAmount(100),
				abi.ChainEpoch(200 + expertfund.RewardVestingDelay): abi.NewTokenAmount(200),
			}))
	})
}

type actorHarness struct {
	expertfund.Actor
	t testing.TB
}

func (h *actorHarness) constructAndVerify(rt *mock.Runtime) {
	rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	ret := rt.Call(h.Constructor, nil)
	require.Nil(h.t, ret)
	rt.Verify()
}

func (h *actorHarness) applyForExpert(rt *mock.Runtime, owner address.Address, expertType builtin.ExpertType,
	expectExecReturn *initact.ExecReturn) *expertfund.ApplyForExpertReturn {
	rt.SetReceived(abi.NewTokenAmount(99))
	rt.SetBalance(abi.NewTokenAmount(99))
	rt.SetCaller(owner, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)

	ctorParams := expert.ConstructorParams{
		Owner:    owner,
		Proposer: owner,
		Type:     expertType,
	}
	ctorParamBuf := new(bytes.Buffer)
	err := ctorParams.MarshalCBOR(ctorParamBuf)
	require.NoError(h.t, err)
	rt.ExpectSend(builtin.InitActorAddr, builtin.MethodsInit.Exec,
		&initact.ExecParams{
			CodeCID:           builtin.ExpertActorCodeID,
			ConstructorParams: ctorParamBuf.Bytes(),
		}, abi.NewTokenAmount(99), expectExecReturn, exitcode.Ok)

	ret := (rt.Call(h.ApplyForExpert, &expertfund.ApplyForExpertParams{Owner: owner})).(*expertfund.ApplyForExpertReturn)
	rt.Verify()
	return ret
}

func (h *actorHarness) onExpertImport(rt *mock.Runtime, exp address.Address, params *builtin.CheckedCID) {
	rt.SetCaller(exp, builtin.ExpertActorCodeID)
	rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
	rt.Call(h.OnExpertImport, params)
	rt.Verify()
}

func (h *actorHarness) getData(rt *mock.Runtime, expertAddr address.Address,
	params *builtin.CheckedCID, expectDataInfo *expert.DataOnChainInfo) *expertfund.GetDataReturn {
	rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
	rt.ExpectValidateCallerAny()
	rt.ExpectSend(expertAddr, builtin.MethodsExpert.GetDatas,
		&builtin.BatchPieceCIDParams{
			PieceCIDs: []builtin.CheckedCID{*params},
		}, abi.NewTokenAmount(0), &expert.GetDatasReturn{
			Infos: []*expert.DataOnChainInfo{expectDataInfo},
		}, exitcode.Ok)
	ret := rt.Call(h.GetData, params).(*expertfund.GetDataReturn)
	rt.Verify()
	return ret
}

func (h *actorHarness) checkState(rt *mock.Runtime) *expertfund.StateSummary {
	var st expertfund.State
	rt.GetState(&st)
	sum, msgs := expertfund.CheckStateInvariants(&st, rt.AdtStore())
	require.True(h.t, msgs.IsEmpty())
	return sum
}

func (h *actorHarness) blockExpert(rt *mock.Runtime, caller,
	blockedExpert address.Address, expectBlockReturn *expert.OnBlockedReturn, expectBurned abi.TokenAmount) {

	rt.SetCaller(caller, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
	rt.ExpectSend(builtin.GovernActorAddr, builtin.MethodsGovern.ValidateGranted, &builtin.ValidateGrantedParams{
		Caller: caller,
		Method: builtin.MethodsExpertFunds.BlockExpert,
	}, big.Zero(), nil, exitcode.Ok)
	rt.SetAddressActorType(blockedExpert, builtin.ExpertActorCodeID)
	rt.ExpectSend(blockedExpert, builtin.MethodsExpert.OnBlocked, nil, big.Zero(), expectBlockReturn, exitcode.Ok)
	rt.ExpectSend(builtin.VoteFundActorAddr, builtin.MethodsVote.OnCandidateBlocked, &blockedExpert, big.Zero(), nil, exitcode.Ok)
	if expectBurned.GreaterThan(big.Zero()) {
		rt.ExpectSend(builtin.BurntFundsActorAddr, builtin.MethodSend, nil, expectBurned, nil, exitcode.Ok)
	}
	rt.Call(h.BlockExpert, &blockedExpert)
	rt.Verify()
}

func (h *actorHarness) onExpertVotesUpdated(rt *mock.Runtime, expertAddr address.Address, enough bool) {
	rt.SetCaller(builtin.VoteFundActorAddr, builtin.VoteFundActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.VoteFundActorAddr)
	params := &builtin.OnExpertVotesUpdatedParams{
		Expert: expertAddr,
		Votes:  abi.NewTokenAmount(500),
	}
	rt.ExpectSend(expertAddr, builtin.MethodsExpert.OnVotesUpdated, params, big.Zero(), &expert.OnVotesUpdatedReturn{VotesEnough: enough}, exitcode.Ok)
	rt.Call(h.OnExpertVotesUpdated, params)
	rt.Verify()
}

type batchStoreDataConf struct {
	expectStoreDataParams map[address.Address][]builtin.CheckedCID
	expectStoreDataReturn map[address.Address]*expert.GetDatasReturn
}

func (h *actorHarness) batchStoreData(rt *mock.Runtime, conf *batchStoreDataConf) {
	rt.SetCaller(builtin.StorageMarketActorAddr, builtin.StorageMarketActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.StorageMarketActorAddr)
	var params builtin.BatchPieceCIDParams
	for expertAddr, checkedIDs := range conf.expectStoreDataParams {
		params.PieceCIDs = append(params.PieceCIDs, checkedIDs...)
		rt.ExpectSend(expertAddr, builtin.MethodsExpert.StoreData, &builtin.BatchPieceCIDParams{PieceCIDs: checkedIDs},
			big.Zero(), conf.expectStoreDataReturn[expertAddr], exitcode.Ok)
	}

	rt.Call(h.BatchStoreData, &params)
	rt.Verify()
}

func (h *actorHarness) claim(rt *mock.Runtime, expertAddr, owner address.Address, requestedAmount, expectClaimed abi.TokenAmount) {
	rt.SetCaller(owner, builtin.AccountActorCodeID)
	rt.ExpectSend(expertAddr, builtin.MethodsExpert.ControlAddress, nil, big.Zero(), &owner, exitcode.Ok)
	rt.ExpectValidateCallerAddr(owner)
	if !expectClaimed.IsZero() {
		rt.ExpectSend(owner, builtin.MethodSend, nil, expectClaimed, nil, exitcode.Ok)
	}
	rt.Call(h.Claim, &expertfund.ClaimFundParams{
		Expert: expertAddr,
		Amount: requestedAmount,
	})
	rt.Verify()
}

func (h *actorHarness) expectPool(rt *mock.Runtime, accPerShare, lastBalance, prevEpoch, curEpoch, prevTotalSize, curTotalSize int64) {
	st := getState(rt)
	info, err := st.GetPool(adt.AsStore(rt))
	require.NoError(h.t, err)
	require.True(h.t,
		info.AccPerShare.Equals(big.NewInt(accPerShare)) &&
			info.LastRewardBalance.Equals(big.NewInt(lastBalance)) &&
			info.PrevEpoch == abi.ChainEpoch(prevEpoch) &&
			info.PrevTotalDataSize == abi.PaddedPieceSize(prevTotalSize) &&
			info.CurrentEpoch == abi.ChainEpoch(curEpoch) &&
			info.CurrentTotalDataSize == abi.PaddedPieceSize(curTotalSize),
	)
}

type expectExpertInfo struct {
	expertfund.ExpertInfo
	vestings map[abi.ChainEpoch]abi.TokenAmount
}

func newExpectExpertInfo() *expectExpertInfo {
	return &expectExpertInfo{
		expertfund.ExpertInfo{
			Active:        false,
			DataSize:      0,
			RewardDebt:    big.Zero(),
			LockedFunds:   big.Zero(),
			UnlockedFunds: big.Zero(),
		},
		make(map[abi.ChainEpoch]big.Int),
	}
}
func (e *expectExpertInfo) withActive(active bool) *expectExpertInfo {
	e.Active = active
	return e
}
func (e *expectExpertInfo) withDatasize(size int64) *expectExpertInfo {
	e.DataSize = abi.PaddedPieceSize(size)
	return e
}
func (e *expectExpertInfo) withRewardDebt(v int64) *expectExpertInfo {
	e.RewardDebt = abi.NewTokenAmount(v)
	return e
}
func (e *expectExpertInfo) withLockedFunds(v int64) *expectExpertInfo {
	e.LockedFunds = abi.NewTokenAmount(v)
	return e
}
func (e *expectExpertInfo) withUnlockedFunds(v int64) *expectExpertInfo {
	e.UnlockedFunds = abi.NewTokenAmount(v)
	return e
}
func (e *expectExpertInfo) withVestings(v map[abi.ChainEpoch]abi.TokenAmount) *expectExpertInfo {
	e.vestings = v
	return e
}

func (h *actorHarness) expectExpert(rt *mock.Runtime, expertAddr address.Address, expect *expectExpertInfo) {
	st := getState(rt)
	info, err := st.GetExpert(adt.AsStore(rt), expertAddr)
	require.NoError(h.t, err)
	fmt.Printf("%+v\n", info)
	require.True(h.t,
		info.Active == expect.Active &&
			info.DataSize == expect.DataSize &&
			info.RewardDebt.Equals(expect.RewardDebt) &&
			info.LockedFunds.Equals(expect.LockedFunds) &&
			info.UnlockedFunds.Equals(expect.UnlockedFunds),
	)
	vestings, err := adt.AsMap(adt.AsStore(rt), info.VestingFunds, builtin.DefaultHamtBitwidth)
	require.NoError(h.t, err)
	count := 0
	var amount abi.TokenAmount
	err = vestings.ForEach(&amount, func(k string) error {
		epoch, err := abi.ParseIntKey(k)
		require.NoError(h.t, err)
		require.True(h.t, expect.vestings[abi.ChainEpoch(epoch)].Equals(amount))
		count++
		return nil
	})
	require.True(h.t, count == len(expect.vestings))
}

func (h *actorHarness) expectDisqualifiedExpert(rt *mock.Runtime, expertAddr address.Address, expectFound bool, expectDisqualifiedAt abi.ChainEpoch) {
	st := getState(rt)
	info, found, err := st.GetDisqualifiedExpertInfo(adt.AsStore(rt), expertAddr)
	require.NoError(h.t, err)
	if expectFound {
		require.True(h.t, found && info.DisqualifiedAt == expectDisqualifiedAt)
	} else {
		require.True(h.t, !found)
	}
}
