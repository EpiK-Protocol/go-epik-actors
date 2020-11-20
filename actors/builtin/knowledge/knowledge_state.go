package knowledge

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	. "github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"
)

type State struct {
	// TODO: init address?
	// Current payee of knowledge fund rewards.
	Payee addr.Address // ID-Address

	// Total distributed rewards.
	TotalDistributed abi.TokenAmount

	// Total undistributed rewards.
	// Rewards will be added to this when Payee is undef
	TotalUndistributed abi.TokenAmount

	Distributions cid.Cid // Map, HAMT [ID-Address]TokenAmount
}

func ConstructState(emptyMapCid cid.Cid, initPayee addr.Address) *State {
	return &State{
		Distributions: emptyMapCid,
		Payee:         initPayee,
	}
}

func (st *State) addDistribution(distributions *adt.Map, payee addr.Address, amount abi.TokenAmount) error {
	AssertMsg(amount.GreaterThanEqual(big.Zero()), "non positive distribution to add")

	old, err := getDistribution(distributions, payee)
	if err != nil {
		return err
	}
	return putDistribution(distributions, payee, big.Add(old, amount))
}

func (st *State) withdrawDistribution(distributions *adt.Map, payee addr.Address, amount abi.TokenAmount) error {
	AssertMsg(amount.GreaterThanEqual(big.Zero()), "non positive distribution to withdraw")

	old, err := getDistribution(distributions, payee)
	if err != nil {
		return err
	}
	if old.LessThan(amount) {
		return xerrors.Errorf("insufficent balance %v of %s, requested %v", old, payee, amount)
	}

	return putDistribution(distributions, payee, big.Sub(old, amount))
}

func getDistribution(distributions *adt.Map, payee addr.Address) (abi.TokenAmount, error) {
	var out abi.TokenAmount
	found, err := distributions.Get(abi.AddrKey(payee), &out)
	if err != nil {
		return abi.NewTokenAmount(0), errors.Wrapf(err, "failed to get distribution of %s", payee)
	}
	if !found {
		return abi.NewTokenAmount(0), nil
	}
	return out, nil
}

func putDistribution(distributions *adt.Map, payee addr.Address, amount abi.TokenAmount) error {
	err := distributions.Put(abi.AddrKey(payee), &amount)
	if err != nil {
		return errors.Wrapf(err, "failed to put new distribution of %s", payee)
	}
	return nil
}
