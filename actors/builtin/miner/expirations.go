package miner

import (
	"sort"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
)

type sectorEpochSet struct {
	epoch   abi.ChainEpoch
	sectors []uint64 // TODO: consider a bitfield if it will be always used that way
	power   PowerPair
	pledge  abi.TokenAmount
}

// Takes a slice of sector infos and returns sector info sets grouped and
// sorted by expiration epoch.
//
// Note: While each sector set is sorted by epoch, the order of per-epoch sector
// sets is maintained.
func groupSectorsByExpiration(sectorSize abi.SectorSize, sectors []*SectorOnChainInfo) []sectorEpochSet {
	sectorsByExpiration := make(map[abi.ChainEpoch][]*SectorOnChainInfo)

	// XXX quantize expiration groups
	for _, sector := range sectors {
		sectorsByExpiration[sector.Expiration] = append(sectorsByExpiration[sector.Expiration], sector)
	}

	sectorEpochSets := make([]sectorEpochSet, 0, len(sectorsByExpiration))

	// This map iteration is non-deterministic but safe because we sort by epoch below.
	for expiration, sectors := range sectorsByExpiration { //nolint:nomaprange // this is copy and sort
		sectorNumbers := make([]uint64, len(sectors))
		totalPower := NewPowerPairZero()
		totalPledge := big.Zero()
		for _, sector := range sectors {
			totalPower = totalPower.Add(PowerPair{
				Raw: big.NewIntUnsigned(uint64(sectorSize)),
				QA:  QAPowerForSector(sectorSize, sector),
			})
			totalPledge = big.Add(totalPledge, sector.InitialPledge)
		}
		sectorEpochSets = append(sectorEpochSets, sectorEpochSet{
			epoch:   expiration,
			sectors: sectorNumbers,
			power:   totalPower,
			pledge:  totalPledge,
		})
	}

	sort.Slice(sectorEpochSets, func(i, j int) bool {
		return sectorEpochSets[i].epoch < sectorEpochSets[j].epoch
	})
	return sectorEpochSets
}
