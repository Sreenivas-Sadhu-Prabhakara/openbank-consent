// Command consent runs the BIAN Consent service: the OBIE consent lifecycle
// for account-access, domestic-payment and funds-confirmation consents.
package main

import (
	"context"
	"embed"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sreeni/openbank-bian/pkg/httpx"
	"github.com/sreeni/openbank-bian/pkg/pg"
	"github.com/sreeni/openbank-bian/services/consent/internal/consent"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	addr := envOr("ADDR", ":8081")
	baseURL := envOr("BASE_URL", "http://localhost:8081")
	dsn := os.Getenv("DATABASE_URL")

	repo, err := newRepository(context.Background(), log, dsn)
	if err != nil {
		log.Error("init repository", "error", err)
		os.Exit(1)
	}

	svc := consent.NewService(repo)
	handler := consent.NewHandler(svc, baseURL)

	root := httpx.Chain(handler.Routes(),
		httpx.FAPIInteractionID,
		httpx.Logger(log),
		httpx.Recoverer(log),
	)

	srv := &http.Server{
		Addr:              addr,
		Handler:           root,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("consent service listening", "addr", addr, "backend", backendName(dsn))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	shutdownOnSignal(log, srv)
}

// newRepository returns a Postgres repository when DATABASE_URL is set,
// otherwise an in-memory repository so the service runs with zero infra.
func newRepository(ctx context.Context, log *slog.Logger, dsn string) (consent.Repository, error) {
	if dsn == "" {
		log.Warn("DATABASE_URL not set, using in-memory store")
		return consent.NewMemRepository(), nil
	}
	pool, err := pg.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pg.RunMigrations(ctx, pool, migrations, "migrations", "consent"); err != nil {
		return nil, err
	}
	return consent.NewPgRepository(pool), nil
}

func shutdownOnSignal(log *slog.Logger, srv *http.Server) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", "error", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func backendName(dsn string) string {
	if dsn == "" {
		return "memory"
	}
	return "postgres"
}
