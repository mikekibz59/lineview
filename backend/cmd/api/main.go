package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	appPool  *pgxpool.Pool
	authPool *pgxpool.Pool
	mode     TenantMode
	log      *slog.Logger
}

func newPool(ctx context.Context, url string, maxConns int32, name string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = maxConns
	cfg.ConnConfig.RuntimeParams["application_name"] = "lineview-api" + name
	return pgxpool.NewWithConfig(ctx, cfg)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.appPool.Ping(ctx); err != nil {
		http.Error(w, "app db down", http.StatusServiceUnavailable)
		return
	}
	if err := s.authPool.Ping(ctx); err != nil {
		http.Error(w, "auth db down", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok"))
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := loadConfig()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	appPool, err := newPool(ctx, cfg.AppURL, cfg.AppMaxConns, "app")
	if err != nil {
		log.Error("app pool", "err", err)
		os.Exit(1)
	}
	defer appPool.Close()

	authPool, err := newPool(ctx, cfg.AuthURL, cfg.AuthMaxConns, "auth")
	if err != nil {
		log.Error("auth pool", "err", err)
		os.Exit(1)
	}
	defer authPool.Close()

	s := Server{appPool: appPool, authPool: authPool, mode: cfg.Mode, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.Handle("GET /api/defect-rate/products", s.requireSession(http.HandlerFunc(s.defectRateByProduct)))

	srv := &http.Server{Addr: cfg.ListenAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	shutdownDone := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	log.Info("listening", "addr", cfg.ListenAddr, "tenant_mode", cfg.Mode)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server", "err", err)
		os.Exit(1)
	}
	<-shutdownDone
}
