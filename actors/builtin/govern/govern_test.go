package govern_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/govern"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/filecoin-project/specs-actors/v2/support/mock"
	tutil "github.com/filecoin-project/specs-actors/v2/support/testing"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, govern.Actor{})
}

func TestConstruction(t *testing.T) {

	actor := govern.Actor{}
	builder := mock.NewBuilder(context.Background(), builtin.GovernActorAddr).
		WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("fallback not set", func(t *testing.T) {
		rt := builder.Build(t)
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalState, "supervisor address must be an ID address", func() {
			rt.Call(actor.Constructor, &address.Undef)
		})
	})

	t.Run("simple construction", func(t *testing.T) {

		fb := tutil.NewIDAddr(t, 101)

		rt := builder.Build(t)
		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
		ret := rt.Call(actor.Constructor, &fb)
		assert.Nil(t, ret)
		rt.Verify()
	})
}

func TestGrant(t *testing.T) {
	votefund := tutil.NewIDAddr(t, 80)
	caller := tutil.NewIDAddr(t, 100)
	super := tutil.NewIDAddr(t, 101)
	governor := tutil.NewIDAddr(t, 102)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), builtin.VoteFundActorAddr).
			WithActorType(votefund, builtin.VoteFundActorCodeID).
			WithActorType(super, builtin.AccountActorCodeID)
		rt := builder.Build(t)

		actor := newHarness(t, super)
		actor.constructAndVerify(rt)

		return rt, actor
	}

	t.Run("fail when illegal governor", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "failed to resovle governor", func() {
			rt.Call(actor.Grant, &govern.GrantOrRevokeParams{Governor: address.Undef})
		})
	})

	t.Run("fail when empty codes", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "no priviledge to grant", func() {
			rt.Call(actor.Grant, &govern.GrantOrRevokeParams{Governor: governor})
		})
	})

	t.Run("fail when empty methods", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetAddressActorType(governor, builtin.AccountActorCodeID)
		rt.SetCaller(super, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(super)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "no priviledge to grant", func() {
			rt.Call(actor.Grant, &govern.GrantOrRevokeParams{
				Governor: governor,
				Authorities: []govern.Authority{
					{ActorCodeID: builtin.StorageMarketActorCodeID, Methods: []abi.MethodNum{}},
					{ActorCodeID: builtin.KnowledgeFundActorCodeID, Methods: []abi.MethodNum{}},
				}})
		})
	})

	t.Run("fail when illegal governor code", func(t *testing.T) {
		rt, actor := setupFunc()
		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "failed to check actor code", func() {
			rt.Call(actor.Grant, &govern.GrantOrRevokeParams{Governor: votefund, All: true})
		})
	})

	t.Run("fail when caller not supervisor", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetAddressActorType(governor, builtin.AccountActorCodeID)

		rt.SetCaller(caller, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(super)
		rt.ExpectAbort(exitcode.SysErrForbidden, func() {
			rt.Call(actor.Grant, &govern.GrantOrRevokeParams{Governor: governor, All: true})
		})
	})

	t.Run("fail when duplicate code", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetAddressActorType(governor, builtin.AccountActorCodeID)
		rt.SetCaller(super, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(super)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "duplicated actor code", func() {
			rt.Call(actor.Grant, &govern.GrantOrRevokeParams{
				Governor: governor,
				Authorities: []govern.Authority{
					{ActorCodeID: builtin.StorageMarketActorCodeID, Methods: []abi.MethodNum{builtin.MethodsMarket.ResetQuotas}},
					{ActorCodeID: builtin.StorageMarketActorCodeID, Methods: []abi.MethodNum{}},
				}})
		})
	})

	t.Run("fail when duplicate methods", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetAddressActorType(governor, builtin.AccountActorCodeID)
		rt.SetCaller(super, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(super)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, "duplicated method", func() {
			rt.Call(actor.Grant, &govern.GrantOrRevokeParams{
				Governor: governor,
				Authorities: []govern.Authority{
					{ActorCodeID: builtin.StorageMarketActorCodeID, Methods: []abi.MethodNum{builtin.MethodsMarket.ResetQuotas, builtin.MethodsMarket.ResetQuotas}},
					{ActorCodeID: builtin.KnowledgeFundActorCodeID, Methods: []abi.MethodNum{}},
				}})
		})
	})

	t.Run("fail when actor not found", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetAddressActorType(governor, builtin.AccountActorCodeID)
		rt.SetCaller(super, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(super)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, fmt.Sprintf("actor code %s not found", builtin.RewardActorCodeID), func() {
			rt.Call(actor.Grant, &govern.GrantOrRevokeParams{
				Governor: governor,
				Authorities: []govern.Authority{
					{ActorCodeID: builtin.RewardActorCodeID, Methods: []abi.MethodNum{builtin.MethodsReward.AwardBlockReward}},
				}})
		})
	})

	t.Run("fail when method not found", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetAddressActorType(governor, builtin.AccountActorCodeID)
		rt.SetCaller(super, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(super)
		rt.ExpectAbortContainsMessage(exitcode.ErrIllegalArgument, fmt.Sprintf("method %d of actor code %s not found", builtin.MethodsVote.Vote, builtin.ExpertFundActorCodeID), func() {
			rt.Call(actor.Grant, &govern.GrantOrRevokeParams{
				Governor: governor,
				Authorities: []govern.Authority{
					{ActorCodeID: builtin.ExpertFundActorCodeID, Methods: []abi.MethodNum{builtin.MethodsVote.Vote}},
				}})
		})
	})

	t.Run("grant all actors", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetAddressActorType(governor, builtin.AccountActorCodeID)
		rt.SetCaller(super, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(super)
		rt.Call(actor.Grant, &govern.GrantOrRevokeParams{Governor: governor, All: true})
		rt.Verify()

		actor.checkAuthorities(rt, authConf{
			expectAuthorities: map[address.Address]map[cid.Cid][]abi.MethodNum{
				governor: {
					builtin.ExpertActorCodeID:        {builtin.MethodsExpert.GovChangeOwner},
					builtin.ExpertFundActorCodeID:    {builtin.MethodsExpertFunds.ChangeThreshold, builtin.MethodsExpertFunds.BlockExpert},
					builtin.KnowledgeFundActorCodeID: {builtin.MethodsKnowledge.ChangePayee},
					builtin.StorageMarketActorCodeID: {builtin.MethodsMarket.ResetQuotas, builtin.MethodsMarket.SetInitialQuota},
					builtin.StoragePowerActorCodeID:  {builtin.MethodsPower.ChangeWdPoStRatio},
				},
			},
		})
	})

	t.Run("grant all methods of actor", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetAddressActorType(governor, builtin.AccountActorCodeID)
		rt.SetCaller(super, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(super)
		rt.Call(actor.Grant, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.ExpertActorCodeID, All: true},
			},
		})
		rt.Verify()

		actor.checkAuthorities(rt, authConf{
			expectAuthorities: map[address.Address]map[cid.Cid][]abi.MethodNum{
				governor: {
					builtin.ExpertActorCodeID: {builtin.MethodsExpert.GovChangeOwner},
				},
			},
		})
	})

	t.Run("re-grant actor all", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetAddressActorType(governor, builtin.AccountActorCodeID)
		actor.grant(rt, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.ExpertActorCodeID, Methods: []abi.MethodNum{builtin.MethodsExpert.GovChangeOwner}},
			},
		})
		actor.grant(rt, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.ExpertActorCodeID, All: true},
			},
		})

		actor.checkAuthorities(rt, authConf{
			expectAuthorities: map[address.Address]map[cid.Cid][]abi.MethodNum{
				governor: {
					builtin.ExpertActorCodeID: {builtin.MethodsExpert.GovChangeOwner},
				},
			},
		})
	})

	t.Run("re-grant all", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetAddressActorType(governor, builtin.AccountActorCodeID)
		actor.grant(rt, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.ExpertActorCodeID, Methods: []abi.MethodNum{builtin.MethodsExpert.GovChangeOwner}},
			},
		})
		actor.grant(rt, &govern.GrantOrRevokeParams{
			Governor: governor,
			All:      true,
		})

		actor.checkAuthorities(rt, authConf{
			expectAuthorities: map[address.Address]map[cid.Cid][]abi.MethodNum{
				governor: {
					builtin.ExpertActorCodeID:        {builtin.MethodsExpert.GovChangeOwner},
					builtin.ExpertFundActorCodeID:    {builtin.MethodsExpertFunds.ChangeThreshold, builtin.MethodsExpertFunds.BlockExpert},
					builtin.KnowledgeFundActorCodeID: {builtin.MethodsKnowledge.ChangePayee},
					builtin.StorageMarketActorCodeID: {builtin.MethodsMarket.ResetQuotas, builtin.MethodsMarket.SetInitialQuota},
					builtin.StoragePowerActorCodeID:  {builtin.MethodsPower.ChangeWdPoStRatio},
				},
			},
		})
	})

	t.Run("grant mix", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetAddressActorType(governor, builtin.AccountActorCodeID)
		rt.SetCaller(super, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerAddr(super)
		rt.Call(actor.Grant, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.ExpertFundActorCodeID, Methods: []abi.MethodNum{builtin.MethodsExpertFunds.BlockExpert}},
				{ActorCodeID: builtin.KnowledgeFundActorCodeID, All: true},
			},
		})
		rt.Verify()

		actor.checkAuthorities(rt, authConf{
			expectAuthorities: map[address.Address]map[cid.Cid][]abi.MethodNum{
				governor: {
					builtin.ExpertFundActorCodeID:    {builtin.MethodsExpertFunds.BlockExpert},
					builtin.KnowledgeFundActorCodeID: {builtin.MethodsKnowledge.ChangePayee},
				},
			},
		})
	})

	t.Run("grant muliti governors", func(t *testing.T) {
		rt, actor := setupFunc()
		governor2 := tutil.NewIDAddr(t, 300)

		rt.SetAddressActorType(governor, builtin.AccountActorCodeID)
		rt.SetAddressActorType(governor2, builtin.MultisigActorCodeID)

		actor.grant(rt, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.ExpertActorCodeID, Methods: []abi.MethodNum{builtin.MethodsExpert.GovChangeOwner}},
			},
		})
		actor.grant(rt, &govern.GrantOrRevokeParams{
			Governor: governor2,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.KnowledgeFundActorCodeID, All: true},
			},
		})

		actor.checkAuthorities(rt, authConf{
			expectAuthorities: map[address.Address]map[cid.Cid][]abi.MethodNum{
				governor: {
					builtin.ExpertActorCodeID: {builtin.MethodsExpert.GovChangeOwner},
				},
				governor2: {
					builtin.KnowledgeFundActorCodeID: {builtin.MethodsKnowledge.ChangePayee},
				},
			},
		})
	})
}

func TestRevoke(t *testing.T) {
	super := tutil.NewIDAddr(t, 100)
	governor := tutil.NewIDAddr(t, 101)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), builtin.VoteFundActorAddr).
			WithActorType(governor, builtin.MultisigActorCodeID).
			WithActorType(super, builtin.AccountActorCodeID)
		rt := builder.Build(t)

		actor := newHarness(t, super)
		actor.constructAndVerify(rt)

		return rt, actor
	}

	t.Run("grant partial, revoke all", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.grant(rt, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.ExpertActorCodeID, Methods: []abi.MethodNum{builtin.MethodsExpert.GovChangeOwner}},
				{ActorCodeID: builtin.KnowledgeFundActorCodeID, All: true},
			},
		})
		actor.revoke(rt, &govern.GrantOrRevokeParams{Governor: governor, All: true})

		actor.checkAuthorities(rt, authConf{
			expectAuthorities: map[address.Address]map[cid.Cid][]abi.MethodNum{},
		})
	})

	t.Run("grant all, revoke all", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.grant(rt, &govern.GrantOrRevokeParams{Governor: governor, All: true})
		actor.revoke(rt, &govern.GrantOrRevokeParams{Governor: governor, All: true})

		actor.checkAuthorities(rt, authConf{
			expectAuthorities: map[address.Address]map[cid.Cid][]abi.MethodNum{},
		})
	})

	t.Run("revoke partial", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.grant(rt, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.ExpertActorCodeID, Methods: []abi.MethodNum{builtin.MethodsExpert.GovChangeOwner}},
				{ActorCodeID: builtin.KnowledgeFundActorCodeID, All: true},
			},
		})
		actor.revoke(rt, &govern.GrantOrRevokeParams{Governor: governor, Authorities: []govern.Authority{
			{ActorCodeID: builtin.KnowledgeFundActorCodeID, Methods: []abi.MethodNum{builtin.MethodsKnowledge.ChangePayee}},
		}})

		actor.checkAuthorities(rt, authConf{
			expectAuthorities: map[address.Address]map[cid.Cid][]abi.MethodNum{
				governor: {
					builtin.ExpertActorCodeID: {builtin.MethodsExpert.GovChangeOwner},
				},
			},
		})
	})

	t.Run("revoke ungranted", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.grant(rt, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.ExpertActorCodeID, Methods: []abi.MethodNum{builtin.MethodsExpert.GovChangeOwner}},
				{ActorCodeID: builtin.KnowledgeFundActorCodeID, All: true},
			},
		})
		// all ungranted
		actor.revoke(rt, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.ExpertFundActorCodeID, Methods: []abi.MethodNum{builtin.MethodsExpertFunds.ChangeThreshold}},
			},
		})
		actor.checkAuthorities(rt, authConf{
			expectAuthorities: map[address.Address]map[cid.Cid][]abi.MethodNum{
				governor: {
					builtin.ExpertActorCodeID:        {builtin.MethodsExpert.GovChangeOwner},
					builtin.KnowledgeFundActorCodeID: {builtin.MethodsKnowledge.ChangePayee},
				},
			},
		})

		// partial ungranted
		actor.revoke(rt, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.ExpertActorCodeID, Methods: []abi.MethodNum{builtin.MethodsExpert.GovChangeOwner}},
				{ActorCodeID: builtin.ExpertFundActorCodeID, Methods: []abi.MethodNum{builtin.MethodsExpertFunds.ChangeThreshold}},
			},
		})
		actor.checkAuthorities(rt, authConf{
			expectAuthorities: map[address.Address]map[cid.Cid][]abi.MethodNum{
				governor: {
					builtin.KnowledgeFundActorCodeID: {builtin.MethodsKnowledge.ChangePayee},
				},
			},
		})
	})
}

func TestValidateGranted(t *testing.T) {
	super := tutil.NewIDAddr(t, 100)
	governor := tutil.NewIDAddr(t, 101)

	setupFunc := func() (*mock.Runtime, *actorHarness) {
		builder := mock.NewBuilder(context.Background(), builtin.VoteFundActorAddr).
			WithActorType(governor, builtin.MultisigActorCodeID).
			WithActorType(super, builtin.AccountActorCodeID)
		rt := builder.Build(t)

		actor := newHarness(t, super)
		actor.constructAndVerify(rt)

		return rt, actor
	}

	t.Run("fail when caller is ungranted", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetCaller(builtin.RewardActorAddr, builtin.RewardActorCodeID)
		rt.ExpectValidateCallerType(govern.GovernedCallerTypes...)
		rt.ExpectAbort(exitcode.SysErrForbidden, func() {
			rt.Call(actor.ValidateGranted, &builtin.ValidateGrantedParams{})
		})
	})

	t.Run("actor not granted", func(t *testing.T) {
		rt, actor := setupFunc()

		rt.SetCaller(builtin.KnowledgeFundActorAddr, builtin.KnowledgeFundActorCodeID)
		rt.ExpectValidateCallerType(govern.GovernedCallerTypes...)
		rt.ExpectAbortContainsMessage(exitcode.ErrForbidden, "method not granted", func() {
			rt.Call(actor.ValidateGranted, &builtin.ValidateGrantedParams{
				Caller: governor,
				Method: builtin.MethodsKnowledge.ChangePayee,
			})
		})
	})

	t.Run("method not granted", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.grant(rt, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.StorageMarketActorCodeID, Methods: []abi.MethodNum{builtin.MethodsMarket.ResetQuotas}},
			},
		})

		rt.SetCaller(builtin.StorageMarketActorAddr, builtin.StorageMarketActorCodeID)
		rt.ExpectValidateCallerType(govern.GovernedCallerTypes...)
		rt.ExpectAbortContainsMessage(exitcode.ErrForbidden, "method not granted", func() {
			rt.Call(actor.ValidateGranted, &builtin.ValidateGrantedParams{
				Caller: governor,
				Method: builtin.MethodsMarket.SetInitialQuota,
			})
		})
	})

	t.Run("granted", func(t *testing.T) {
		rt, actor := setupFunc()

		actor.grant(rt, &govern.GrantOrRevokeParams{
			Governor: governor,
			Authorities: []govern.Authority{
				{ActorCodeID: builtin.StorageMarketActorCodeID, Methods: []abi.MethodNum{builtin.MethodsMarket.ResetQuotas}},
			},
		})

		rt.SetCaller(builtin.StorageMarketActorAddr, builtin.StorageMarketActorCodeID)
		rt.ExpectValidateCallerType(govern.GovernedCallerTypes...)
		rt.Call(actor.ValidateGranted, &builtin.ValidateGrantedParams{
			Caller: governor,
			Method: builtin.MethodsMarket.ResetQuotas,
		})
	})
}

type actorHarness struct {
	govern.Actor
	t *testing.T

	supervisor address.Address
}

func newHarness(t *testing.T, supervisor address.Address) *actorHarness {
	assert.NotEqual(t, supervisor, address.Undef)
	return &actorHarness{
		Actor:      govern.Actor{},
		t:          t,
		supervisor: supervisor,
	}
}

func (h *actorHarness) constructAndVerify(rt *mock.Runtime) {
	rt.SetCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
	rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	ret := rt.Call(h.Actor.Constructor, &h.supervisor)
	assert.Nil(h.t, ret)
	rt.Verify()

	var st govern.State
	rt.GetState(&st)

	assert.Equal(h.t, st.Supervisor, h.supervisor)

	governors, err := adt.AsMap(adt.AsStore(rt), st.Governors, builtin.DefaultHamtBitwidth)
	assert.NoError(h.t, err)
	keys, err := governors.CollectKeys()
	require.NoError(h.t, err)
	assert.Empty(h.t, keys)
}

func (h *actorHarness) grant(rt *mock.Runtime, params *govern.GrantOrRevokeParams) {
	rt.SetCaller(h.supervisor, builtin.MultisigActorCodeID)
	rt.ExpectValidateCallerAddr(h.supervisor)
	rt.Call(h.Grant, params)
	rt.Verify()
}

func (h *actorHarness) revoke(rt *mock.Runtime, params *govern.GrantOrRevokeParams) {
	rt.SetCaller(h.supervisor, builtin.MultisigActorCodeID)
	rt.ExpectValidateCallerAddr(h.supervisor)
	rt.Call(h.Revoke, params)
	rt.Verify()
}

type authConf struct {
	// Governor -> ActorCodeID -> Methods
	expectAuthorities map[address.Address]map[cid.Cid][]abi.MethodNum
}

func (h *actorHarness) checkAuthorities(rt *mock.Runtime, conf authConf) {
	var st govern.State
	rt.GetState(&st)
	store := adt.AsStore(rt)

	governors, err := adt.AsMap(store, st.Governors, builtin.DefaultHamtBitwidth)
	require.NoError(h.t, err)

	keys, err := governors.CollectKeys()
	require.NoError(h.t, err)
	require.True(h.t, len(keys) == len(conf.expectAuthorities))

	var ga govern.GrantedAuthorities
	err = governors.ForEach(&ga, func(k string) error {
		governor, err := address.NewFromBytes([]byte(k))
		require.NoError(h.t, err)

		actorToMethods, ok := conf.expectAuthorities[governor]
		require.True(h.t, ok)

		cms, err := adt.AsMap(store, ga.CodeMethods, builtin.DefaultHamtBitwidth)
		require.NoError(h.t, err)
		cmskeys, err := cms.CollectKeys()
		require.NoError(h.t, err)
		require.True(h.t, len(cmskeys) == len(actorToMethods))

		for code, methods := range actorToMethods {
			var bf bitfield.BitField
			found, err := cms.Get(abi.CidKey(code), &bf)
			require.NoError(h.t, err)
			cnt, err := bf.Count()
			require.NoError(h.t, err)
			require.True(h.t, found && int(cnt) == len(methods))

			for _, method := range methods {
				set, err := bf.IsSet(uint64(method))
				require.NoError(h.t, err)
				require.True(h.t, set, method)
			}
		}
		return nil
	})
	require.NoError(h.t, err)
}
