package expertfund

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type StateSummary struct {
	ExpertsCount int
	TrackedCount int
	DatasCount   int
}

// Checks internal invariants of expertfund state.
func CheckStateInvariants(st *State, store adt.Store) (*StateSummary, *builtin.MessageAccumulator) {
	acc := &builtin.MessageAccumulator{}
	sum := &StateSummary{}

	// Experts
	sumDataSize := abi.PaddedPieceSize(0)
	if experts, err := adt.AsMap(store, st.Experts, builtin.DefaultHamtBitwidth); err != nil {
		acc.Addf("failed to load experts: %v", err)
	} else {
		var ei ExpertInfo
		err = experts.ForEach(&ei, func(k string) error {
			sum.ExpertsCount++
			sumDataSize += ei.DataSize
			return nil
		})
		acc.RequireNoError(err, "failed to iterate experts")
	}
	acc.Require(st.ExpertsCount == uint64(sum.ExpertsCount), "experts count mismatch: %d, %d", st.ExpertsCount, sum.ExpertsCount)
	acc.Require(st.TotalExpertDataSize == sumDataSize, "total data size != sum of experts' data size")

	// TrackedExperts
	if texperts, err := adt.AsSet(store, st.TrackedExperts, builtin.DefaultHamtBitwidth); err != nil {
		acc.Addf("failed to load tracked experts: %v", err)
	} else {
		err = texperts.ForEach(func(k string) error {
			sum.TrackedCount++
			return nil
		})
		acc.RequireNoError(err, "failed to iterate tracked experts")
	}

	// Datas
	if datas, err := adt.AsMap(store, st.Datas, builtin.DefaultHamtBitwidth); err != nil {
		acc.Addf("failed to load datas: %v", err)
	} else {
		var adr address.Address
		err = datas.ForEach(&adr, func(k string) error {
			sum.DatasCount++
			return nil
		})
		acc.RequireNoError(err, "failed to iterate datas")
	}

	return sum, acc
}
