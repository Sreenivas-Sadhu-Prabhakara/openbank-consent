package consentcli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGetAuthorisedConsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/consents/consent-1" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ConsentId":"consent-1",
			"Type":"account-access",
			"Status":"Authorised",
			"Permissions":["ReadAccountsBasic","ReadTransactionsDetail"]
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	v, err := c.Get(context.Background(), "consent-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v.Status != StatusAuthorised {
		t.Fatalf("status = %q", v.Status)
	}
	if !v.HasPermission("ReadTransactionsDetail") {
		t.Fatal("expected ReadTransactionsDetail permission")
	}
	if v.HasPermission("ReadParty") {
		t.Fatal("did not expect ReadParty permission")
	}
}

func TestClientGetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Get(context.Background(), "missing")
	if err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
