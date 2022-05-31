package timeseries

import (
	"context"

	"gitlab.com/thorchain/midgard/internal/db"
	"gitlab.com/thorchain/midgard/openapi/generated/oapigen"
)

func GetBalance(ctx context.Context, address string) (*oapigen.Balance, error) {
	rows, err := db.Query(
		ctx,
		`SELECT
			amount_e8, asset
		FROM 
			midgard_agg.current_balances
		WHERE
			addr = $1
		`, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	coins := oapigen.Coins{}

	for rows.Next() {
		c := oapigen.Coin{}
		err := rows.Scan(&c.Amount, &c.Asset)
		if err != nil {
			return nil, err
		}
		coins = append(coins, c)
	}

	return &oapigen.Balance{Coins: coins}, nil
}
