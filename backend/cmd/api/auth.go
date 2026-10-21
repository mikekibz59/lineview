package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/mikekibz59/lineview/internal/authtoken"
)

type Session struct {
	TenantID string
	UserID   string
}

type ctxKey struct{}

func sessionFrom(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(ctxKey{}).(Session)
	return s, ok
}

// requireSession: cookie "session" -> hash -> auth.sessions (via lineview_auth) -> Session in ctx.
// This lookup deliberately uses authPool, never appPool. The timing hook for
// "session-lookup latency, separate from query time" goes around the QueryRow below.
func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session")
		if err != nil || c.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		h := authtoken.Hash(c.Value) // [32]byte, sha256 over the token's hex text
		var sess Session
		err = s.authPool.QueryRow(
			r.Context(),
			`
				SELECT tenant_id::text, user_id::text
				FROM auth.sessions
				WHERE token_hash = $1 AND expires_at > now();
			`,
			h[:]).Scan(&sess.TenantID, &sess.UserID)

		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if err != nil {
			s.log.Error("session lookup", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, sess)))
	})
}
