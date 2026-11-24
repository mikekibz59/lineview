package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type machine struct {
	TenantID    string    `db:"tenant_id"`
	ID          string    `db:"id"`
	MachineCode string    `db:"machine_code"`
	Name        string    `db:"name"`
	CreatedAt   time.Time `db:"created_at"`
}

var machineColumns = []string{"tenant_id", "id", "machine_code", "name", "created_at"}
var machineBatchSize = 20

const machineFilter = `tenant_id=$1::uuid AND created_at <= $2`

var machineSpec = spec[machine]{
	name:    MACHINES_TABLE,
	columns: machineColumns,
	batchQuery: fmt.Sprintf(`
			SELECT id FROM (
				SELECT id, row_number() OVER (ORDER BY id ASC) AS rn
				FROM %s
				WHERE `+machineFilter+`
			)
			WHERE rn %% %d=1;
		`, MACHINES_TABLE.toString(), machineBatchSize),
	fetchQuery: fmt.Sprintf(`
			SELECT %s FROM %s
			WHERE `+machineFilter+` AND id > $3
		 	ORDER BY id ASC
			LIMIT %d
		`, strings.Join(machineColumns, ","), MACHINES_TABLE.toString(), machineBatchSize),
	rowMapper: func(m machine) []any {
		return []any{
			m.TenantID,
			m.ID,
			m.MachineCode,
			m.Name,
			m.CreatedAt,
		}
	},
}

func MachineTable(tenantID string, workers int, appPool, destPool *pgxpool.Pool) *Table[machine] {
	opts, err := LoadOpts(tenantID, workers)
	if err != nil {
		log.Fatalf("Error loading machine opts: %v", err)
	}
	return &Table[machine]{
		spec:     machineSpec,
		opts:     opts,
		appPool:  appPool,
		destPool: destPool,
	}
}
