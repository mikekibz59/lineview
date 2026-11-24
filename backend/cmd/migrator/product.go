package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type product struct {
	TenantID  string    `db:"tenant_id"`
	ID        string    `db:"id"`
	SKU       string    `db:"sku"`
	Name      string    `db:"name"`
	UnitCost  float64   `db:"unit_cost"`
	Currency  string    `db:"currency"`
	CreatedAt time.Time `db:"created_at"`
}

var productColumns = []string{"tenant_id", "id", "sku", "name", "unit_cost", "currency", "created_at"}
var productBatchSize = 20

const productFilter = `tenant_id = $1::uuid AND created_at <= $2`

var productSpec = spec[product]{
	name:    PRODUCTS_TABLE,
	columns: productColumns,
	batchQuery: fmt.Sprintf(`
		SELECT id FROM (
			SELECT id, row_number() OVER (ORDER BY id ASC) AS rn
			FROM %s
			WHERE `+productFilter+`
		)
		WHERE rn %% %d = 1;`, PRODUCTS_TABLE.toString(), productBatchSize),
	fetchQuery: fmt.Sprintf(`
		SELECT %s FROM %s
		WHERE `+productFilter+` AND id > $3
		ORDER BY id ASC
		LIMIT %d`, strings.Join(productColumns, ", "), PRODUCTS_TABLE.toString(), productBatchSize),
	rowMapper: func(p product) []any {
		return []any{
			p.TenantID,
			p.ID,
			p.SKU,
			p.Name,
			p.UnitCost,
			p.Currency,
			p.CreatedAt,
		}
	},
}

func ProductTable(tenantID string, workers int, appPool, destPool *pgxpool.Pool) *Table[product] {
	opts, err := LoadOpts(tenantID, workers)
	if err != nil {
		log.Fatalf("Error loading product opts: %v", err)
	}

	return &Table[product]{
		spec:     productSpec,
		opts:     opts,
		appPool:  appPool,
		destPool: destPool,
	}
}
