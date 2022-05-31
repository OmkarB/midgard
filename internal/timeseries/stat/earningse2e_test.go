package stat_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/thorchain/midgard/internal/db"
	"gitlab.com/thorchain/midgard/internal/db/testdb"
	"gitlab.com/thorchain/midgard/internal/util"
	"gitlab.com/thorchain/midgard/openapi/generated/oapigen"
)

// TODO(muninn): split up to separate tests, migrate to fakeblocks.
func TestEarningsHistoryE2E(t *testing.T) {
	testdb.InitTest(t)

	// Before Interval
	testdb.InsertUpdateNodeAccountStatusEvent(t,
		testdb.FakeNodeStatus{NodeAddr: "node1", Former: "Standby", Current: "Active"},
		"2020-09-02 12:00:00")
	testdb.InsertUpdateNodeAccountStatusEvent(t,
		testdb.FakeNodeStatus{NodeAddr: "node2", Former: "Standby", Current: "Active"},
		"2020-09-02 12:00:00")

	// 3rd of September
	testdb.InsertSwapEvent(t, testdb.FakeSwap{Pool: "BNB.BTCB-1DE", FromAsset: "THOR.RUNE", LiqFeeInRuneE8: 1, LiqFeeE8: 10, BlockTimestamp: "2020-09-03 12:00:00"})
	testdb.InsertSwapEvent(t, testdb.FakeSwap{Pool: "BNB.BTCB-1DE", FromAsset: "BNB.BTCB-1DE", LiqFeeInRuneE8: 2, LiqFeeE8: 2, BlockTimestamp: "2020-09-03 12:30:00"})
	testdb.InsertUpdateNodeAccountStatusEvent(t,
		testdb.FakeNodeStatus{NodeAddr: "node1", Former: "Active", Current: "Standby"},
		"2020-09-03 12:30:00")
	testdb.InsertRewardsEvent(t, 3, "2020-09-03 13:00:00")
	testdb.InsertRewardsEventEntry(t, 4, "BNB.BTCB-1DE", "2020-09-03 13:00:00")

	// 5th of September
	testdb.InsertSwapEvent(t, testdb.FakeSwap{Pool: "BNB.BNB", FromAsset: "THOR.RUNE", LiqFeeInRuneE8: 5, LiqFeeE8: 50, BlockTimestamp: "2020-09-05 12:00:00"})
	testdb.InsertSwapEvent(t, testdb.FakeSwap{Pool: "BNB.BNB", FromAsset: "BNB.BNB", LiqFeeInRuneE8: 6, LiqFeeE8: 6, BlockTimestamp: "2020-09-05 12:20:00"})
	testdb.InsertRewardsEvent(t, 7, "2020-09-05 13:05:00")
	testdb.InsertRewardsEventEntry(t, 8, "BNB.BNB", "2020-09-05 13:05:00")
	testdb.InsertUpdateNodeAccountStatusEvent(t,
		testdb.FakeNodeStatus{NodeAddr: "node3", Former: "Standby", Current: "Active"},
		"2020-09-05 14:00:00")
	testdb.InsertUpdateNodeAccountStatusEvent(t,
		testdb.FakeNodeStatus{NodeAddr: "node4", Former: "Standby", Current: "Active"},
		"2020-09-05 14:00:00")
	testdb.InsertUpdateNodeAccountStatusEvent(t,
		testdb.FakeNodeStatus{NodeAddr: "node5", Former: "Standby", Current: "Active"},
		"2020-09-05 14:00:00")

	// TODO(acsaba): the values reported change based on the from-to window. Fix.
	from := db.StrToSec("2020-09-03 00:00:00")
	to := db.StrToSec("2020-09-06 00:00:00")

	// Check all pools
	body := testdb.CallJSON(t, fmt.Sprintf(
		"http://localhost:8080/v2/history/earnings?interval=day&from=%d&to=%d", from, to))

	var jsonResult oapigen.EarningsHistoryResponse
	testdb.MustUnmarshal(t, body, &jsonResult)

	// Node count weights
	// 3 Sep
	expectedNodeCountWeight1 := 2 * (toUnix("2020-09-03 12:30:00") - toUnix("2020-09-03 00:00:00"))
	expectedNodeCountWeight2 := 1 * (db.StrToSec("2020-09-04 00:00:00") - db.StrToSec("2020-09-03 12:30:00")).ToI()

	// 4 Sep
	expectedNodeCountWeight3 := 1 * (db.StrToSec("2020-09-05 00:00:00") - db.StrToSec("2020-09-04 00:00:00")).ToI()

	// 5 Sep
	expectedNodeCountWeight4 := 1 * (db.StrToSec("2020-09-05 14:00:00") - db.StrToSec("2020-09-05 00:00:00")).ToI()
	expectedNodeCountWeight5 := 4 * (to - db.StrToSec("2020-09-05 14:00:00")).ToI()

	expectedNodeCountTotalWeight := expectedNodeCountWeight1 + expectedNodeCountWeight2 + expectedNodeCountWeight3 + expectedNodeCountWeight4 + expectedNodeCountWeight5

	// Meta
	expectedMetaLiquidityFees := util.IntStr(1 + 2 + 5 + 6)
	expectedMetaBondingEarnings := util.IntStr(3 + 7)
	expectedMetaLiquidityEarnings := util.IntStr(1 + 2 + 5 + 6 + 4 + 8)
	expectedMetaAvgNodeCount := floatStr2Digits(float64(expectedNodeCountTotalWeight) / float64(to-db.StrToSec("2020-09-03 00:00:00")))
	require.Equal(t, epochStr("2020-09-03 00:00:00"), jsonResult.Meta.StartTime)
	require.Equal(t, epochStr("2020-09-06 00:00:00"), jsonResult.Meta.EndTime)
	require.Equal(t, expectedMetaLiquidityFees, jsonResult.Meta.LiquidityFees)
	require.Equal(t, expectedMetaBondingEarnings, jsonResult.Meta.BondingEarnings)
	require.Equal(t, expectedMetaLiquidityEarnings, jsonResult.Meta.LiquidityEarnings)
	require.Equal(t, expectedMetaAvgNodeCount, jsonResult.Meta.AvgNodeCount)
	require.Equal(t, 2, len(jsonResult.Meta.Pools))
	for _, p := range jsonResult.Meta.Pools {
		switch p.Pool {
		case "BNB.BTCB-1DE":
			require.Equal(t, util.IntStr(4), p.Rewards)
			require.Equal(t, util.IntStr(1+2+4), p.Earnings)
		case "BNB.BNB":
			require.Equal(t, util.IntStr(8), p.Rewards)
			require.Equal(t, util.IntStr(5+6+8), p.Earnings)
		}
	}

	// Start and End times for intervals
	require.Equal(t, 3, len(jsonResult.Intervals))
	require.Equal(t, epochStr("2020-09-03 00:00:00"), jsonResult.Intervals[0].StartTime)
	require.Equal(t, epochStr("2020-09-04 00:00:00"), jsonResult.Intervals[0].EndTime)
	require.Equal(t, epochStr("2020-09-05 00:00:00"), jsonResult.Intervals[2].StartTime)
	require.Equal(t, util.IntStr(to.ToI()), jsonResult.Intervals[2].EndTime)

	// 3 Sep
	require.Equal(t, util.IntStr(1+2), jsonResult.Intervals[0].LiquidityFees)
	require.Equal(t, "3", jsonResult.Intervals[0].BondingEarnings)
	require.Equal(t, util.IntStr(1+2+4), jsonResult.Intervals[0].LiquidityEarnings)
	require.Equal(t, floatStr2Digits(float64(expectedNodeCountWeight1+expectedNodeCountWeight2)/float64(toUnix("2020-09-04 00:00:00")-toUnix("2020-09-03 00:00:00"))), jsonResult.Intervals[0].AvgNodeCount)
	for _, p := range jsonResult.Intervals[0].Pools {
		switch p.Pool {
		case "BNB.BTCB-1DE":
			require.Equal(t, util.IntStr(4), p.Rewards)
			require.Equal(t, util.IntStr(1+2+4), p.Earnings)
		case "BNB.BNB":
			require.Equal(t, util.IntStr(0), p.Rewards)
			require.Equal(t, util.IntStr(0), p.Earnings)
		}
	}

	// 4 Sep (nothing happened)
	require.Equal(t, "0", jsonResult.Intervals[1].LiquidityFees)
	require.Equal(t, "1.00", jsonResult.Intervals[1].AvgNodeCount)

	// 5 Sep
	require.Equal(t, util.IntStr(5+6), jsonResult.Intervals[2].LiquidityFees)
	require.Equal(t, "7", jsonResult.Intervals[2].BondingEarnings)
	require.Equal(t, util.IntStr(5+6+8), jsonResult.Intervals[2].LiquidityEarnings)
	require.Equal(t, floatStr2Digits(float64(expectedNodeCountWeight4+expectedNodeCountWeight5)/float64(to.ToI()-toUnix("2020-09-05 00:00:00"))), jsonResult.Intervals[2].AvgNodeCount)
	for _, p := range jsonResult.Intervals[2].Pools {
		switch p.Pool {
		case "BNB.BTCB-1DE":
			require.Equal(t, util.IntStr(0), p.Rewards)
			require.Equal(t, util.IntStr(0), p.Earnings)
		case "BNB.BNB":
			require.Equal(t, util.IntStr(8), p.Rewards)
			require.Equal(t, util.IntStr(5+6+8), p.Earnings)
		}
	}

	//////////
	// This is to test that "month" intervals (which produced by a different aggregating mechanism)
	// work as expected

	body = testdb.CallJSON(t, fmt.Sprintf(
		"http://localhost:8080/v2/history/earnings?interval=month&from=%d&to=%d", from, to))
	testdb.MustUnmarshal(t, body, &jsonResult)

	require.Equal(t, epochStr("2020-09-01 00:00:00"), jsonResult.Meta.StartTime)
	require.Equal(t, epochStr("2020-10-01 00:00:00"), jsonResult.Meta.EndTime)
	require.Equal(t, 1, len(jsonResult.Intervals))
	require.Equal(t, expectedMetaLiquidityFees, jsonResult.Meta.LiquidityFees)
	require.Equal(t, expectedMetaBondingEarnings, jsonResult.Meta.BondingEarnings)
	require.Equal(t, expectedMetaLiquidityEarnings, jsonResult.Meta.LiquidityEarnings)
	require.Equal(t, 2, len(jsonResult.Meta.Pools))

	//////////
	// This is to test that non-intervaled request (which produced by combining the materialized
	// view and raw table) work as expected

	from = db.StrToSec("2020-09-03 11:22:00")
	to = db.StrToSec("2020-09-05 13:20:00")

	body = testdb.CallJSON(t, fmt.Sprintf(
		"http://localhost:8080/v2/history/earnings?from=%d&to=%d", from, to))
	testdb.MustUnmarshal(t, body, &jsonResult)

	require.Equal(t, util.IntStr(from.ToI()), jsonResult.Meta.StartTime)
	require.Equal(t, util.IntStr(to.ToI()), jsonResult.Meta.EndTime)
	require.Equal(t, 0, len(jsonResult.Intervals))
	require.Equal(t, expectedMetaLiquidityFees, jsonResult.Meta.LiquidityFees)
	require.Equal(t, expectedMetaBondingEarnings, jsonResult.Meta.BondingEarnings)
	require.Equal(t, expectedMetaLiquidityEarnings, jsonResult.Meta.LiquidityEarnings)
	require.Equal(t, 2, len(jsonResult.Meta.Pools))
}

func TestEarningsNoActiveNode(t *testing.T) {
	testdb.SetupTestDB(t)

	testdb.MustExec(t, "DELETE FROM swap_events")
	testdb.MustExec(t, "DELETE FROM rewards_events")
	testdb.MustExec(t, "DELETE FROM rewards_event_entries")
	testdb.MustExec(t, "DELETE FROM update_node_account_status_events")

	// Call should not fail without any active nodes
	testdb.CallJSON(t, "http://localhost:8080/v2/history/earnings?interval=day&count=20")
}

func toUnix(str string) int64 {
	return db.StrToSec(str).ToI()
}

func floatStr2Digits(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func TestEarningsLiquidityFees(t *testing.T) {
	blocks := testdb.InitTestBlocks(t)

	blocks.NewBlock(t, "2010-01-01 00:00:00", testdb.AddLiquidity{
		Pool: "BNB.BNB", AssetAmount: 1000, RuneAmount: 2000,
	}, testdb.PoolActivate{Pool: "BNB.BNB"})

	// 3rd of September
	blocks.NewBlock(t, "2020-09-03 12:00:00", testdb.Swap{
		Pool:               "BNB.BTCB-1DE",
		EmitAsset:          "1 BNB.BTCB-1DE",
		Coin:               "1 THOR.RUNE",
		LiquidityFeeInRune: 1,
		LiquidityFee:       10,
	})
	blocks.NewBlock(t, "2020-09-03 12:30:00", testdb.Swap{
		Pool:               "BNB.BTCB-1DE",
		EmitAsset:          "1 THOR.RUNE",
		Coin:               "1 BNB.BTCB-1DE",
		LiquidityFeeInRune: 2,
		LiquidityFee:       2,
	})

	// 5th of September
	blocks.NewBlock(t, "2020-09-05 12:00:00", testdb.Swap{
		Pool:               "BNB.BNB",
		EmitAsset:          "1 BNB.BNB",
		Coin:               "1 THOR.RUNE",
		LiquidityFeeInRune: 5,
		LiquidityFee:       50,
	})
	blocks.NewBlock(t, "2020-09-05 12:20:00", testdb.Swap{
		Pool:               "BNB.BNB",
		EmitAsset:          "1 THOR.RUNE",
		Coin:               "1 BNB.BNB",
		LiquidityFeeInRune: 6,
		LiquidityFee:       6,
	})

	from := db.StrToSec("2020-09-03 00:00:00")
	to := db.StrToSec("2020-09-06 00:00:00")

	// Check all pools
	body := testdb.CallJSON(t, fmt.Sprintf(
		"http://localhost:8080/v2/history/earnings?interval=day&from=%d&to=%d", from, to))

	var jsonResult oapigen.EarningsHistoryResponse
	testdb.MustUnmarshal(t, body, &jsonResult)

	metaPools := map[string]oapigen.EarningsHistoryItemPool{}
	for _, p := range jsonResult.Meta.Pools {
		metaPools[p.Pool] = p
	}

	require.Equal(t, "2", metaPools["BNB.BTCB-1DE"].RuneLiquidityFees)
	require.Equal(t, "10", metaPools["BNB.BTCB-1DE"].AssetLiquidityFees)
	require.Equal(t, "3", metaPools["BNB.BTCB-1DE"].TotalLiquidityFeesRune)

	require.Equal(t, "6", metaPools["BNB.BNB"].RuneLiquidityFees)
	require.Equal(t, "50", metaPools["BNB.BNB"].AssetLiquidityFees)
	require.Equal(t, "11", metaPools["BNB.BNB"].TotalLiquidityFeesRune)
}
