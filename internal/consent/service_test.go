package consent

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/sreeni/openbank-bian/pkg/httpx"
	"github.com/sreeni/openbank-bian/pkg/obie"
)

// newTestService returns a service backed by an in-memory repo with a fixed
// clock and deterministic ids, so assertions are stable.
func newTestService() *Service {
	s := NewService(NewMemRepository())
	fixed := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	n := 0
	s.newID = func() string { n++; return "consent-" + itoa(n) }
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func wantStatus(t *testing.T, err error, status int) {
	t.Helper()
	var apiErr *httpx.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *httpx.APIError, got %v", err)
	}
	if apiErr.Status != status {
		t.Fatalf("status = %d, want %d (%s)", apiErr.Status, status, apiErr.Message)
	}
}

func TestCreateAccountAccess(t *testing.T) {
	ctx := context.Background()
	s := newTestService()

	c, err := s.CreateAccountAccess(ctx, AccountAccessInput{
		Permissions: []string{"ReadAccountsBasic", "ReadBalances"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.Status != StatusAwaitingAuthorisation || c.Type != TypeAccountAccess {
		t.Fatalf("unexpected consent %+v", c)
	}

	t.Run("rejects empty permissions", func(t *testing.T) {
		_, err := s.CreateAccountAccess(ctx, AccountAccessInput{})
		wantStatus(t, err, http.StatusBadRequest)
	})
	t.Run("rejects unknown permission", func(t *testing.T) {
		_, err := s.CreateAccountAccess(ctx, AccountAccessInput{Permissions: []string{"ReadEverything"}})
		wantStatus(t, err, http.StatusBadRequest)
	})
	t.Run("rejects past expiry", func(t *testing.T) {
		past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		_, err := s.CreateAccountAccess(ctx, AccountAccessInput{
			Permissions:        []string{"ReadBalances"},
			ExpirationDateTime: &past,
		})
		wantStatus(t, err, http.StatusBadRequest)
	})
	t.Run("rejects inverted transaction window", func(t *testing.T) {
		from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		_, err := s.CreateAccountAccess(ctx, AccountAccessInput{
			Permissions:             []string{"ReadBalances"},
			TransactionFromDateTime: &from,
			TransactionToDateTime:   &to,
		})
		wantStatus(t, err, http.StatusBadRequest)
	})
}

func TestCreateDomesticPayment(t *testing.T) {
	ctx := context.Background()
	s := newTestService()
	creditor := Account{SchemeName: "UK.OBIE.SortCodeAccountNumber", Identification: "08080021325698", Name: "ACME"}

	c, err := s.CreateDomesticPayment(ctx, DomesticPaymentInput{
		InstructionIdentification: "ID-1",
		EndToEndIdentification:    "E2E-1",
		InstructedAmount:          obie.MustAmount("165.88", "GBP"),
		CreditorAccount:           creditor,
		Reference:                 "INV-9",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.InstructedAmount.String() != "165.88" {
		t.Fatalf("amount = %s", c.InstructedAmount)
	}

	t.Run("requires instruction id", func(t *testing.T) {
		_, err := s.CreateDomesticPayment(ctx, DomesticPaymentInput{
			EndToEndIdentification: "E2E", InstructedAmount: obie.MustAmount("1", "GBP"), CreditorAccount: creditor,
		})
		wantStatus(t, err, http.StatusBadRequest)
	})
	t.Run("requires creditor identification", func(t *testing.T) {
		_, err := s.CreateDomesticPayment(ctx, DomesticPaymentInput{
			InstructionIdentification: "ID", EndToEndIdentification: "E2E",
			InstructedAmount: obie.MustAmount("1", "GBP"),
			CreditorAccount:  Account{SchemeName: "UK.OBIE.SortCodeAccountNumber"},
		})
		wantStatus(t, err, http.StatusBadRequest)
	})
}

func TestCreateFundsConfirmation(t *testing.T) {
	ctx := context.Background()
	s := newTestService()

	_, err := s.CreateFundsConfirmation(ctx, FundsConfirmationInput{
		DebtorAccount: Account{SchemeName: "UK.OBIE.SortCodeAccountNumber", Identification: "1111"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	t.Run("requires debtor account", func(t *testing.T) {
		_, err := s.CreateFundsConfirmation(ctx, FundsConfirmationInput{})
		wantStatus(t, err, http.StatusBadRequest)
	})
}

func TestGetTypeMismatchIsNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestService()
	c, _ := s.CreateAccountAccess(ctx, AccountAccessInput{Permissions: []string{"ReadBalances"}})

	// Reading an account-access consent through the payment type must 404.
	_, err := s.Get(ctx, c.ID, TypeDomesticPayment)
	wantStatus(t, err, http.StatusNotFound)
}

func TestAuthoriseConsumeAndView(t *testing.T) {
	ctx := context.Background()
	s := newTestService()
	c, _ := s.CreateDomesticPayment(ctx, DomesticPaymentInput{
		InstructionIdentification: "ID-1",
		EndToEndIdentification:    "E2E-1",
		InstructedAmount:          obie.MustAmount("10.00", "GBP"),
		CreditorAccount:           Account{SchemeName: "S", Identification: "I"},
	})

	// Cannot consume before authorisation.
	if _, err := s.Consume(ctx, c.ID); err == nil {
		t.Fatal("expected error consuming an unauthorised consent")
	}

	authd, err := s.Authorise(ctx, c.ID)
	if err != nil || authd.Status != StatusAuthorised {
		t.Fatalf("authorise: err=%v status=%s", err, authd.Status)
	}

	view, err := s.View(ctx, c.ID)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if view.Status != "Authorised" || view.InstructedAmount == nil || view.InstructedAmount.String() != "10" {
		t.Fatalf("unexpected view %+v", view)
	}

	consumed, err := s.Consume(ctx, c.ID)
	if err != nil || consumed.Status != StatusConsumed {
		t.Fatalf("consume: err=%v status=%s", err, consumed.Status)
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestService()
	c, _ := s.CreateAccountAccess(ctx, AccountAccessInput{Permissions: []string{"ReadBalances"}})

	if err := s.Delete(ctx, c.ID, TypeAccountAccess); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := s.Get(ctx, c.ID, TypeAccountAccess)
	wantStatus(t, err, http.StatusNotFound)
}
