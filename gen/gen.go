package main

import (
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/account"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/cron"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expertfund"
	init_ "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/knowledge"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/multisig"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/retrieval"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/reward"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/system"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/verifreg"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/vote"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime/proof"
	"github.com/filecoin-project/specs-actors/v2/actors/states"
	"github.com/filecoin-project/specs-actors/v2/actors/util/smoothing"
	gen "github.com/whyrusleeping/cbor-gen"
)

func main() {
	// Common types
	if err := gen.WriteTupleEncodersToFile("./actors/runtime/proof/cbor_gen.go", "proof",
		proof.SectorInfo{},            // Aliased from v0
		proof.SealVerifyInfo{},        // Aliased from v0
		proof.PoStProof{},             // Aliased from v0
		proof.WindowPoStVerifyInfo{},  // Aliased from v0
		proof.WinningPoStVerifyInfo{}, // Aliased from v0
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/cbor_gen.go", "builtin",
		builtin.MinerAddrs{},
		builtin.ExpertAddr{},
		builtin.ConfirmSectorProofsParams{}, // Aliased from v0
		builtin.ApplyRewardParams{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/states/cbor_gen.go", "states",
		states.Actor{},
	); err != nil {
		panic(err)
	}

	// Actors
	if err := gen.WriteTupleEncodersToFile("./actors/builtin/system/cbor_gen.go", "system",
		// actor state
		system.State{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/account/cbor_gen.go", "account",
		// actor state
		account.State{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/expert/cbor_gen.go", "expert",
		// actor state
		expert.State{},
		expert.ExpertInfo{},
		// method params
		// expert.ConstructorParams{},
		expert.GetControlAddressReturn{},
		expert.ChangePeerIDParams{},
		expert.ChangeMultiaddrsParams{},
		expert.ChangeAddressParams{},
		expert.ExpertDataParams{},
		expert.DataOnChainInfo{},
		// other types
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/expertfund/cbor_gen.go", "expertfund",
		// actor state
		expertfund.State{},
		// method params
		expertfund.ExpertInfo{},
		expertfund.ExpertDepositParams{},
		expertfund.ClaimFundParams{},
		expertfund.ExpertParams{},
		expertfund.NotifyVoteParams{},
		// other types
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/init/cbor_gen.go", "init",
		// actor state
		init_.State{},
		// method params and returns
		init_.ConstructorParams{}, // Aliased from v0
		init_.ExecParams{},        // Aliased from v0
		init_.ExecReturn{},        // Aliased from v0
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/cron/cbor_gen.go", "cron",
		// actor state
		cron.State{},
		cron.Entry{},
		// method params and returns
		cron.ConstructorParams{}, // Aliased from v0
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/reward/cbor_gen.go", "reward",
		// actor state
		reward.State{},
		// method params and returns
		reward.AwardBlockRewardParams{}, // Aliased from v0
		reward.ThisEpochRewardReturn{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/multisig/cbor_gen.go", "multisig",
		// actor state
		multisig.State{},
		multisig.Transaction{},      // Aliased from v0
		multisig.ProposalHashData{}, // Aliased from v0
		// method params and returns
		multisig.ConstructorParams{},
		multisig.ProposeParams{},                     // Aliased from v0
		multisig.ProposeReturn{},                     // Aliased from v0
		multisig.AddSignerParams{},                   // Aliased from v0
		multisig.RemoveSignerParams{},                // Aliased from v0
		multisig.TxnIDParams{},                       // Aliased from v0
		multisig.ApproveReturn{},                     // Aliased from v0
		multisig.ChangeNumApprovalsThresholdParams{}, // Aliased from v0
		multisig.SwapSignerParams{},                  // Aliased from v0
		multisig.LockBalanceParams{},                 // Aliased from v0
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/paych/cbor_gen.go", "paych",
		// actor state
		paych.State{},
		paych.LaneState{},
		// method params and returns
		paych.ConstructorParams{}, // Aliased from v0
		paych.UpdateChannelStateParams{},
		paych.SignedVoucher{},   // Aliased from v0
		paych.ModVerifyParams{}, // Aliased from v0
		// other types
		paych.Merge{}, // Aliased from v0
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/power/cbor_gen.go", "power",
		// actors state
		power.State{},
		power.Claim{},
		power.CronEvent{},
		// method params and returns
		power.CreateMinerParams{},        // Aliased from v0
		power.CreateMinerReturn{},        // Aliased from v0
		power.EnrollCronEventParams{},    // Aliased from v0
		power.UpdateClaimedPowerParams{}, // Aliased from v0
		power.CurrentTotalPowerReturn{},
		// other types
		power.MinerConstructorParams{},
		// expert
		power.Expert{},
		power.CreateExpertParams{},
		power.CreateExpertReturn{},
		power.DeleteExpertParams{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/market/cbor_gen.go", "market",
		// actor state
		market.State{},
		// method params and returns
		market.WithdrawBalanceParams{}, // Aliased from v0
		market.PublishStorageDataRef{},
		market.PublishStorageDealsParams{},      // Aliased from v0
		market.PublishStorageDealsReturn{},      // Aliased from v0
		market.ActivateDealsParams{},            // Aliased from v0
		market.VerifyDealsForActivationParams{}, // Aliased from v0
		market.VerifyDealsForActivationReturn{},
		market.ComputeDataCommitmentParams{},   // Aliased from v0
		market.OnMinerSectorsTerminateParams{}, // Aliased from v0
		// other types
		market.DealProposal{},       // Aliased from v0
		market.ClientDealProposal{}, // Aliased from v0
		market.DealState{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/miner/cbor_gen.go", "miner",
		// actor state
		miner.State{},
		miner.MinerInfo{},
		miner.Deadlines{},
		miner.Deadline{},
		miner.Partition{},
		miner.ExpirationSet{},
		miner.PowerPair{},
		miner.SectorPreCommitOnChainInfo{},
		miner.SectorPreCommitInfo{},
		miner.SectorOnChainInfo{},
		miner.WorkerKeyChange{},
		miner.VestingFunds{},
		miner.VestingFund{},
		// method params and returns
		// miner.ConstructorParams{},        // in power actor
		miner.SubmitWindowedPoStParams{}, // Aliased from v0
		miner.TerminateSectorsParams{},   // Aliased from v0
		miner.TerminateSectorsReturn{},   // Aliased from v0
		miner.ChangePeerIDParams{},       // Aliased from v0
		miner.ChangeMultiaddrsParams{},   // Aliased from v0
		miner.PreCommitSectorParams{},
		miner.ProveCommitSectorParams{},      // Aliased from v0
		miner.ChangeWorkerAddressParams{},    // Aliased from v0
		miner.ExtendSectorExpirationParams{}, // Aliased from v0
		miner.DeclareFaultsParams{},          // Aliased from v0
		miner.DeclareFaultsRecoveredParams{}, // Aliased from v0
		miner.ReportConsensusFaultParams{},   // Aliased from v0
		miner.GetControlAddressesReturn{},
		miner.CheckSectorProvenParams{},    // Aliased from v0
		miner.WithdrawBalanceParams{},      // Aliased from v0
		miner.CompactPartitionsParams{},    // Aliased from v0
		miner.CompactSectorNumbersParams{}, // Aliased from v0
		miner.CronEventPayload{},           // Aliased from v0
		// other types
		miner.FaultDeclaration{},       // Aliased from v0
		miner.RecoveryDeclaration{},    // Aliased from v0
		miner.ExpirationExtension{},    // Aliased from v0
		miner.TerminationDeclaration{}, // Aliased from v0
		miner.PoStPartition{},          // Aliased from v0
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/retrieval/cbor_gen.go", "retrieval",
		// actor state
		retrieval.State{},
		// method params and returns
		retrieval.WithdrawBalanceParams{},
		retrieval.RetrievalDataParams{},
		retrieval.RetrievalState{},
		retrieval.LockedState{},
		retrieval.TotalCollateralReturn{},
		// other types
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/verifreg/cbor_gen.go", "verifreg",
		// actor state
		verifreg.State{},
		// method params and returns
		verifreg.AddVerifierParams{},       // Aliased from v0
		verifreg.AddVerifiedClientParams{}, // Aliased from v0
		verifreg.UseBytesParams{},          // Aliased from v0
		verifreg.RestoreBytesParams{},      // Aliased from v0
		// other types
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/util/smoothing/cbor_gen.go", "smoothing",
		smoothing.FilterEstimate{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/vote/cbor_gen.go", "vote",
		vote.State{},
		vote.Candidate{},
		vote.Voter{},
		vote.VotingRecord{},
		vote.RevokeParams{},
		vote.VoteParams{},
		vote.RegisterCandidatesParams{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/knowledge/cbor_gen.go", "knowledge",
		knowledge.State{},
		knowledge.ChangePayeeParams{},
		knowledge.AssignUndistributedParams{},
		knowledge.WithdrawBalanceParams{},
	); err != nil {
		panic(err)
	}

}
