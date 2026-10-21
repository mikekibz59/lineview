package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (s *Server) withTenant(ctx context.Context, tenantID string, fn func(q Querier) error) error {
	if s.mode == ModeUnsafe {
		return s.withTenantUnsafe(ctx, tenantID, fn)
	}
	return s.withTenantTx(ctx, tenantID, fn)
}

// withTenantTx: one connection, one transaction, settings scoped to that transaction
// set_config(..., true) means is_local: it reverts at COMMIT/ROLLBACK, so nothing
// survives on the connection when it returns to the pool.
func (s *Server) withTenantTx(ctx context.Context, tenantID string, fn func(q Querier) error) error {
	tx, err := s.appPool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// withTenantUnsafe: the bug in the blog. The set and the query are two separate
// pool checkouts, so they usually run on DIFFERENT connections, and the setting is
// session-level (is_local=false), so it sticks to whichever connection it landed on.
// Under concurrency, a query can run on a connection last set by another tenant.
func (s *Server) withTenantUnsafe(ctx context.Context, tenantID string, fn func(q Querier) error) error {
	if _, err := s.appPool.Exec(ctx, `SELECT set_config('app.tenant_id', $1, false)`, tenantID); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	return fn(s.appPool)
}
