package expertfund

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type StateSummary struct {
	ExpertsCount      int
	TrackedCount      int
	DatasCount        int
	LastRewardBalance abi.TokenAmount
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

	var pool PoolInfo
	if err := store.Get(context.Background(), st.PoolInfo, &pool); err != nil {
		acc.Addf("failed to load Pool: %v", err)
	} else {
		acc.Require(pool.CurrentTotalDataSize == sumDataSize, "total data size != sum of experts' data size")
		sum.LastRewardBalance = pool.LastRewardBalance
	}

	// DisqualifiedExperts
	if texperts, err := adt.AsSet(store, st.DisqualifiedExperts, builtin.DefaultHamtBitwidth); err != nil {
		acc.Addf("failed to load tracked experts: %v", err)
	} else {
		err = texperts.ForEach(func(k string) error {
			sum.TrackedCount++
			return nil
		})
		acc.RequireNoError(err, "failed to iterate tracked experts")
	}

	// PieceInfos
	if eop, err := adt.AsMap(store, st.PieceInfos, builtin.DefaultHamtBitwidth); err != nil {
		acc.Addf("failed to load datas: %v", err)
	} else {
		err = eop.ForEach(nil, func(k string) error {
			sum.DatasCount++
			return nil
		})
		acc.RequireNoError(err, "failed to iterate datas")
	}

	return sum, acc
}
