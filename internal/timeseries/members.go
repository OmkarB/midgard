package timeseries

import (
	"context"

	"gitlab.com/thorchain/midgard/internal/db"
	"gitlab.com/thorchain/midgard/internal/util"
	"gitlab.com/thorchain/midgard/openapi/generated/oapigen"
)

// GetMemberIds returns the ids of all known members.
//
// The id of a member is defined as their rune address if they are participating with their rune
// address, or as their asset address otherwise (for members with asset address only.)
//
// Member ids present in multiple pools will be only returned once.
func GetMemberIds(ctx context.Context, pool *string) (addrs []string, err error) {
	poolFilter := ""
	qargs := []interface{}{}
	if pool != nil {
		poolFilter = "pool = $1"
		qargs = append(qargs, pool)
	}

	q := "SELECT DISTINCT member_id FROM midgard_agg.members " + db.Where(poolFilter)

	rows, err := db.Query(ctx, q, qargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var member string
		err := rows.Scan(&member)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, member)
	}

	return addrs, nil
}

// Info of a member in a specific pool.
type MemberPool struct {
	Pool           string
	RuneAddress    string
	AssetAddress   string
	LiquidityUnits int64
	RuneAdded      int64
	AssetAdded     int64
	RunePending    int64
	AssetPending   int64
	DateFirstAdded int64
	DateLastAdded  int64
	RuneWithdrawn  int64
	AssetWithdrawn int64
}

func (memberPool MemberPool) toOapigen() oapigen.MemberPool {
	return oapigen.MemberPool{
		Pool:           memberPool.Pool,
		RuneAddress:    memberPool.RuneAddress,
		AssetAddress:   memberPool.AssetAddress,
		LiquidityUnits: util.IntStr(memberPool.LiquidityUnits),
		RuneAdded:      util.IntStr(memberPool.RuneAdded),
		AssetAdded:     util.IntStr(memberPool.AssetAdded),
		RuneWithdrawn:  util.IntStr(memberPool.RuneWithdrawn),
		AssetWithdrawn: util.IntStr(memberPool.AssetWithdrawn),
		RunePending:    util.IntStr(memberPool.RunePending),
		AssetPending:   util.IntStr(memberPool.AssetPending),
		DateFirstAdded: util.IntStr(memberPool.DateFirstAdded),
		DateLastAdded:  util.IntStr(memberPool.DateLastAdded),
	}
}

// Pools data associated with a single member
type MemberPools []MemberPool

func (memberPools MemberPools) ToOapigen() []oapigen.MemberPool {
	ret := make([]oapigen.MemberPool, len(memberPools))
	for i, memberPool := range memberPools {
		ret[i] = memberPool.toOapigen()
	}

	return ret
}

func GetMemberPools(ctx context.Context, address string) (MemberPools, error) {
	q := `
		SELECT
			pool,
			COALESCE(rune_addr, ''),
			COALESCE(asset_addr, ''),
			lp_units_total,
			added_rune_e8_total,
			added_asset_e8_total,
			withdrawn_rune_e8_total,
			withdrawn_asset_e8_total,
			pending_rune_e8_total,
			pending_asset_e8_total,
			COALESCE(first_added_timestamp / 1000000000, 0),
			COALESCE(last_added_timestamp / 1000000000, 0)
		FROM midgard_agg.members
		WHERE member_id = $1 OR asset_addr = $1
		ORDER BY pool
	`

	rows, err := db.Query(ctx, q, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results MemberPools
	for rows.Next() {
		var entry MemberPool
		err := rows.Scan(
			&entry.Pool,
			&entry.RuneAddress,
			&entry.AssetAddress,
			&entry.LiquidityUnits,
			&entry.RuneAdded,
			&entry.AssetAdded,
			&entry.RuneWithdrawn,
			&entry.AssetWithdrawn,
			&entry.RunePending,
			&entry.AssetPending,
			&entry.DateFirstAdded,
			&entry.DateLastAdded,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, entry)
	}
	return results, nil
}
