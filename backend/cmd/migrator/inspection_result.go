package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type inspectionResult struct {
	TenantID    string    `db:"tenant_id"`
	ID          string    `db:"id"`
	MachineID   string    `db:"machine_id"`
	ProductID   string    `db:"product_id"`
	Outcome     string    `db:"outcome"`
	InspectedAt time.Time `db:"inspected_at"`
}

var inpectionResultColumns = []string{"tenant_id", "id", "machine_id", "product_id", "outcome", "inspected_at"}

const inspectionResultBatchSize = 10_000
const inspectionResultFilter = `tenant_id=$1::uuid AND inspected_at <= $2`

var inspectionResultSpec = spec[inspectionResult]{
	name:    INSPECTION_RESULTS_TABLE,
	columns: inpectionResultColumns,
	batchQuery: fmt.Sprintf(`
			SELECT id FROM (
				SELECT id, row_number() OVER (ORDER BY id ASC) AS rn
				FROM %s
				WHERE `+inspectionResultFilter+`
			)
			WHERE rn %% %d=1;
		`, INSPECTION_RESULTS_TABLE.toString(), inspectionResultBatchSize),
	fetchQuery: fmt.Sprintf(`
			SELECT %s FROM %s
			WHERE `+inspectionResultFilter+` AND id > $3
			ORDER BY id ASC
			LIMIT %d;
		`, strings.Join(inpectionResultColumns, ","), INSPECTION_RESULTS_TABLE.toString()),
	rowMapper: func(ir inspectionResult) []any {
		return []any{
			ir.TenantID,
			ir.ID,
			ir.MachineID,
			ir.ProductID,
			ir.Outcome,
			ir.InspectedAt,
		}
	},
}

func InspectionResultTable(tenantID string, workers int, appPool, destPool *pgxpool.Pool) *Table[inspectionResult] {
	opts, err := LoadOpts(tenantID, workers)
	if err != nil {
		log.Fatalf("Error loading %s opts: %v", inspectionResultSpec.name, err)
	}
	return &Table[inspectionResult]{
		spec:     inspectionResultSpec,
		opts:     opts,
		appPool:  appPool,
		destPool: destPool,
	}
}
