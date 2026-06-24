// Package consentcli is the client the AIS/PIS/CBPII services use to validate
// consents held by the consent service. Centralising consent in one BIAN
// "Consent" service domain means the functional services never trust the
// caller's claimed consent directly — they fetch and check it here.
package consentcli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sreeni/openbank-bian/pkg/obie"
)

// Consent types, mirrored from the consent service's domain. Kept as plain
// strings so this shared package has no dependency on any service.
const (
	TypeAccountAccess     = "account-access"
	TypeDomesticPayment   = "domestic-payment"
	TypeFundsConfirmation = "funds-confirmation"
)

// Consent statuses per the OBIE consent lifecycle.
const (
	StatusAwaitingAuthorisation = "AwaitingAuthorisation"
	StatusAuthorised            = "Authorised"
	StatusRejected              = "Rejected"
	StatusRevoked               = "Revoked"
	StatusConsumed              = "Consumed"
)

// View is the internal projection of a consent that functional services need
// to authorise a request. It is returned by the consent service's
// GET /internal/consents/{id} endpoint and is deliberately smaller than the
// public OBIE consent resource.
type View struct {
	ConsentID        string       `json:"ConsentId"`
	Type             string       `json:"Type"`
	Status           string       `json:"Status"`
	Permissions      []string     `json:"Permissions,omitempty"`
	DebtorAccountID  string       `json:"DebtorAccountId,omitempty"`
	InstructedAmount *obie.Amount `json:"InstructedAmount,omitempty"`
	CreditorName     string       `json:"CreditorName,omitempty"`
	ExpiresAt        string       `json:"ExpirationDateTime,omitempty"`
}

// HasPermission reports whether the consent grants the named AIS permission
// (e.g. "ReadTransactionsDetail").
func (v View) HasPermission(p string) bool {
	for _, perm := range v.Permissions {
		if perm == p {
			return true
		}
	}
	return false
}

// Client calls the consent service's internal API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client targeting the consent service base URL (e.g.
// "http://consent:8081").
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// ErrNotFound is returned when the consent id is unknown to the consent service.
var ErrNotFound = fmt.Errorf("consent not found")

// Get fetches the consent view for id. It returns ErrNotFound for a 404 so
// callers can map it to the correct OBIE error.
func (c *Client) Get(ctx context.Context, id string) (*View, error) {
	url := fmt.Sprintf("%s/internal/consents/%s", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call consent service: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var v View
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			return nil, fmt.Errorf("decode consent view: %w", err)
		}
		return &v, nil
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("consent service returned %d", resp.StatusCode)
	}
}

// Consume marks a domestic-payment consent as used. The payments service calls
// this once it accepts a payment, enforcing single-use payment consents. A 404
// is returned as ErrNotFound; a non-2xx for an already-consumed consent is
// surfaced as an error so the caller can reject the duplicate.
func (c *Client) Consume(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/internal/consents/%s/consume", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call consent service: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return fmt.Errorf("consent service returned %d consuming consent", resp.StatusCode)
	}
}
