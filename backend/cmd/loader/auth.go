package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikekibz59/lineview/internal/authtoken"
)

type user struct {
	ID    uuid.UUID
	Email string
	Name  string
	Token string // plaintext; only ever written to the tokens file
}

type tokenEntry struct {
	Tenant   string `json:"tenant"`
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
	Token    string `json:"token"`
}

// newToken returns 32 random bytes as hex. Tokens are the on thing that is
// NOT deterministic across runs, on purpose
func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func buildUsers(t tenant) []user {
	s := t.Spec
	f := gofakeit.New(s.Seed + 1000)
	users := make([]user, s.Users)
	for i := range users {
		users[i] = user{
			ID:    uuid.Must(uuid.NewV7()),
			Email: fmt.Sprintf("manager%03d@%s", i+1, s.Domain),
			Name:  f.Name(),
			Token: newToken(),
		}
	}
	return users
}

// insertAuth writes users and on long-lived session per user, atomically
func insertAuth(ctx context.Context, pool *pgxpool.Pool, ts []tenant, users [][]user) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	expires := time.Now().AddDate(1, 0, 0)
	var urows, srows [][]any
	for i, t := range ts {
		for _, u := range users[i] {
			urows = append(urows, []any{pg(t.ID), pg(u.ID), u.Email, u.Name})
			h := authtoken.Hash(u.Token)
			srows = append(srows, []any{h[:], pg(t.ID), pg(u.ID), expires})
		}
	}

	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"auth", "users"}, []string{"tenant_id", "id", "email", "display_name"}, pgx.CopyFromRows(urows)); err != nil {
		return fmt.Errorf("auth.users: %w", err)
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"auth", "sessions"}, []string{"token_hash", "tenant_id", "user_id", "expires_at"}, pgx.CopyFromRows(srows)); err != nil {
		return fmt.Errorf("auth.sessions: %w", err)
	}
	return tx.Commit(ctx)
}

// writeTokens dumps plaintext tokes for Locust. Mock credentials for a local database
// but the file isn't going to be in git.

func writeTokens(path string, ts []tenant, users [][]user) error {
	var out []tokenEntry
	for i, t := range ts {
		for _, u := range users[i] {
			out = append(out, tokenEntry{t.Spec.Name, t.ID.String(), u.ID.String(), u.Token})
		}
	}
	data, err := json.MarshalIndent(out, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
