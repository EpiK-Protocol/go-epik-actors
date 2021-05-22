package retrieval_test

import (
	"context"
	"testing"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	builtin "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/retrieval"
	mock "github.com/filecoin-project/specs-actors/v2/support/mock"
	tutil "github.com/filecoin-project/specs-actors/v2/support/testing"
	"github.com/stretchr/testify/require"
)

func getState(rt *mock.Runtime) *retrieval.State {
	var st retrieval.State
	rt.GetState(&st)
	return &st
}

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, retrieval.Actor{})
}

func TestConstruction(t *testing.T) {
	actor := actorHarness{retrieval.Actor{}, t}
	receiver := tutil.NewIDAddr(t, 100)
	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("construction", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)
	})
}

func TestActorPledge(t *testing.T) {
	client := tutil.NewIDAddr(t, 101)
	miner := tutil.NewIDAddr(t, 102)
	target := tutil.NewIDAddr(t, 103)

	actor := actorHarness{retrieval.Actor{}, t}
	receiver := tutil.NewIDAddr(t, 100)
	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("pledge with no miner", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		rt.SetAddressActorType(client, builtin.AccountActorCodeID)
		rt.SetAddressActorType(miner, builtin.StorageMinerActorCodeID)
		rt.SetAddressActorType(target, builtin.AccountActorCodeID)

		amount := big.Mul(big.NewInt(2), builtin.TokenPrecision)
		actor.pledge(rt, client, amount, &retrieval.PledgeParams{
			Address: target,
		})
	})

	t.Run("pledge with miner", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		rt.SetAddressActorType(client, builtin.AccountActorCodeID)
		rt.SetAddressActorType(miner, builtin.StorageMinerActorCodeID)
		rt.SetAddressActorType(target, builtin.AccountActorCodeID)

		amount := big.Mul(big.NewInt(2), builtin.TokenPrecision)
		actor.pledge(rt, client, amount, &retrieval.PledgeParams{
			Address: target,
			Miners:  []address.Address{miner},
		})
	})

	t.Run("apply withdraw", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		rt.SetAddressActorType(client, builtin.AccountActorCodeID)
		rt.SetAddressActorType(miner, builtin.StorageMinerActorCodeID)
		rt.SetAddressActorType(target, builtin.AccountActorCodeID)

		amount := big.Mul(big.NewInt(2), builtin.TokenPrecision)
		actor.pledge(rt, client, amount, &retrieval.PledgeParams{
			Address: target,
			Miners:  []address.Address{miner},
		})

		actor.applyForWithdraw(rt, client, &retrieval.WithdrawBalanceParams{
			ProviderOrClientAddress: client,
			Amount:                  amount,
		})
	})
}

func TestRetrievalData(t *testing.T) {
	owner := tutil.NewIDAddr(t, 101)
	provider := tutil.NewIDAddr(t, 102)
	worker := tutil.NewIDAddr(t, 103)
	flowch := tutil.NewIDAddr(t, 104)
	expert := tutil.NewIDAddr(t, 105)
	coinbase := tutil.NewIDAddr(t, 106)
	minerAddrs := &minerAddrs{owner, worker, coinbase, provider, expert, nil}

	actor := actorHarness{retrieval.Actor{}, t}
	receiver := tutil.NewIDAddr(t, 100)
	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("normal retrieval", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		rt.SetAddressActorType(owner, builtin.AccountActorCodeID)
		rt.SetAddressActorType(provider, builtin.StorageMinerActorCodeID)
		rt.SetAddressActorType(flowch, builtin.FlowChannelActorCodeID)

		amount := big.Mul(big.NewInt(2), builtin.TokenPrecision)
		actor.pledge(rt, owner, amount, &retrieval.PledgeParams{
			Address: owner,
		})

		params := retrieval.RetrievalDataParams{
			PayloadId: flowch.String(),
			Size:      uint64(100000),
			Client:    owner,
			Provider:  provider,
		}

		expectReward := abi.NewTokenAmount(0)
		actor.retrievalData(rt, flowch, minerAddrs, expectReward, &params)

		actor.confirmData(rt, flowch, &params)
	})

	t.Run("miner retrieval", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		rt.SetAddressActorType(owner, builtin.AccountActorCodeID)
		rt.SetAddressActorType(provider, builtin.StorageMinerActorCodeID)
		rt.SetAddressActorType(flowch, builtin.FlowChannelActorCodeID)

		amount := big.Mul(big.NewInt(2), builtin.TokenPrecision)
		actor.pledge(rt, owner, amount, &retrieval.PledgeParams{
			Address: owner,
			Miners:  []address.Address{provider},
		})

		params := retrieval.RetrievalDataParams{
			PayloadId: flowch.String(),
			Size:      uint64(100000),
			Client:    owner,
			Provider:  provider,
		}
		actor.minerRetrieval(rt, provider, &params)
	})

	t.Run("miner not bind", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		rt.SetAddressActorType(owner, builtin.AccountActorCodeID)
		rt.SetAddressActorType(provider, builtin.StorageMinerActorCodeID)
		rt.SetAddressActorType(flowch, builtin.FlowChannelActorCodeID)

		amount := big.Mul(big.NewInt(2), builtin.TokenPrecision)
		actor.pledge(rt, owner, amount, &retrieval.PledgeParams{
			Address: owner,
			Miners:  []address.Address{provider},
		})

		params := retrieval.RetrievalDataParams{
			PayloadId: flowch.String(),
			Size:      uint64(100000),
			Client:    owner,
			Provider:  provider,
		}
		actor.minerRetrieval(rt, provider, &params)
	})
}

func TestActorBind(t *testing.T) {
	client := tutil.NewIDAddr(t, 101)
	target := tutil.NewIDAddr(t, 102)
	miner1 := tutil.NewIDAddr(t, 103)
	miner2 := tutil.NewIDAddr(t, 104)

	actor := actorHarness{retrieval.Actor{}, t}
	receiver := tutil.NewIDAddr(t, 100)
	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("bind miner", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		rt.SetAddressActorType(client, builtin.AccountActorCodeID)
		rt.SetAddressActorType(target, builtin.AccountActorCodeID)
		rt.SetAddressActorType(miner1, builtin.StorageMinerActorCodeID)
		rt.SetAddressActorType(miner2, builtin.StorageMinerActorCodeID)

		amount := big.Mul(big.NewInt(2), builtin.TokenPrecision)
		actor.pledge(rt, client, amount, &retrieval.PledgeParams{
			Address: target,
		})

		actor.bindMiners(rt, target, &retrieval.BindMinersParams{
			Pledger: target,
			Miners:  []address.Address{miner1, miner2},
		})

		actor.unbindMiners(rt, target, &retrieval.BindMinersParams{
			Pledger: target,
			Miners:  []address.Address{miner1},
		})
	})
}

type actorHarness struct {
	retrieval.Actor
	t testing.TB
}

func (h *actorHarness) constructAndVerify(rt *mock.Runtime) {
	rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	ret := rt.Call(h.Constructor, nil)
	require.Nil(h.t, ret)
	rt.Verify()
}

func (h *actorHarness) pledge(rt *mock.Runtime, pledger address.Address, amount abi.TokenAmount, params *retrieval.PledgeParams) {
	rt.SetCaller(pledger, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)

	rt.SetReceived(amount)

	if len(params.Miners) > 0 {
		for _, m := range params.Miners {
			rt.ExpectSend(m, builtin.MethodsMiner.BindRetrievalPledger, &miner.RetrievalPledgeParams{Pledger: params.Address}, abi.NewTokenAmount(0), nil, exitcode.Ok)
		}
	}

	rt.Call(h.Pledge, params)
	rt.Verify()
}

func (h *actorHarness) applyForWithdraw(rt *mock.Runtime, pledger address.Address, params *retrieval.WithdrawBalanceParams) {
	rt.SetCaller(pledger, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(pledger)

	rt.Call(h.ApplyForWithdraw, params)
	rt.Verify()
}

func (h *actorHarness) withdrawBalance(rt *mock.Runtime, pledger address.Address, params *retrieval.WithdrawBalanceParams) {
	rt.SetCaller(pledger, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)

	rt.Call(h.WithdrawBalance, params)
	rt.Verify()
}

func (h *actorHarness) retrievalData(rt *mock.Runtime, flowch address.Address, miner *minerAddrs, expectReward abi.TokenAmount, params *retrieval.RetrievalDataParams) {
	rt.SetCaller(flowch, builtin.FlowChannelActorCodeID)
	rt.ExpectValidateCallerType(builtin.FlowChannelActorCodeID)

	expectGetControlAddresses(rt, miner.provider, miner.owner, miner.worker, miner.coinbase)

	if !expectReward.IsZero() {
		rt.ExpectSend(miner.coinbase, builtin.MethodSend, nil, expectReward, nil, exitcode.Ok)
	}
	rt.Call(h.RetrievalData, params)
	rt.Verify()
}

func (h *actorHarness) confirmData(rt *mock.Runtime, flowch address.Address, params *retrieval.RetrievalDataParams) {
	rt.SetCaller(flowch, builtin.FlowChannelActorCodeID)
	rt.ExpectValidateCallerType(builtin.FlowChannelActorCodeID)

	rt.Call(h.ConfirmData, params)
	rt.Verify()
}

func (h *actorHarness) minerRetrieval(rt *mock.Runtime, miner address.Address, params *retrieval.RetrievalDataParams) {
	rt.SetCaller(miner, builtin.StorageMinerActorCodeID)
	rt.ExpectValidateCallerType(builtin.StorageMinerActorCodeID)

	rt.Call(h.MinerRetrieval, params)
	rt.Verify()
}

func (h *actorHarness) applyRewards(rt *mock.Runtime, rewards abi.TokenAmount) {
	rt.SetCaller(builtin.RewardActorAddr, builtin.RewardActorCodeID)
	rt.ExpectValidateCallerType(builtin.RewardActorCodeID)

	require.Equal(h.t, rewards, rt.Balance())

	rt.Call(h.ApplyRewards, nil)
	rt.Verify()
}

func (h *actorHarness) bindMiners(rt *mock.Runtime, target address.Address, params *retrieval.BindMinersParams) {
	rt.SetCaller(target, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(target)

	for _, miner := range params.Miners {
		rt.ExpectSend(miner, builtin.MethodsMiner.BindRetrievalPledger,
			&builtin.RetrievalPledgeParams{
				Pledger: target,
			}, abi.NewTokenAmount(0), &builtin.Discard{}, exitcode.Ok)
	}
	rt.Call(h.BindMiners, params)
	rt.Verify()
}

func (h *actorHarness) unbindMiners(rt *mock.Runtime, target address.Address, params *retrieval.BindMinersParams) {
	rt.SetCaller(target, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerAddr(target)

	rt.Call(h.UnbindMiners, params)
	rt.Verify()
}

type minerAddrs struct {
	owner    address.Address
	worker   address.Address
	coinbase address.Address
	provider address.Address
	expert   address.Address
	control  []address.Address
}

func expectGetControlAddresses(rt *mock.Runtime, provider address.Address, owner, worker, coinbase address.Address, controls ...address.Address) {
	result := &builtin.GetControlAddressesReturn{Owner: owner, Worker: worker, Coinbase: coinbase, ControlAddrs: controls}
	rt.ExpectSend(
		provider,
		builtin.MethodsMiner.ControlAddresses,
		nil,
		big.Zero(),
		result,
		exitcode.Ok,
	)
}
