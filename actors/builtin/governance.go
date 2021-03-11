package builtin

import (
	addr "github.com/filecoin-project/go-address"
	address "github.com/filecoin-project/go-address"
	abi "github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
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

type NotifyExpertImportParams struct {
	Expert  address.Address
	PieceID cid.Cid
}

func NotifyExpertImport(rt runtime.Runtime, expertAddr addr.Address, pieceID cid.Cid) {
	params := &NotifyExpertImportParams{
		Expert:  expertAddr,
		PieceID: pieceID,
	}
	code := rt.Send(ExpertFundActorAddr, MethodsExpertFunds.NotifyImport, params, abi.NewTokenAmount(0), &Discard{})
	RequireSuccess(rt, code, "failed to notify expert import")
}

func NotifyExpertFundReset(rt runtime.Runtime) {
	code := rt.Send(ExpertFundActorAddr, MethodsExpertFunds.ResetExpert, nil, abi.NewTokenAmount(0), &Discard{})
	RequireSuccess(rt, code, "failed to reset expert")
}

// NotifyVoteParams vote params
type NotifyVote struct {
	Expert address.Address
	Amount abi.TokenAmount
}

type BlockCandidatesParams struct {
	Candidates []addr.Address
}

func NotifyExpertsBlocked(rt runtime.Runtime, blockedExperts ...addr.Address) {
	params := &BlockCandidatesParams{
		Candidates: blockedExperts,
	}
	code := rt.Send(VoteFundActorAddr, MethodsVote.BlockCandidates, params, big.Zero(), &Discard{})
	RequireSuccess(rt, code, "failed to notify experts blocked")
}

type ExpertAddr struct {
	Owner addr.Address
}

func RequestExpertControlAddr(rt runtime.Runtime, expertAddr addr.Address) (ownerAddr addr.Address) {
	var addr ExpertAddr
	code := rt.Send(expertAddr, MethodsExpert.ControlAddress, nil, abi.NewTokenAmount(0), &addr)
	RequireSuccess(rt, code, "failed fetching expert control address")

	return addr.Owner
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
