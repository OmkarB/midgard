package stat

import (
	"context"

	"gitlab.com/thorchain/midgard/internal/db"
	"gitlab.com/thorchain/midgard/internal/util/miderr"
)

func SwitchedRune(ctx context.Context) (int64, error) {
	q := `SELECT COALESCE(SUM(mint_e8), 0) FROM switch_events`

	rows, err := db.Query(ctx, q)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	if !rows.Next() {
		return 0, miderr.InternalErr("switched query returned no results")
	}

	var ret int64
	err = rows.Scan(&ret)
	return ret, err
}
