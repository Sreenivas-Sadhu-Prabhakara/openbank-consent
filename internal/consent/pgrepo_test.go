//go:build integration

package consent

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sreeni/openbank-bian/pkg/obie"
	"github.com/sreeni/openbank-bian/pkg/pg"
	"github.com/sreeni/openbank-bian/pkg/testutil"
)

// newPgRepo spins up a throwaway Postgres, applies migrations and returns a
// Postgres-backed repository. Migrations are read from the module's migrations
// directory relative to this test package.
func newPgRepo(t *testing.T) *PgRepository {
	t.Helper()
	ctx := context.Background()
	dsn := testutil.PostgresDSN(t)

	pool, err := pg.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pg.RunMigrations(ctx, pool, os.DirFS("../.."), "migrations", "consent"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewPgRepository(pool)
}

func TestPgRepositoryAccountAccessRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := newPgRepo(t)

	exp := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	c := &Consent{
		ID:                   "c-aac-1",
		Type:                 TypeAccountAccess,
		Status:               StatusAwaitingAuthorisation,
		CreationDateTime:     time.Now().UTC().Truncate(time.Second),
		StatusUpdateDateTime: time.Now().UTC().Truncate(time.Second),
		Permissions:          []string{"ReadAccountsBasic", "ReadBalances"},
		ExpirationDateTime:   &exp,
	}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Type != TypeAccountAccess || len(got.Permissions) != 2 || got.ExpirationDateTime == nil {
		t.Fatalf("unexpected consent %+v", got)
	}

	// Update status and confirm persistence.
	got.Status = StatusAuthorised
	got.StatusUpdateDateTime = time.Now().UTC().Truncate(time.Second)
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reloaded, _ := repo.Get(ctx, c.ID)
	if reloaded.Status != StatusAuthorised {
		t.Fatalf("status = %s", reloaded.Status)
	}

	if err := repo.Delete(ctx, c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, c.ID); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPgRepositoryPaymentAmountRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := newPgRepo(t)

	amt := obie.MustAmount("165.88", "GBP")
	c := &Consent{
		ID:                        "c-pay-1",
		Type:                      TypeDomesticPayment,
		Status:                    StatusAwaitingAuthorisation,
		CreationDateTime:          time.Now().UTC().Truncate(time.Second),
		StatusUpdateDateTime:      time.Now().UTC().Truncate(time.Second),
		InstructionIdentification: "ID412",
		EndToEndIdentification:    "E2E412",
		InstructedAmount:          &amt,
		CreditorAccount:           &Account{SchemeName: "UK.OBIE.SortCodeAccountNumber", Identification: "0808", Name: "ACME"},
		Reference:                 "INV-9",
	}
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.InstructedAmount == nil || got.InstructedAmount.String() != "165.88" {
		t.Fatalf("amount = %v", got.InstructedAmount)
	}
	if got.CreditorAccount == nil || got.CreditorAccount.Name != "ACME" {
		t.Fatalf("creditor = %+v", got.CreditorAccount)
	}
	if got.Reference != "INV-9" {
		t.Fatalf("reference = %s", got.Reference)
	}
}

func TestPgRepositoryGetMissing(t *testing.T) {
	repo := newPgRepo(t)
	if _, err := repo.Get(context.Background(), "nope"); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
