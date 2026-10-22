package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type productDefectRow struct {
	TenantID   string  `json:"tenant_id" db:"tenant_id"`
	ProductID  string  `json:"product_id" db:"product_id"`
	SKU        string  `json:"sku" db:"sku"`
	Name       string  `json:"product_name" db:"product_name"`
	Inspected  int64   `json:"inspected" db:"inspected"`
	Deformed   int64   `json:"deformed" db:"deformed"`
	DefectRate float64 `json:"defect_rate" db:"defect_rate"`
}

const defectRateByProductSQL = `
WITH agg AS (
	SELECT 	tenant_id,
			product_id,
			count(*)									 AS inspected,
			count(*) FILTER (WHERE outcome = 'deformed') AS deformed
	FROM inspection_results
	WHERE inspected_at >= $1 AND inspected_at < $2
	GROUP BY tenant_id, product_id
)

SELECT 	a.tenant_id::text,
		a.product_id::text,
		p.sku,
		p.name As product_name,
		a.inspected,
		a.deformed,
		a.deformed::float8 / a.inspected AS defect_rate
FROM agg a
JOIN products p ON p.tenant_id = a.tenant_id AND p.id = a.product_id
ORDER BY defect_rate DESC;
`

func (s *Server) defectRateByProduct(w http.ResponseWriter, r *http.Request) {
	sess, _ := sessionFrom(r.Context())

	from, to := time.Unix(0, 0).UTC(), time.Now().UTC().Add(24*time.Hour)
	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			http.Error(w, "from must be RFC3339", http.StatusBadRequest)
			return
		}
		from = t
	}

	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			http.Error(w, "to must be RFC3339", http.StatusBadRequest)
			return
		}
		to = t
	}

	var out []productDefectRow
	err := s.withTenant(r.Context(), sess.TenantID, func(q Querier) error {
		rows, err := q.Query(r.Context(), defectRateByProductSQL, from, to)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowToStructByName[productDefectRow])
		return err
	})

	if err != nil {
		s.log.Error("defect rate by product", "err", err, "tenant", sess.TenantID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
