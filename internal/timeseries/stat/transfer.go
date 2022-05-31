package stat

import (
	"context"

	"gitlab.com/thorchain/midgard/internal/db"
)

type PoolAdds struct {
	AssetE8Total int64
	RuneE8Total  int64
}

func PoolAddsLookup(ctx context.Context, pool string, w db.Window) (*PoolAdds, error) {
	const q = `SELECT COALESCE(SUM(asset_e8), 0), COALESCE(SUM(rune_e8), 0)
FROM add_events
WHERE pool = $1 AND block_timestamp >= $2 AND block_timestamp < $3`

	rows, err := db.Query(ctx, q, pool, w.From.ToNano(), w.Until.ToNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var r PoolAdds
	if rows.Next() {
		err := rows.Scan(&r.AssetE8Total, &r.RuneE8Total)
		if err != nil {
			return nil, err
		}
	}
	return &r, rows.Err()
}

type PoolErratas struct {
	AssetE8Total int64
	RuneE8Total  int64
}

func PoolErratasLookup(ctx context.Context, pool string, w db.Window) (*PoolErratas, error) {
	const q = `SELECT COALESCE(SUM(asset_e8), 0), COALESCE(SUM(rune_e8), 0) FROM errata_events
WHERE asset = $1 AND block_timestamp >= $2 AND block_timestamp < $3`

	rows, err := db.Query(ctx, q, pool, w.From.ToNano(), w.Until.ToNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var r PoolErratas
	if rows.Next() {
		err := rows.Scan(&r.AssetE8Total, &r.RuneE8Total)
		if err != nil {
			return nil, err
		}
	}
	return &r, rows.Err()
}

type PoolFees struct {
	AssetE8Total    int64
	AssetE8Avg      float64
	PoolDeductTotal int64
}

func PoolFeesLookup(ctx context.Context, pool string, w db.Window) (PoolFees, error) {
	const q = `SELECT COALESCE(SUM(asset_e8), 0), COALESCE(AVG(asset_E8), 0), COALESCE(SUM(pool_deduct), 0) FROM fee_events
WHERE asset = $1 AND block_timestamp >= $2 AND block_timestamp < $3`

	rows, err := db.Query(ctx, q, pool, w.From.ToNano(), w.Until.ToNano())
	if err != nil {
		return PoolFees{}, err
	}
	defer rows.Close()

	if !rows.Next() {
		return PoolFees{}, rows.Err()
	}

	var r PoolFees
	if err := rows.Scan(&r.AssetE8Total, &r.AssetE8Avg, &r.PoolDeductTotal); err != nil {
		return PoolFees{}, err
	}
	return r, rows.Err()
}

type PoolGas struct {
	AssetE8Total int64
	RuneE8Total  int64
}

func PoolGasLookup(ctx context.Context, pool string, w db.Window) (*PoolGas, error) {
	const q = `SELECT COALESCE(SUM(asset_e8), 0), COALESCE(SUM(rune_e8), 0)
FROM gas_events
WHERE asset = $1 AND block_timestamp >= $2 AND block_timestamp < $3`

	rows, err := db.Query(ctx, q, pool, w.From.ToNano(), w.Until.ToNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var r PoolGas
	if rows.Next() {
		err := rows.Scan(&r.AssetE8Total, &r.RuneE8Total)
		if err != nil {
			return nil, err
		}
	}
	return &r, rows.Err()
}

type PoolSlashes struct {
	AssetE8Total int64
}

func PoolSlashesLookup(ctx context.Context, pool string, w db.Window) (*PoolSlashes, error) {
	const q = `SELECT COALESCE(SUM(asset_e8), 0)
FROM slash_amounts
WHERE pool = $1 AND block_timestamp >= $2 AND block_timestamp < $3`

	rows, err := db.Query(ctx, q, pool, w.From.ToNano(), w.Until.ToNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var r PoolSlashes
	if rows.Next() {
		err := rows.Scan(&r.AssetE8Total)
		if err != nil {
			return nil, err
		}
	}
	return &r, rows.Err()
}
