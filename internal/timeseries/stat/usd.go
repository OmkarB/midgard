package stat

import (
	"fmt"
	"math"
	"net/http"

	"gitlab.com/thorchain/midgard/config"
	"gitlab.com/thorchain/midgard/internal/timeseries"
)

func runePriceUSDForDepths(depths timeseries.DepthMap) float64 {
	ret := math.NaN()
	var maxdepth int64 = -1

	for _, pool := range config.Global.UsdPools {
		poolInfo, ok := depths[pool]
		if ok && maxdepth < poolInfo.RuneDepth {
			maxdepth = poolInfo.RuneDepth
			ret = 1 / poolInfo.AssetPrice()
		}
	}
	return ret
}

// Returns the 1/price from the depest whitelisted pool.
func RunePriceUSD() float64 {
	return runePriceUSDForDepths(timeseries.Latest.GetState().Pools)
}

func ServeUSDDebug(resp http.ResponseWriter, req *http.Request) {
	state := timeseries.Latest.GetState()
	for _, pool := range config.Global.UsdPools {
		poolInfo := state.PoolInfo(pool)
		if poolInfo == nil {
			fmt.Fprintf(resp, "%s - pool not found\n", pool)
		} else {
			depth := float64(poolInfo.RuneDepth) / 1e8
			runePrice := 1 / poolInfo.AssetPrice()
			fmt.Fprintf(resp, "%s - runeDepth: %.0f runePriceUsd: %.2f\n", pool, depth, runePrice)
		}
	}

	fmt.Fprintf(resp, "\n\nrunePriceUSD: %v", RunePriceUSD())
}
