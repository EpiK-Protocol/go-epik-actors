package main

import (
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/account"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/cron"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expert"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/expertfund"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/govern"
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
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/vote"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime/proof"
	"github.com/filecoin-project/specs-actors/v2/actors/states"
	"github.com/filecoin-project/specs-actors/v2/actors/util/smoothing"
	gen "github.com/whyrusleeping/cbor-gen"
)

func main() {
	// Common types
	if err := gen.WriteTupleEncodersToFile("./actors/runtime/proof/cbor_gen.go", "proof",
		proof.SectorInfo{},
		proof.SealVerifyInfo{},
		proof.PoStProof{},
		proof.WindowPoStVerifyInfo{},
		proof.WinningPoStVerifyInfo{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/cbor_gen.go", "builtin",
		builtin.GetControlAddressesReturn{},
		builtin.ExpertControlAddressReturn{},
		builtin.ConfirmSectorProofsParams{},
		builtin.ApplyRewardParams{},
		builtin.BoolValue{},
		builtin.ValidateGrantedParams{},
		builtin.BatchPieceCIDParams{},
		builtin.CheckedCID{},
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
		expert.ConstructorParams{},
		expert.ExpertDataParams{},
		expert.DataOnChainInfo{},
		expert.NominateExpertParams{},
		expert.CheckStateReturn{},
		expert.OnTrackUpdateParams{},
		expert.OnTrackUpdateReturn{},
		expert.StoreDataReturn{},
		expert.GetDatasReturn{},
		// other types
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/expertfund/cbor_gen.go", "expertfund",
		// actor state
		expertfund.State{},
		expertfund.DataInfo{},
		expertfund.PoolInfo{},
		expertfund.ExpertInfo{},
		// method params and returns
		expertfund.ClaimFundParams{},
		expertfund.GetDataReturn{},
		expertfund.ChangeThresholdParams{},
		expertfund.ApplyForExpertParams{},
		expertfund.ApplyForExpertReturn{},
		expertfund.CheckedPiece{},
		expertfund.BatchCheckDataParams{},
		// other types
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/init/cbor_gen.go", "init",
		// actor state
		init_.State{},
		// method params and returns
		init_.ConstructorParams{},
		init_.ExecParams{},
		init_.ExecReturn{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/cron/cbor_gen.go", "cron",
		// actor state
		cron.State{},
		cron.Entry{},
		// method params and returns
		cron.ConstructorParams{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/reward/cbor_gen.go", "reward",
		// actor state
		reward.State{},
		// method params and returns
		reward.AwardBlockRewardParams{},
		reward.AwardBlockRewardReturn{},
		reward.ThisEpochRewardReturn{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/multisig/cbor_gen.go", "multisig",
		// actor state
		multisig.State{},
		multisig.Transaction{},
		multisig.ProposalHashData{},
		// method params and returns
		multisig.ConstructorParams{},
		multisig.ProposeParams{},
		multisig.ProposeReturn{},
		multisig.AddSignerParams{},
		multisig.RemoveSignerParams{},
		multisig.TxnIDParams{},
		multisig.ApproveReturn{},
		multisig.ChangeNumApprovalsThresholdParams{},
		multisig.SwapSignerParams{},
		multisig.LockBalanceParams{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/paych/cbor_gen.go", "paych",
		// actor state
		paych.State{},
		paych.LaneState{},
		// method params and returns
		paych.ConstructorParams{},
		paych.UpdateChannelStateParams{},
		paych.SignedVoucher{},
		paych.ModVerifyParams{},
		// other types
		paych.Merge{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/power/cbor_gen.go", "power",
		// actors state
		power.State{},
		power.Claim{},
		power.CronEvent{},
		power.WdPoStRatio{},
		// method params and returns
		power.CreateMinerParams{},
		power.CreateMinerReturn{},
		power.EnrollCronEventParams{},
		power.UpdateClaimedPowerParams{},
		power.CurrentTotalPowerReturn{},
		power.AllowNoWindowPoStParams{},
		power.AllowNoWindowPoStReturn{},
		power.ChangeWdPoStRatioParams{},
		// other types
		power.MinerConstructorParams{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/market/cbor_gen.go", "market",
		// actor state
		market.State{},
		// method params and returns
		market.WithdrawBalanceParams{},
		market.StorageDataRef{},
		market.PublishStorageDealsParams{},
		market.PublishStorageDealsReturn{},
		market.ActivateDealsParams{},
		market.ActivateDealsReturn{},
		market.VerifyDealsForActivationParams{},
		market.VerifyDealsForActivationReturn{},
		market.SectorDeals{},
		market.SectorDealInfos{},
		market.ComputeDataCommitmentParams{},
		market.OnMinerSectorsTerminateParams{},
		market.NewQuota{},
		market.ResetQuotasParams{},
		// other types
		market.DealProposal{},
		market.ClientDealProposal{},
		market.DealState{},
		market.DataIndex{},
		market.PendingProposal{},
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
		miner.WindowedPoSt{},
		// method params and returns
		// miner.ConstructorParams{},        // in power actor
		miner.SubmitWindowedPoStParams{},
		miner.TerminateSectorsParams{},
		miner.TerminateSectorsReturn{},
		miner.ChangePeerIDParams{},
		miner.ChangeMultiaddrsParams{},
		miner.PreCommitSectorParams{},
		miner.ProveCommitSectorParams{},
		miner.ChangeWorkerAddressParams{},
		/* miner.ExtendSectorExpirationParams{},  */
		miner.DeclareFaultsParams{},
		miner.DeclareFaultsRecoveredParams{},
		miner.ReportConsensusFaultParams{},
		miner.CheckSectorProvenParams{},
		miner.WithdrawBalanceParams{},
		miner.CompactPartitionsParams{},
		miner.CompactSectorNumbersParams{},
		miner.CronEventPayload{},
		miner.WithdrawPledgeParams{},
		miner.DisputeWindowedPoStParams{},
		// other types
		miner.FaultDeclaration{},
		miner.RecoveryDeclaration{},
		/* miner.ExpirationExtension{},     */
		miner.TerminationDeclaration{},
		miner.PoStPartition{},
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

	// if err := gen.WriteTupleEncodersToFile("./actors/builtin/verifreg/cbor_gen.go", "verifreg",
	// 	// actor state
	// 	verifreg.State{},
	// 	// method params and returns
	// 	verifreg.AddVerifierParams{},
	// 	verifreg.AddVerifiedClientParams{},
	// 	verifreg.UseBytesParams{},
	// 	verifreg.RestoreBytesParams{},
	// 	// other types
	// ); err != nil {
	// 	panic(err)
	// }

	if err := gen.WriteTupleEncodersToFile("./actors/util/smoothing/cbor_gen.go", "smoothing",
		smoothing.FilterEstimate{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/vote/cbor_gen.go", "vote",
		vote.State{},
		vote.Candidate{},
		vote.Voter{},
		vote.VotesInfo{},
		vote.RescindParams{},
		vote.GetCandidatesParams{},
		vote.GetCandidatesReturn{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/knowledge/cbor_gen.go", "knowledge",
		knowledge.State{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/govern/cbor_gen.go", "govern",
		// actor state
		govern.State{},
		govern.GrantedAuthorities{},
		govern.GrantOrRevokeParams{},
		govern.Authority{},
	); err != nil {
		panic(err)
	}

}
