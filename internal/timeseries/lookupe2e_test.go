// End to end tests here are checkning lookup funcionality from Database to HTTP Api.

package timeseries_test

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/stretchr/testify/require"
	"gitlab.com/thorchain/midgard/internal/db"
	"gitlab.com/thorchain/midgard/internal/db/testdb"
	"gitlab.com/thorchain/midgard/internal/graphql"
	"gitlab.com/thorchain/midgard/internal/graphql/generated"
	"gitlab.com/thorchain/midgard/internal/graphql/model"

	"gitlab.com/thorchain/midgard/internal/timeseries"
	"gitlab.com/thorchain/midgard/openapi/generated/oapigen"
)

func callPools(t *testing.T, url string) map[string]oapigen.PoolDetail {
	body := testdb.CallJSON(t, url)

	var response oapigen.PoolsResponse
	testdb.MustUnmarshal(t, body, &response)
	sortedResp := map[string]oapigen.PoolDetail{}

	for _, poolDetail := range response {
		sortedResp[poolDetail.Asset] = poolDetail
	}
	return sortedResp
}

func callPoolsGraphql() map[string]model.Pool {
	schema := generated.NewExecutableSchema(generated.Config{Resolvers: &graphql.Resolver{}})
	gqlClient := client.New(handler.NewDefaultServer(schema))

	queryString := `{
	  pools(limit: 10) {
	  asset
	  depth {
		assetDepth
		runeDepth
	  }
	  poolAPY
	  price
	  status
	  units
      volume24h
	  }
	}`

	type Response struct {
		Pools []*model.Pool
	}
	var response Response
	gqlClient.MustPost(queryString, &response)

	sortedResp := map[string]model.Pool{}

	for _, poolDetail := range response.Pools {
		sortedResp[poolDetail.Asset] = *poolDetail
	}
	return sortedResp
}

func TestPoolsE2E(t *testing.T) {
	testdb.InitTest(t)

	testdb.InsertStakeEvent(t, testdb.FakeStake{Pool: "BNB.BNB", BlockTimestamp: "2020-01-01 00:00:00"})
	testdb.InsertStakeEvent(t, testdb.FakeStake{Pool: "POOL2"})
	testdb.InsertStakeEvent(t, testdb.FakeStake{Pool: "POOL3"})

	testdb.InsertPoolEvents(t, "BNB.BNB", "Available")
	testdb.InsertPoolEvents(t, "POOL2", "Available")
	testdb.InsertPoolEvents(t, "POOL3", "Staged")

	depths := []timeseries.Depth{
		{"BNB.BNB", 2, 1, 0},
		{"POOL2", 2, 1, 0},
		{"POOL3", 2, 1, 0},
	}
	timeseries.SetDepthsForTest(depths)

	sortedResp := callPools(t, "http://localhost:8080/v2/pools")
	sortedRespGraphql := callPoolsGraphql()

	require.Equal(t, len(sortedResp), 3)
	require.Equal(t, len(sortedRespGraphql), 3)
	require.Equal(t, sortedResp["POOL2"].AssetDepth, "2")
	require.Equal(t, sortedRespGraphql["POOL2"].Depth.AssetDepth, int64(2))
	require.Equal(t, sortedResp["POOL2"].RuneDepth, "1")
	require.Equal(t, sortedRespGraphql["POOL2"].Depth.RuneDepth, int64(1))
	require.Equal(t, sortedResp["POOL2"].AssetPrice, "0.5")
	require.Equal(t, sortedRespGraphql["POOL2"].Price, 0.5)
	_, has_pool3 := sortedResp["POOL3"]
	require.Equal(t, has_pool3, true) // Without filter we have the Staged pool
	_, has_pool3_graphql := sortedRespGraphql["POOL3"]
	require.Equal(t, has_pool3_graphql, true)

	// check filtering
	sortedResp = callPools(t, "http://localhost:8080/v2/pools?status=available")
	require.Equal(t, len(sortedResp), 2)
	_, has_pool3 = sortedResp["POOL3"]
	require.Equal(t, has_pool3, false)

	// Check bad requests fail.
	testdb.JSONFailGeneral(t, "http://localhost:8080/v2/pools?status=badname")
}

func TestGenesisNodeGoesOut(t *testing.T) {
	testdb.InitTest(t)
	testdb.InsertUpdateNodeAccountStatusEvent(t,
		testdb.FakeNodeStatus{NodeAddr: "node1", Former: "Standby", Current: "Active"},
		"2020-09-02 12:00:00")
	testdb.InsertUpdateNodeAccountStatusEvent(t,
		testdb.FakeNodeStatus{NodeAddr: "node2", Former: "Standby", Current: "Active"},
		"2020-09-02 12:00:00")
	testdb.InsertUpdateNodeAccountStatusEvent(t,
		testdb.FakeNodeStatus{NodeAddr: "genesisNode", Former: "Active", Current: "Standby"},
		"2020-09-03 12:00:00")

	n, err := timeseries.ActiveNodeCount(context.Background(),
		db.StrToSec("2020-09-10 12:00:00").ToNano())
	require.NoError(t, err)
	require.Equal(t, int64(2), n)
}
