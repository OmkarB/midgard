package stat_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/thorchain/midgard/config"
	"gitlab.com/thorchain/midgard/internal/db/testdb"
	"gitlab.com/thorchain/midgard/internal/timeseries"
	"gitlab.com/thorchain/midgard/openapi/generated/oapigen"
)

func TestUsdPrices(t *testing.T) {
	testdb.InitTest(t)
	timeseries.SetDepthsForTest([]timeseries.Depth{
		{Pool: "BNB.BNB", AssetDepth: 1000, RuneDepth: 2000},
		{Pool: "USDA", AssetDepth: 300, RuneDepth: 100},
		{Pool: "USDB", AssetDepth: 5000, RuneDepth: 1000},
	})

	config.Global.UsdPools = []string{"USDA", "USDB"}

	{
		body := testdb.CallJSON(t,
			"http://localhost:8080/v2/stats")

		var result oapigen.StatsData
		testdb.MustUnmarshal(t, body, &result)
		require.Equal(t, "5", result.RunePriceUSD)
	}

	{
		body := testdb.CallJSON(t,
			"http://localhost:8080/v2/pool/BNB.BNB/stats")

		var result oapigen.PoolStatsDetail
		testdb.MustUnmarshal(t, body, &result)
		require.Equal(t, "10", result.AssetPriceUSD)
	}

	{
		body := testdb.CallJSON(t,
			"http://localhost:8080/v2/pool/BNB.BNB")

		var result oapigen.PoolDetail
		testdb.MustUnmarshal(t, body, &result)
		require.Equal(t, "10", result.AssetPriceUSD)
	}
}

func TestPrices(t *testing.T) {
	testdb.InitTest(t)
	timeseries.SetDepthsForTest([]timeseries.Depth{
		{Pool: "BNB.BNB", AssetDepth: 1000, RuneDepth: 2000},
	})

	{
		body := testdb.CallJSON(t,
			"http://localhost:8080/v2/pool/BNB.BNB/stats")

		var result oapigen.PoolStatsDetail
		testdb.MustUnmarshal(t, body, &result)
		require.Equal(t, "2", result.AssetPrice)
	}

	{
		body := testdb.CallJSON(t,
			"http://localhost:8080/v2/pool/BNB.BNB")

		var result oapigen.PoolDetail
		testdb.MustUnmarshal(t, body, &result)
		require.Equal(t, "2", result.AssetPrice)
	}
}
