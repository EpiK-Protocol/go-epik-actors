package builtin

import (
	addr "github.com/filecoin-project/go-address"
	address "github.com/filecoin-project/go-address"
	abi "github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
	"github.com/ipfs/go-cid"
)

// ExpertType is the expert type.
type ExpertType uint64

const (
	// ExpertFoundation foundation expert
	ExpertFoundation ExpertType = iota

	// ExpertNormal normal expert
	ExpertNormal
)

func NotifyExpertImport(rt runtime.Runtime, pieceID cid.Cid) {
	params := &CheckedCID{
		CID: pieceID,
	}
	code := rt.Send(ExpertFundActorAddr, MethodsExpertFunds.OnExpertImport, params, abi.NewTokenAmount(0), &Discard{})
	RequireSuccess(rt, code, "failed to notify expert import")
}

func RequestExpertControlAddr(rt runtime.Runtime, expertAddr addr.Address) (ownerAddr addr.Address) {
	code := rt.Send(expertAddr, MethodsExpert.ControlAddress, nil, abi.NewTokenAmount(0), &ownerAddr)
	RequireSuccess(rt, code, "failed fetching expert control address")
	return
}

// ============== govern ============

type ValidateGrantedParams struct {
	Caller address.Address
	Method abi.MethodNum
}

// Validates that if caller is granted on the method
func ValidateCallerGranted(rt runtime.Runtime, caller addr.Address, method abi.MethodNum) {
	params := &ValidateGrantedParams{
		Caller: caller,
		Method: method,
	}
	code := rt.Send(GovernActorAddr, MethodsGovern.ValidateGranted, params, abi.NewTokenAmount(0), &Discard{})
	errMsg := "failed to validate caller granted"
	if code == exitcode.ErrForbidden {
		errMsg = "method not granted"
	}
	RequireSuccess(rt, code, errMsg)
}

type CheckExpertStateReturn struct {
	AllowVote bool
	Qualified bool
}

func CheckVoteAllowed(rt runtime.Runtime, expertAddr addr.Address) bool {
	var out CheckExpertStateReturn
	code := rt.Send(expertAddr, MethodsExpert.CheckState, nil, abi.NewTokenAmount(0), &out)
	RequireSuccess(rt, code, "failed to get expert state %s", expertAddr)
	return out.AllowVote
}

type OnExpertVotesUpdatedParams struct {
	Expert address.Address
	Votes  abi.TokenAmount
}
