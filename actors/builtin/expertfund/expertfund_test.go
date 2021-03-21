package expertfund_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	expert "github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expertfund"
	initact "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
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
		require.True(t, sum.ExpertsCount == 1 && st.ExpertsCount == 1 && ret1.IDAddress == expert1)
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
		require.True(t, sum.ExpertsCount == 2 && st.ExpertsCount == 2 && ret2.IDAddress == expert2)
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
		actor.onExpertImport(rt, expert1, &builtin.OnExpertImportParams{PieceID: pieceID1})
		st := getState(rt)
		expertAddr, found, err := st.GetData(adt.AsStore(rt), pieceID1.String())
		require.NoError(t, err)
		require.True(t, found && expertAddr == expert1)

		// piece 2
		pieceID2 := tutil.MakeCID("2", &miner.SealedCIDPrefix)
		actor.onExpertImport(rt, expert2, &builtin.OnExpertImportParams{PieceID: pieceID2})
		st = getState(rt)
		expertAddr2, found, err := st.GetData(adt.AsStore(rt), pieceID2.String())
		require.NoError(t, err)
		require.True(t, found && expertAddr2 == expert2)

		// not found
		_, found, err = st.GetData(adt.AsStore(rt), tutil.MakeCID("3", &miner.SealedCIDPrefix).String())
		require.NoError(t, err)
		require.True(t, !found)

		// re-put piece 1 with expert 1
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "put duplicate data", func() {
			actor.onExpertImport(rt, expert1, &builtin.OnExpertImportParams{PieceID: pieceID1})
		})
		// re-put piece 1 with expert 2
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "put duplicate data", func() {
			actor.onExpertImport(rt, expert2, &builtin.OnExpertImportParams{PieceID: pieceID1})
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

		pieceID := tutil.MakeCID("1", &miner.SealedCIDPrefix)
		actor.onExpertImport(rt, expert1, &builtin.OnExpertImportParams{PieceID: pieceID})

		di := actor.getData(rt, expert1, &expertfund.GetDataParams{PieceID: pieceID}, &expert.DataOnChainInfo{PieceID: pieceID.String()})
		require.True(t, di.Expert == expert1 && di.Data.PieceID == pieceID.String())
	})

	t.Run("get nonexistent data", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		pieceID := tutil.MakeCID("1", &miner.SealedCIDPrefix)

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "data not found", func() {
			actor.getData(rt, expert1, &expertfund.GetDataParams{PieceID: pieceID}, &expert.DataOnChainInfo{PieceID: pieceID.String()})
		})
	})

	t.Run("call expert actor error", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		pieceID := tutil.MakeCID("1", &miner.SealedCIDPrefix)
		actor.onExpertImport(rt, expert1, &builtin.OnExpertImportParams{PieceID: pieceID})

		rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
		rt.ExpectValidateCallerAny()
		rt.ExpectSend(expert1, builtin.MethodsExpert.GetData, &expert.ExpertDataParams{PieceID: pieceID}, abi.NewTokenAmount(0), &expert.DataOnChainInfo{PieceID: pieceID.String()}, exitcode.ErrForbidden)

		rt.ExpectAbort(exitcode.ErrForbidden, func() {
			rt.Call(actor.GetData, &expertfund.GetDataParams{PieceID: pieceID})
		})
	})
}

func TestAddTrackedExpert(t *testing.T) {
	receiver := tutil.NewIDAddr(t, 100)
	expert1 := tutil.NewIDAddr(t, 1000)
	expert2 := tutil.NewIDAddr(t, 1001)

	actor := actorHarness{expertfund.Actor{}, t}
	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("add tracked expert", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		// add expert1
		actor.addTrackedExpert(rt, expert1)
		st := getState(rt)
		experts, err := st.ListTrackedExperts(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, len(experts) == 1 && experts[0] == expert1)

		// add expert2
		actor.addTrackedExpert(rt, expert2)
		st = getState(rt)
		experts, err = st.ListTrackedExperts(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, len(experts) == 2 &&
			(experts[0] == expert1 || experts[1] == expert2) ||
			(experts[0] == expert2 || experts[1] == expert1))

		actor.checkState(rt)
	})

	t.Run("nothing adding duplicate", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		st := getState(rt)
		experts, err := st.ListTrackedExperts(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, len(experts) == 0)

		// add expert1
		actor.addTrackedExpert(rt, expert1)
		// re-add expert1
		actor.addTrackedExpert(rt, expert1)
		st = getState(rt)
		experts, err = st.ListTrackedExperts(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, len(experts) == 1 && experts[0] == expert1)
	})
}

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

func (h *actorHarness) addTrackedExpert(rt *mock.Runtime, exp address.Address) {
	rt.SetCaller(exp, builtin.ExpertActorCodeID)
	rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
	rt.Call(h.AddTrackedExpert, nil)
	rt.Verify()
}

func (h *actorHarness) onExpertImport(rt *mock.Runtime, exp address.Address, params *builtin.OnExpertImportParams) {
	rt.SetCaller(exp, builtin.ExpertActorCodeID)
	rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
	rt.Call(h.OnExpertImport, params)
	rt.Verify()
}

func (h *actorHarness) getData(rt *mock.Runtime, expertAddr address.Address,
	params *expertfund.GetDataParams, expectDataInfo *expert.DataOnChainInfo) *expertfund.DataInfo {
	rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
	rt.ExpectValidateCallerAny()
	rt.ExpectSend(expertAddr, builtin.MethodsExpert.GetData, &expert.ExpertDataParams{PieceID: params.PieceID}, abi.NewTokenAmount(0), expectDataInfo, exitcode.Ok)
	ret := rt.Call(h.GetData, params).(*expertfund.DataInfo)
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
