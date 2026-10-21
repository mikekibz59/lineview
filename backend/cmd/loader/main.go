package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"sync/atomic"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

const batchSize = 50_000

// Inspections are spread over a fixed window so runs are comparable.
var (
	epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	span  = 270 * 24 * time.Hour
)

type machine struct {
	ID         uuid.UUID
	Code, Name string
	DefectRate float64
}

type product struct {
	ID       uuid.UUID
	SKU      string
	Name     string
	UnitCost float64
	Currency string
}

type tenant struct {
	ID       uuid.UUID
	Spec     tenantSpec
	Machines []machine
	Products []product
}

func pg(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }

func round2(x float64) float64 { return math.Round(x*100) / 100 }

// buildTenant creates the machines and products for one tenant.
// Most machines fail 1-4% of the time; the first n/10 are "lemons" at
// 10-20%, so "which machine fails most?" has a real answer.
func buildTenant(s tenantSpec) tenant {
	f := gofakeit.New(s.Seed)
	rng := rand.New(rand.NewPCG(s.Seed, 0xD1CE))
	t := tenant{ID: uuid.Must(uuid.NewV7()), Spec: s}

	kinds := []string{"Press", "Lathe", "Mill", "Extruder", "Welder", "Stamper"}
	lemons := max(1, s.Machines/10)
	for i := 0; i < s.Machines; i++ {
		rate := 0.01 + rng.Float64()*0.03
		if i < lemons {
			rate = 0.10 + rng.Float64()*0.10
		}
		t.Machines = append(t.Machines, machine{
			ID:         uuid.Must(uuid.NewV7()),
			Code:       fmt.Sprintf("M-%03d", i+1),
			Name:       fmt.Sprintf("%s %02d", kinds[rng.IntN(len(kinds))], i+1),
			DefectRate: rate,
		})
	}
	for i := 0; i < s.Products; i++ {
		t.Products = append(t.Products, product{
			ID:       uuid.Must(uuid.NewV7()),
			SKU:      fmt.Sprintf("SKU-%05d", i+1),
			Name:     f.ProductName(),
			UnitCost: round2(f.Price(2, 400)),
			Currency: s.Currency,
		})
	}
	return t
}

func insertDims(ctx context.Context, pool *pgxpool.Pool, ts []tenant) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var trows, mrows, prows [][]any
	for _, t := range ts {
		trows = append(trows, []any{pg(t.ID), t.Spec.Name})
		for _, m := range t.Machines {
			mrows = append(mrows, []any{pg(t.ID), pg(m.ID), m.Code, m.Name})
		}
		for _, p := range t.Products {
			prows = append(prows, []any{pg(t.ID), pg(p.ID), p.SKU, p.Name, p.UnitCost, p.Currency})
		}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"tenants"},
		[]string{"id", "name"}, pgx.CopyFromRows(trows)); err != nil {
		return fmt.Errorf("tenants: %w", err)
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"machines"},
		[]string{"tenant_id", "id", "machine_code", "name"}, pgx.CopyFromRows(mrows)); err != nil {
		return fmt.Errorf("machines: %w", err)
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"products"},
		[]string{"tenant_id", "id", "sku", "name", "unit_cost", "currency"}, pgx.CopyFromRows(prows)); err != nil {
		return fmt.Errorf("products: %w", err)
	}
	return tx.Commit(ctx)
}

// inspectionSource lazily generates rows for pgx.CopyFrom.
// The id column is omitted from the COPY, so the database fills it with uuidv7().
type inspectionSource struct {
	t    *tenant
	rng  *rand.Rand
	left int
	cur  []any
}

func (s *inspectionSource) Next() bool {
	if s.left == 0 {
		return false
	}
	s.left--
	m := &s.t.Machines[s.rng.IntN(len(s.t.Machines))]
	p := &s.t.Products[s.rng.IntN(len(s.t.Products))]
	outcome := "finished"
	if s.rng.Float64() < m.DefectRate {
		outcome = "deformed"
	}
	at := epoch.Add(time.Duration(s.rng.Int64N(int64(span))))
	s.cur = []any{pg(s.t.ID), pg(m.ID), pg(p.ID), outcome, at}
	return true
}
func (s *inspectionSource) Values() ([]any, error) { return s.cur, nil }
func (s *inspectionSource) Err() error             { return nil }

type job struct {
	t     *tenant
	batch int
	rows  int
}

func makeJobs(ts []tenant) []job {
	var jobs []job
	for i := range ts {
		t := &ts[i]
		for b := 0; b*batchSize < t.Spec.Rows; b++ {
			jobs = append(jobs, job{t, b, min(batchSize, t.Spec.Rows-b*batchSize)})
		}
	}
	return jobs
}

func loadFacts(ctx context.Context, pool *pgxpool.Pool, jobs []job, workers int, done *atomic.Int64) error {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)
	for _, j := range jobs {
		g.Go(func() error {
			src := &inspectionSource{
				t: j.t,
				// Seeded by (tenant, batch): identical data on every run.
				rng:  rand.New(rand.NewPCG(j.t.Spec.Seed, uint64(j.batch)+1)),
				left: j.rows,
			}
			n, err := pool.CopyFrom(gctx, pgx.Identifier{"inspection_results"},
				[]string{"tenant_id", "machine_id", "product_id", "outcome", "inspected_at"}, src)
			if err != nil {
				return fmt.Errorf("%s batch %d: %w", j.t.Spec.Name, j.batch, err)
			}
			done.Add(n)
			return nil
		})
	}
	return g.Wait()
}

func main() {
	workers := flag.Int("workers", 4, "concurrent COPY workers (= pool connections)")
	reset := flag.Bool("reset", false, "truncate all tables before loading")
	tokensDefault := os.Getenv("TOKENS_PATH")
	if tokensDefault == "" {
		tokensDefault = "tokens.json"
	}
	tokensPath := flag.String("tokens", tokensDefault, "where to write plaintext session tokens for load tests")
	flag.Parse()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://dev_user:dev_postgres@localhost:5432/lineview_db"
	}

	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Fatal(err)
	}
	cfg.MaxConns = int32(*workers + 1) // +1 for the housekeeping queries
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	if *reset {
		if _, err := pool.Exec(ctx, "TRUNCATE auth.sessions, auth.users, inspection_results, products, machines, tenants"); err != nil {
			log.Fatal(err)
		}
	}
	var existing int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM tenants").Scan(&existing); err != nil {
		log.Fatal(err)
	}
	if existing > 0 {
		log.Fatal("tables are not empty; rerun with -reset")
	}

	tenants := make([]tenant, len(specs))
	total := 0
	for i, s := range specs {
		tenants[i] = buildTenant(s)
		total += s.Rows
	}
	if err := insertDims(ctx, pool, tenants); err != nil {
		log.Fatal(err)
	}
	log.Printf("dimensions loaded")

	users := make([][]user, len(tenants))
	for i, t := range tenants {
		users[i] = buildUsers(t)
	}
	if err := insertAuth(ctx, pool, tenants, users); err != nil {
		log.Fatal(err)
	}
	if err := writeTokens(*tokensPath, tenants, users); err != nil {
		log.Fatal(err)
	}
	log.Printf("auth loaded; session tokens written to %s", *tokensPath)
	log.Printf("streaming %d inspection rows with %d workers", total, *workers)

	var done atomic.Int64
	stop := make(chan struct{})
	go func() {
		tk := time.NewTicker(2 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				d := done.Load()
				log.Printf("%d / %d rows (%.0f%%)", d, total, 100*float64(d)/float64(total))
			case <-stop:
				return
			}
		}
	}()

	start := time.Now()
	err = loadFacts(ctx, pool, makeJobs(tenants), *workers, &done)
	close(stop)
	if err != nil {
		log.Fatal(err)
	}
	elapsed := time.Since(start)
	log.Printf("loaded %d rows in %s (%.0f rows/s)", done.Load(), elapsed.Round(time.Millisecond),
		float64(done.Load())/elapsed.Seconds())

	if _, err := pool.Exec(ctx, "ANALYZE"); err != nil {
		log.Fatal(err)
	}

	rows, err := pool.Query(ctx, `
		SELECT t.name, count(*), count(*) FILTER (WHERE r.outcome = 'deformed')
		FROM inspection_results r JOIN tenants t ON t.id = r.tenant_id
		GROUP BY t.name ORDER BY 2 DESC`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var n, bad int
		if err := rows.Scan(&name, &n, &bad); err != nil {
			log.Fatal(err)
		}
		log.Printf("%-22s %9d rows, %5.2f%% deformed", name, n, 100*float64(bad)/float64(n))
	}
}
