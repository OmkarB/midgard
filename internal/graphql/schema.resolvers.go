package graphql

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"gitlab.com/thorchain/midgard/internal/db"
	"gitlab.com/thorchain/midgard/internal/timeseries"
	"gitlab.com/thorchain/midgard/internal/util/miderr"

	"gitlab.com/thorchain/midgard/internal/fetch/notinchain"
	"gitlab.com/thorchain/midgard/internal/graphql/generated"
	"gitlab.com/thorchain/midgard/internal/graphql/model"
	"gitlab.com/thorchain/midgard/internal/timeseries/stat"
)

func (r *poolResolver) Status(ctx context.Context, obj *model.Pool) (string, error) {
	return timeseries.PoolStatus(ctx, obj.Asset)
}

func (r *poolResolver) Price(ctx context.Context, obj *model.Pool) (float64, error) {
	assetE8DepthPerPool, runeE8DepthPerPool, _ := timeseries.AssetAndRuneDepths()
	assetDepth := assetE8DepthPerPool[obj.Asset]
	runeDepth := runeE8DepthPerPool[obj.Asset]

	if assetDepth == 0 {
		return 0, nil
	}

	priceInRune := big.NewRat(runeDepth, assetDepth)
	f, _ := priceInRune.Float64()
	return f, nil
}

func (r *poolResolver) Units(ctx context.Context, obj *model.Pool) (int64, error) {
	poolUnits, err := stat.CurrentPoolsLiquidityUnits(ctx, []string{obj.Asset})
	if err != nil {
		return 0, err
	}

	return poolUnits[obj.Asset], nil
}

// TODO(donfrigo) add memoization layer to cache requests
// or find a way to only make the same query once every request
func (r *poolResolver) Volume24h(ctx context.Context, obj *model.Pool) (int64, error) {
	// TODO(acsaba): don't check pool existence at each graphql field.
	if !timeseries.PoolExists(obj.Asset) {
		return 0, errors.New("pool not found")
	}
	now := db.NowSecond()
	window24h := db.Window{From: now - 24*60*60, Until: now}
	dailyVolume, err := stat.PoolsTotalVolume(ctx, []string{obj.Asset}, window24h)
	if err != nil {
		return 0, err
	}
	return dailyVolume[obj.Asset], err
}

func (r *poolResolver) PoolApy(ctx context.Context, obj *model.Pool) (float64, error) {
	_, runeE8DepthPerPool, timestamp := timeseries.AssetAndRuneDepths()

	runeDepth, ok := runeE8DepthPerPool[obj.Asset]
	if !ok {
		return 0, errors.New("pool not found")
	}

	now := db.TimeToSecond(timestamp)
	week := db.Window{From: now - 7*24*60*60, Until: now}
	poolAPY, err := timeseries.GetSinglePoolAPY(
		ctx, runeDepth, obj.Asset, week)
	if err != nil {
		return 0, miderr.InternalErrE(err)
	}

	return poolAPY, nil
}

// TODO(acsaba): make inner libraries return ints, drop this function
func strToInt(s string) int64 {
	// discard err
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

func (r *poolResolver) Stakes(ctx context.Context, obj *model.Pool) (*model.PoolStakes, error) {
	assetE8DepthPerPool, runeE8DepthPerPool, _ := timeseries.AssetAndRuneDepths()
	allLiquidity, err := stat.GetLiquidityHistory(ctx, db.AllHistoryBuckets(), obj.Asset)
	if err != nil {
		return nil, err
	}

	meta := allLiquidity.Meta
	assetNetStaked := strToInt(meta.AddRuneLiquidityVolume) - strToInt(meta.WithdrawRuneVolume)
	runeNetStaked := strToInt(meta.AddAssetLiquidityVolume) - strToInt(meta.WithdrawAssetVolume)

	ps := &model.PoolStakes{
		AssetStaked: assetNetStaked,
		RuneStaked:  runeNetStaked,
	}
	assetDepth := assetE8DepthPerPool[obj.Asset]
	runeDepth := runeE8DepthPerPool[obj.Asset]

	if assetDepth != 0 {
		priceInRune := big.NewRat(runeDepth, assetDepth)
		poolStakedTotal := big.NewRat(assetNetStaked, 1)
		poolStakedTotal.Mul(poolStakedTotal, priceInRune)
		poolStakedTotal.Add(poolStakedTotal, big.NewRat(runeNetStaked, 1))
		ps.PoolStaked = new(big.Int).Div(poolStakedTotal.Num(), poolStakedTotal.Denom()).Int64()
	}

	return ps, nil
}

func (r *poolResolver) Depth(ctx context.Context, obj *model.Pool) (*model.PoolDepth, error) {
	assetE8DepthPerPool, runeE8DepthPerPool, _ := timeseries.AssetAndRuneDepths()
	assetDepth := assetE8DepthPerPool[obj.Asset]
	runeDepth := runeE8DepthPerPool[obj.Asset]
	return &model.PoolDepth{
		AssetDepth: assetDepth,
		RuneDepth:  runeDepth,
		PoolDepth:  2 * runeDepth,
	}, nil
}

func (r *queryResolver) Pool(ctx context.Context, asset string) (*model.Pool, error) {
	result := &model.Pool{
		Asset: asset,
	}
	return result, nil
}

func (r *queryResolver) Pools(ctx context.Context, limit *int) ([]*model.Pool, error) {
	pools, err := timeseries.PoolsWithDeposit(ctx)
	if err != nil {
		return nil, err
	}

	var result []*model.Pool

	for _, p := range pools {
		result = append(result, &model.Pool{
			Asset: p,
		})
	}

	return result, nil
}

func (r *queryResolver) Stakers(ctx context.Context) ([]*model.Staker, error) {
	addrs, err := timeseries.GetMemberIds(ctx, nil)
	if err != nil {
		return nil, err
	}

	result := make([]*model.Staker, len(addrs))
	for i, a := range addrs {
		result[i] = &model.Staker{
			Address: a,
			// TODO(kashif) other fields require subquery.
			// Not implemented yet.
		}
	}

	return result, nil
}

func (r *queryResolver) Staker(ctx context.Context, address string) (*model.Staker, error) {
	pools, err := timeseries.GetMemberPools(ctx, address)
	if err != nil {
		return nil, err
	}
	if len(pools) == 0 {
		return nil, miderr.BadRequestF("Not found address: %s", address)
	}

	var runeE8Total int64
	assets := make([]*string, len(pools))
	for i := range pools {
		assets[i] = &pools[i].Pool
		runeE8Total += pools[i].RuneAdded
	}

	// TODO(kashif) extra fields aren't supported yet as
	// it is still not available for v1.
	result := &model.Staker{
		PoolsArray:  assets,
		TotalStaked: runeE8Total,
		Address:     address,
	}

	return result, nil
}

func (r *queryResolver) Node(ctx context.Context, address string) (*model.Node, error) {
	node, err := notinchain.CachedNodeAccountLookup(address)
	if err != nil {
		return nil, err
	}

	result := &model.Node{
		PublicKeys: &model.PublicKeys{
			Secp256k1: node.PublicKeys.Secp256k1,
			Ed25519:   node.PublicKeys.Ed25519,
		},
		Address:          node.NodeAddr,
		Status:           node.Status,
		Bond:             node.Bond,
		RequestedToLeave: node.RequestedToLeave,
		ForcedToLeave:    node.ForcedToLeave,
		LeaveHeight:      node.LeaveHeight,
		IPAddress:        node.IpAddress,
		Version:          node.Version,
		SlashPoints:      node.SlashPoints,
		Jail: &model.JailInfo{
			NodeAddr:      node.Jail.NodeAddr,
			ReleaseHeight: node.Jail.ReleaseHeight,
			Reason:        node.Jail.Reason,
		},
		CurrentAward: node.CurrentAward,
	}

	return result, nil
}

func (r *queryResolver) Nodes(ctx context.Context, status *model.NodeStatus) ([]*model.Node, error) {
	nodes, err := notinchain.CachedNodeAccountsLookup()
	if err != nil {
		return nil, err
	}

	// Filter by status
	filteredNodes := []*notinchain.NodeAccount{}

	if status != nil {
		for _, n := range nodes {
			if n.Status == strings.ToLower(status.String()) {
				filteredNodes = append(filteredNodes, n)
			}
		}
		nodes = filteredNodes
	}

	result := make([]*model.Node, 0, len(nodes))
	for _, e := range nodes {
		result = append(result, &model.Node{
			PublicKeys: &model.PublicKeys{
				Secp256k1: e.PublicKeys.Secp256k1,
				Ed25519:   e.PublicKeys.Ed25519,
			},
			Address:          e.NodeAddr,
			Status:           e.Status,
			Bond:             e.Bond,
			RequestedToLeave: e.RequestedToLeave,
			ForcedToLeave:    e.ForcedToLeave,
			LeaveHeight:      e.LeaveHeight,
			IPAddress:        e.IpAddress,
			Version:          e.Version,
			SlashPoints:      e.SlashPoints,
			Jail: &model.JailInfo{
				NodeAddr:      e.Jail.NodeAddr,
				ReleaseHeight: e.Jail.ReleaseHeight,
				Reason:        e.Jail.Reason,
			},
			CurrentAward: e.CurrentAward,
		})
	}

	return result, nil
}

// TODO(kashif) This applies to ALL the stuff here. Ideally we
// should have a common service layer to handle all the business logic.
// So v1 and v2 can both call into the same common one.
func (r *queryResolver) Stats(ctx context.Context) (*model.Stats, error) {
	_, runeE8DepthPerPool, timestamp := timeseries.AssetAndRuneDepths()
	window := db.Window{From: 0, Until: db.TimeToSecond(timestamp)}

	stakes, err := stat.StakesLookup(ctx, window)
	if err != nil {
		return nil, err
	}
	unstakes, err := stat.UnstakesLookup(ctx, window)
	if err != nil {
		return nil, err
	}

	var runeDepth int64
	for _, depth := range runeE8DepthPerPool {
		runeDepth += depth
	}

	result := &model.Stats{
		DailyActiveUsers:   0,
		DailyTx:            0,
		MonthlyActiveUsers: 0,
		MonthlyTx:          0,
		TotalAssetBuys:     0,
		TotalAssetSells:    0,
		TotalDepth:         runeDepth,
		TotalStakeTx:       stakes.Count + unstakes.Count,
		TotalStaked:        stakes.TotalVolume - unstakes.TotalVolume,
		TotalTx:            0 + stakes.Count + unstakes.Count,
		TotalUsers:         0,
		TotalVolume:        0,
		TotalWithdrawTx:    unstakes.TotalVolume,
	}

	return result, nil
}

// TODO(donfrigo) investigate if caching is possible for this endpoint as well
func (r *queryResolver) Network(ctx context.Context) (*model.Network, error) {
	networkData, err := timeseries.GetNetworkData(ctx)
	if err != nil {
		return nil, err
	}

	return &networkData, nil
}

// Modifies incoming parameters.
func setupDefaultParameters(from *int64, until *int64, interval *model.Interval) db.Window {
	// If from is not provided, we go back one week
	if from == nil {
		fromPointer := int64(0)
		from = &fromPointer
	}
	if until == nil {
		untilPointer := int64(0)
		until = &untilPointer
	}
	if interval == nil {
		intervalPointer := model.IntervalDay
		interval = &intervalPointer
	}

	return db.Window{
		// TODO(acsaba): check if timezones matter.
		From:  db.Second(*from),
		Until: db.Second(*until),
	}
}

// Bucketing logic under timeseries uses another enum than the public facing one.
var toStatInterval = map[model.Interval]db.Interval{
	model.IntervalMinute5: db.Min5,
	model.IntervalHour:    db.Hour,
	model.IntervalDay:     db.Day,
	model.IntervalMonth:   db.Month,
	model.IntervalQuarter: db.Quarter,
	model.IntervalYear:    db.Year,
}

func (r *queryResolver) VolumeHistory(ctx context.Context, pool *string, from int64, until int64, interval model.Interval) (*model.PoolVolumeHistory, error) {
	window := setupDefaultParameters(&from, &until, &interval)

	var err error
	buckets, err := db.BucketsFromWindow(ctx, window, toStatInterval[interval])
	if err != nil {
		return nil, err
	}

	poolSwaps, err := stat.GetPoolSwaps(ctx, pool, buckets)
	if err != nil {
		return nil, err
	}

	result, err := createPoolVolumeHistory(poolSwaps)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type volumeMetaData struct {
	time time.Time

	ToRuneTxCount       int64
	ToRuneFeesInRune    int64
	ToRuneVolumesInRune int64

	ToAssetTxCount       int64
	ToAssetFeesInRune    int64
	ToAssetVolumesInRune int64

	CombTxCount       int64
	CombFeesInRune    int64
	CombVolumesInRune int64
}

func createPoolVolumeHistory(buckets []stat.SwapBucket) (*model.PoolVolumeHistory, error) {
	meta := &volumeMetaData{}

	result := &model.PoolVolumeHistory{
		Intervals: []*model.PoolVolumeHistoryBucket{},
	}

	for _, bucket := range buckets {
		ps := model.PoolVolumeHistoryBucket{
			Time: bucket.StartTime.ToI(),
			ToAsset: &model.VolumeStats{
				Count:        bucket.RuneToAssetCount,
				VolumeInRune: bucket.RuneToAssetVolume,
				FeesInRune:   0,
			},
			ToRune: &model.VolumeStats{
				Count:        bucket.RuneToAssetCount,
				VolumeInRune: bucket.AssetToRuneVolume,
				FeesInRune:   0,
			},
			Combined: &model.VolumeStats{
				Count:        bucket.TotalCount,
				VolumeInRune: bucket.TotalVolume,
				FeesInRune:   bucket.TotalFees,
			},
		}
		result.Intervals = append(result.Intervals, &ps)

		updateSwapMetadata(meta, bucket)
	}

	result.Meta = &model.PoolVolumeHistoryMeta{
		ToRune: &model.VolumeStats{
			Count:        meta.ToRuneTxCount,
			FeesInRune:   meta.ToRuneFeesInRune,
			VolumeInRune: meta.ToRuneVolumesInRune,
		},
		ToAsset: &model.VolumeStats{
			Count:        meta.ToAssetTxCount,
			FeesInRune:   meta.ToAssetFeesInRune,
			VolumeInRune: meta.ToAssetVolumesInRune,
		},
		Combined: &model.VolumeStats{
			Count:        meta.CombTxCount,
			FeesInRune:   meta.CombFeesInRune,
			VolumeInRune: meta.CombVolumesInRune,
		},
	}

	inv := result.Intervals
	if len(inv) > 0 {
		result.Meta.First = inv[0].Time
		result.Meta.Last = buckets[len(buckets)-1].EndTime.ToI()
	}

	return result, nil
}

func updateSwapMetadata(meta *volumeMetaData, bucket stat.SwapBucket) {
	meta.ToAssetTxCount += bucket.RuneToAssetCount
	meta.ToAssetFeesInRune += 0
	meta.ToAssetVolumesInRune += bucket.RuneToAssetVolume

	meta.ToRuneTxCount += bucket.AssetToRuneCount
	meta.ToRuneFeesInRune += 0
	meta.ToRuneVolumesInRune += bucket.AssetToRuneVolume

	meta.CombTxCount += bucket.TotalCount
	meta.CombFeesInRune += bucket.TotalFees
	meta.CombVolumesInRune += bucket.TotalVolume
}

func (r *queryResolver) PoolHistory(ctx context.Context, pool string, from *int64, until *int64, interval *model.Interval) (*model.PoolHistoryDetails, error) {
	window := setupDefaultParameters(from, until, interval)
	var err error
	buckets, err := db.BucketsFromWindow(ctx, window, toStatInterval[*interval])
	if err != nil {
		return nil, err
	}

	_, depthsArr, err := stat.PoolDepthHistory(ctx, buckets, pool)
	if err != nil {
		return nil, err
	}

	modelDepths := make([]*model.PoolHistoryBucket, 0, len(depthsArr))
	for _, v := range depthsArr {
		modelDepths = append(modelDepths,
			&model.PoolHistoryBucket{
				Time:  v.Window.From.ToI(),
				Rune:  v.Depths.RuneDepth,
				Asset: v.Depths.AssetDepth,
				Price: v.Depths.AssetPrice(),
			})
	}

	meta := model.PoolHistoryMeta{}
	if len(modelDepths) > 0 {
		first := modelDepths[0]
		last := modelDepths[len(modelDepths)-1]

		// Array is ORDERED by time. (see depth.go)
		meta.First = first.Time
		meta.RuneFirst = first.Rune
		meta.AssetFirst = first.Asset
		meta.PriceFirst = first.Price
		meta.Last = last.Time
		meta.RuneLast = last.Rune
		meta.AssetLast = last.Asset
		meta.PriceLast = last.Price
	}

	result := &model.PoolHistoryDetails{
		Meta:      &meta,
		Intervals: modelDepths,
	}

	return result, nil
}

func (r *queryResolver) StakeHistory(ctx context.Context, pool string, from *int64, until *int64, interval *model.Interval) (*model.PoolStakeHistory, error) {
	// TODO(acsaba): figure out which json api this should correspond to.
	return &model.PoolStakeHistory{}, nil
}

// Pool returns generated.PoolResolver implementation.
func (r *Resolver) Pool() generated.PoolResolver { return &poolResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type poolResolver struct{ *Resolver }

type queryResolver struct{ *Resolver }
