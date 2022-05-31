// Depth recorder fills keeps track of historical depth values and inserts changes
// in the block_pool_depths table.
package timeseries

import (
	"fmt"
	"time"

	"gitlab.com/thorchain/midgard/internal/db"
)

// MapDiff helps to get differences between snapshots of a map.
type mapDiff struct {
	snapshot map[string]int64
}

// Save the map as the new snapshot.
func (md *mapDiff) save(newMap map[string]int64) {
	md.snapshot = map[string]int64{}
	for k, v := range newMap {
		md.snapshot[k] = v
	}
}

// Check if there is a chage for this pool.
func (md *mapDiff) diffAtKey(pool string, newMap map[string]int64) (hasDiff bool, newValue int64) {
	oldV, hasOld := md.snapshot[pool]
	newV, hasNew := newMap[pool]
	if hasNew {
		return !hasOld || oldV != newV, newV
	} else {
		return hasOld, 0
	}
}

type depthManager struct {
	assetE8DepthSnapshot mapDiff
	runeE8DepthSnapshot  mapDiff
	synthE8DepthSnapshot mapDiff
}

var depthRecorder depthManager

// Insert rows in the block_pool_depths for every changed value in the depth maps.
// If there is no change it doesn't write out anything.
// All values will be writen out together (assetDepth, runeDepth, synthDepth), even if only one of the values
// changed in the pool.
func (sm *depthManager) update(
	timestamp time.Time, assetE8DepthPerPool, runeE8DepthPerPool, synthE8DepthPerPool map[string]int64) error {
	blockTimestamp := timestamp.UnixNano()
	// We need to iterate over all 2*n maps: {old,new}{Asset,Rune,Synth}.
	// First put all pool names into a set.
	poolNames := map[string]bool{}
	accumulatePoolNames := func(m map[string]int64) {
		for pool := range m {
			poolNames[pool] = true
		}
	}
	accumulatePoolNames(assetE8DepthPerPool)
	accumulatePoolNames(runeE8DepthPerPool)
	accumulatePoolNames(synthE8DepthPerPool)
	accumulatePoolNames(sm.assetE8DepthSnapshot.snapshot)
	accumulatePoolNames(sm.runeE8DepthSnapshot.snapshot)
	accumulatePoolNames(sm.synthE8DepthSnapshot.snapshot)

	cols := []string{"pool", "asset_e8", "rune_e8", "synth_e8", "block_timestamp"}

	var err error
	for pool := range poolNames {
		assetDiff, assetValue := sm.assetE8DepthSnapshot.diffAtKey(pool, assetE8DepthPerPool)
		runeDiff, runeValue := sm.runeE8DepthSnapshot.diffAtKey(pool, runeE8DepthPerPool)
		synthDiff, synthValue := sm.synthE8DepthSnapshot.diffAtKey(pool, synthE8DepthPerPool)
		if assetDiff || runeDiff || synthDiff {
			err = db.Inserter.Insert("block_pool_depths", cols, pool, assetValue, runeValue, synthValue, blockTimestamp)
			if err != nil {
				break
			}
		}
	}
	sm.assetE8DepthSnapshot.save(assetE8DepthPerPool)
	sm.runeE8DepthSnapshot.save(runeE8DepthPerPool)
	sm.synthE8DepthSnapshot.save(synthE8DepthPerPool)

	if err != nil {
		return fmt.Errorf("error saving depths (timestamp: %d): %w", blockTimestamp, err)
	}

	return nil
}
