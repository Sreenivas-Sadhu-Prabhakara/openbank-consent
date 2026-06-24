//go:build integration

// Package testutil provides shared test infrastructure. It is only compiled
// under the `integration` build tag so the fast unit-test path never pulls in
// Docker/testcontainers.
package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresDSN starts a throwaway Postgres container and returns a connection
// DSN. The container is terminated automatically when the test finishes. Use
// it from integration tests:
//
//	dsn := testutil.PostgresDSN(t)
//	pool, _ := pg.Connect(ctx, dsn)
func PostgresDSN(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("openbank"),
		postgres.WithUsername("openbank"),
		postgres.WithPassword("openbank"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	return dsn
}
