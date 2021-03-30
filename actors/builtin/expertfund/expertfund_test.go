package expertfund_test

import (
	"bytes"
	"context"
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
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})
		st := getState(rt)
		dataInfo, err := st.GetDataInfos(adt.AsStore(rt), pieceID1)
		require.NoError(t, err)
		require.True(t, dataInfo[0].Expert == expert1)

		// piece 2
		pieceID2 := tutil.MakeCID("2", &miner.SealedCIDPrefix)
		actor.onExpertImport(rt, expert2, &builtin.CheckedCID{CID: pieceID2})
		st = getState(rt)
		dataInfo2, err := st.GetDataInfos(adt.AsStore(rt), pieceID2)
		require.NoError(t, err)
		require.True(t, dataInfo2[0].Expert == expert2)

		// not found
		_, err = st.GetDataInfos(adt.AsStore(rt), tutil.MakeCID("3", &miner.SealedCIDPrefix))
		require.True(t, strings.Contains(err.Error(), "DataInfo not found"))

		// re-put piece 1 with expert 1
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "duplicate imported data", func() {
			actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID1})
		})
		// re-put piece 1 with expert 2
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "duplicate imported data", func() {
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

		pieceID := tutil.MakeCID("1", &miner.SealedCIDPrefix)
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID})

		di := actor.getData(rt, expert1, &builtin.CheckedCID{CID: pieceID}, &expert.DataOnChainInfo{PieceID: pieceID.String()})
		require.True(t, di.Expert == expert1 && di.Data.PieceID == pieceID.String())
	})

	t.Run("get nonexistent data", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		pieceID := tutil.MakeCID("1", &miner.SealedCIDPrefix)

		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "DataInfo not found", func() {
			actor.getData(rt, expert1, &builtin.CheckedCID{CID: pieceID}, &expert.DataOnChainInfo{PieceID: pieceID.String()})
		})
	})

	t.Run("call expert actor error", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		pieceID := tutil.MakeCID("1", &miner.SealedCIDPrefix)
		actor.onExpertImport(rt, expert1, &builtin.CheckedCID{CID: pieceID})

		rt.SetCaller(builtin.ExpertFundActorAddr, builtin.ExpertFundActorCodeID)
		rt.ExpectValidateCallerAny()
		rt.ExpectSend(expert1, builtin.MethodsExpert.GetData, &builtin.CheckedCID{CID: pieceID}, abi.NewTokenAmount(0), &expert.DataOnChainInfo{PieceID: pieceID.String()}, exitcode.ErrForbidden)

		rt.ExpectAbort(exitcode.ErrForbidden, func() {
			rt.Call(actor.GetData, &builtin.CheckedCID{CID: pieceID})
		})
	})
}

func TestOnExpertNominated(t *testing.T) {
	receiver := tutil.NewIDAddr(t, 100)
	owner := tutil.NewIDAddr(t, 101)
	expert1 := tutil.NewIDAddr(t, 1000)
	expert2 := tutil.NewIDAddr(t, 1001)
	uniqueAddr1 := tutil.NewActorAddr(t, "expert1")
	uniqueAddr2 := tutil.NewActorAddr(t, "expert2")

	actor := actorHarness{expertfund.Actor{}, t}
	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("add tracked expert", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		expectExecRet := &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1}
		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, expectExecRet)

		expectExecRet = &initact.ExecReturn{IDAddress: expert2, RobustAddress: uniqueAddr2}
		actor.applyForExpert(rt, owner, builtin.ExpertNormal, expectExecRet)

		// add expert1
		actor.onExpertNominated(rt, expert1)
		st := getState(rt)
		experts, err := st.ListTrackedExperts(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, len(experts) == 1 && experts[0] == expert1)

		// add expert2
		actor.onExpertNominated(rt, expert2)
		st = getState(rt)
		experts, err = st.ListTrackedExperts(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, len(experts) == 2 &&
			(experts[0] == expert1 || experts[1] == expert2) ||
			(experts[0] == expert2 || experts[1] == expert1))

		actor.checkState(rt)
	})

	t.Run("fail adding duplicate", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		st := getState(rt)
		experts, err := st.ListTrackedExperts(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, len(experts) == 0)

		expectExecRet := &initact.ExecReturn{IDAddress: expert1, RobustAddress: uniqueAddr1}
		actor.applyForExpert(rt, owner, builtin.ExpertFoundation, expectExecRet)

		// add expert1
		actor.onExpertNominated(rt, expert1)
		st = getState(rt)
		experts, err = st.ListTrackedExperts(adt.AsStore(rt))
		require.NoError(t, err)
		require.True(t, len(experts) == 1 && experts[0] == expert1)

		// re-add expert1
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "expert already activated", func() {
			actor.onExpertNominated(rt, expert1)
		})
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

func (h *actorHarness) onExpertNominated(rt *mock.Runtime, exp address.Address) {
	rt.SetCaller(exp, builtin.ExpertActorCodeID)
	rt.ExpectValidateCallerType(builtin.ExpertActorCodeID)
	rt.Call(h.OnExpertNominated, nil)
	rt.Verify()
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
	rt.ExpectSend(expertAddr, builtin.MethodsExpert.GetData, &builtin.CheckedCID{CID: params.CID}, abi.NewTokenAmount(0), expectDataInfo, exitcode.Ok)
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
