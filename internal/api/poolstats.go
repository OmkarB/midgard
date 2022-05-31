package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"gitlab.com/thorchain/midgard/internal/db"
	"gitlab.com/thorchain/midgard/internal/timeseries"
	"gitlab.com/thorchain/midgard/internal/timeseries/stat"
	"gitlab.com/thorchain/midgard/internal/util"
	"gitlab.com/thorchain/midgard/internal/util/miderr"
	"gitlab.com/thorchain/midgard/openapi/generated/oapigen"
)

func setAggregatesStats(
	ctx context.Context, pool string, buckets db.Buckets,
	ret *oapigen.PoolStatsResponse) (merr miderr.Err) {

	state := timeseries.Latest.GetState()

	poolInfo := state.PoolInfo(pool)
	if poolInfo == nil || !poolInfo.ExistsNow() {
		merr = miderr.BadRequestF("Unknown pool: %s", pool)
		return
	}

	liquidityUnitsMap, err := stat.CurrentPoolsLiquidityUnits(ctx, []string{pool})
	if err != nil {
		return miderr.InternalErrE(err)
	}

	poolAPY, err := timeseries.GetSinglePoolAPY(ctx, poolInfo.RuneDepth, pool, buckets.Window())
	if err != nil {
		return miderr.InternalErrE(err)
	}

	status, err := timeseries.PoolStatus(ctx, pool)
	if err != nil {
		merr = miderr.InternalErrE(err)
		return
	}

	price := poolInfo.AssetPrice()
	priceUSD := price * stat.RunePriceUSD()
	liquidityUnits := liquidityUnitsMap[pool]
	synthUnits := timeseries.GetSinglePoolSynthUnits(ctx, poolInfo.AssetDepth, poolInfo.SynthDepth, liquidityUnits)
	poolUnits := liquidityUnits + synthUnits

	ret.Asset = pool
	ret.AssetDepth = util.IntStr(poolInfo.AssetDepth)
	ret.RuneDepth = util.IntStr(poolInfo.RuneDepth)
	ret.PoolAPY = floatStr(poolAPY)
	ret.AssetPrice = floatStr(price)
	ret.AssetPriceUSD = floatStr(priceUSD)
	ret.Status = status
	ret.Units = util.IntStr(poolUnits)
	ret.LiquidityUnits = util.IntStr(liquidityUnits)
	ret.SynthUnits = util.IntStr(synthUnits)
	ret.SynthSupply = util.IntStr(poolInfo.SynthDepth)

	return
}

func setSwapStats(
	ctx context.Context, pool string, buckets db.Buckets,
	ret *oapigen.PoolStatsResponse) (merr miderr.Err) {

	swapHistory, err := stat.GetOneIntervalSwapsNoUSD(ctx, &pool, buckets)
	if err != nil {
		merr = miderr.InternalErrE(err)
		return
	}

	ret.ToRuneVolume = util.IntStr(swapHistory.AssetToRuneVolume)
	ret.ToAssetVolume = util.IntStr(swapHistory.RuneToAssetVolume)
	ret.SwapVolume = util.IntStr(swapHistory.TotalVolume)

	ret.ToRuneCount = util.IntStr(swapHistory.AssetToRuneCount)
	ret.ToAssetCount = util.IntStr(swapHistory.RuneToAssetCount)
	ret.SwapCount = util.IntStr(swapHistory.TotalCount)

	ret.ToAssetAverageSlip = ratioStr(swapHistory.RuneToAssetSlip, swapHistory.RuneToAssetCount)
	ret.ToRuneAverageSlip = ratioStr(swapHistory.AssetToRuneSlip, swapHistory.AssetToRuneCount)
	ret.AverageSlip = ratioStr(swapHistory.TotalSlip, swapHistory.TotalCount)

	ret.ToAssetFees = util.IntStr(swapHistory.RuneToAssetFees)
	ret.ToRuneFees = util.IntStr(swapHistory.AssetToRuneFees)
	ret.TotalFees = util.IntStr(swapHistory.TotalFees)

	return
}

func setLiquidityStats(
	ctx context.Context, pool string, buckets db.Buckets,
	ret *oapigen.PoolStatsResponse) (merr miderr.Err) {

	var allLiquidity oapigen.LiquidityHistoryResponse
	// TODO(muninn): replace with GetLiquidityHistoryNOUSD to speed up
	allLiquidity, err := stat.GetLiquidityHistory(ctx, buckets, pool)
	if err != nil {
		merr = miderr.InternalErrE(err)
		return
	}
	ret.AddAssetLiquidityVolume = allLiquidity.Meta.AddAssetLiquidityVolume
	ret.AddRuneLiquidityVolume = allLiquidity.Meta.AddRuneLiquidityVolume
	ret.AddLiquidityVolume = allLiquidity.Meta.AddLiquidityVolume
	ret.AddLiquidityCount = allLiquidity.Meta.AddLiquidityCount
	ret.WithdrawAssetVolume = allLiquidity.Meta.WithdrawAssetVolume
	ret.WithdrawRuneVolume = allLiquidity.Meta.WithdrawRuneVolume
	ret.ImpermanentLossProtectionPaid = allLiquidity.Meta.ImpermanentLossProtectionPaid
	ret.WithdrawVolume = allLiquidity.Meta.WithdrawVolume
	ret.WithdrawCount = allLiquidity.Meta.WithdrawCount
	return
}

func statsForPool(ctx context.Context, pool string, buckets db.Buckets) (
	ret oapigen.PoolStatsResponse, merr miderr.Err) {

	merr = setAggregatesStats(ctx, pool, buckets, &ret)
	if merr != nil {
		return
	}

	merr = setSwapStats(ctx, pool, buckets, &ret)
	if merr != nil {
		return
	}

	// TODO(huginn): optimize deposit/withdraw total volme and count
	merr = setLiquidityStats(ctx, pool, buckets, &ret)
	if merr != nil {
		return
	}

	// TODO(huginn): optimize unique member adresses to use latest
	members, err := timeseries.GetMemberIds(ctx, &pool)
	if err != nil {
		merr = miderr.InternalErrE(err)
		return
	}
	ret.UniqueMemberCount = strconv.Itoa(len(members))

	ret.UniqueSwapperCount = "0" // deprecated

	return
}

func jsonPoolStats(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pool := ps[0].Value

	urlParams := r.URL.Query()
	period := util.ConsumeUrlParam(&urlParams, "period")
	if period == "" {
		period = "30d"
	}
	var buckets db.Buckets
	now := db.NowSecond()
	switch period {
	case "1h":
		buckets = db.Buckets{Timestamps: db.Seconds{now - 60*60, now}}
	case "24h":
		buckets = db.Buckets{Timestamps: db.Seconds{now - 24*60*60, now}}
	case "7d":
		buckets = db.Buckets{Timestamps: db.Seconds{now - 7*24*60*60, now}}
	case "30d":
		buckets = db.Buckets{Timestamps: db.Seconds{now - 30*24*60*60, now}}
	case "90d":
		buckets = db.Buckets{Timestamps: db.Seconds{now - 90*24*60*60, now}}
	case "180d":
		buckets = db.Buckets{Timestamps: db.Seconds{now - 180*24*60*60, now}}
	case "365d":
		buckets = db.Buckets{Timestamps: db.Seconds{now - 365*24*60*60, now}}
	case "all":
		buckets = db.AllHistoryBuckets()
	default:
		miderr.BadRequestF(
			"Parameter period parameter(%s). Accepted values:  1h, 24h, 7d, 30d, 90d, 365d, all",
			period).ReportHTTP(w)
		return
	}

	merr := util.CheckUrlEmpty(urlParams)
	if merr != nil {
		merr.ReportHTTP(w)
		return
	}

	result, merr := statsForPool(r.Context(), pool, buckets)
	if merr != nil {
		merr.ReportHTTP(w)
		return
	}
	respJSON(w, result)
}
