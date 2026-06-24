package consent

import (
	"testing"
	"time"
)

func TestConsentLifecycleTransitions(t *testing.T) {
	now := time.Now()

	t.Run("awaiting can be authorised", func(t *testing.T) {
		c := &Consent{Status: StatusAwaitingAuthorisation}
		if err := c.Authorise(now); err != nil {
			t.Fatalf("authorise: %v", err)
		}
		if c.Status != StatusAuthorised {
			t.Fatalf("status = %s", c.Status)
		}
	})

	t.Run("cannot authorise twice", func(t *testing.T) {
		c := &Consent{Status: StatusAuthorised}
		if err := c.Authorise(now); err == nil {
			t.Fatal("expected error authorising an already-authorised consent")
		}
	})

	t.Run("awaiting can be rejected", func(t *testing.T) {
		c := &Consent{Status: StatusAwaitingAuthorisation}
		if err := c.Reject(now); err != nil || c.Status != StatusRejected {
			t.Fatalf("reject: err=%v status=%s", err, c.Status)
		}
	})

	t.Run("authorised can be revoked", func(t *testing.T) {
		c := &Consent{Status: StatusAuthorised}
		if err := c.Revoke(now); err != nil || c.Status != StatusRevoked {
			t.Fatalf("revoke: err=%v status=%s", err, c.Status)
		}
	})

	t.Run("rejected cannot be revoked", func(t *testing.T) {
		c := &Consent{Status: StatusRejected}
		if err := c.Revoke(now); err == nil {
			t.Fatal("expected error revoking a rejected consent")
		}
	})

	t.Run("only payment consents can be consumed", func(t *testing.T) {
		c := &Consent{Type: TypeAccountAccess, Status: StatusAuthorised}
		if err := c.Consume(now); err == nil {
			t.Fatal("expected error consuming a non-payment consent")
		}
		p := &Consent{Type: TypeDomesticPayment, Status: StatusAuthorised}
		if err := p.Consume(now); err != nil || p.Status != StatusConsumed {
			t.Fatalf("consume: err=%v status=%s", err, p.Status)
		}
	})
}

func TestConsentIsActive(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-time.Hour)

	cases := []struct {
		name   string
		status Status
		expiry *time.Time
		want   bool
	}{
		{"authorised no expiry", StatusAuthorised, nil, true},
		{"authorised future expiry", StatusAuthorised, &future, true},
		{"authorised expired", StatusAuthorised, &past, false},
		{"awaiting", StatusAwaitingAuthorisation, nil, false},
		{"revoked", StatusRevoked, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Consent{Status: tc.status, ExpirationDateTime: tc.expiry}
			if got := c.IsActive(now); got != tc.want {
				t.Fatalf("IsActive = %v, want %v", got, tc.want)
			}
		})
	}
}
