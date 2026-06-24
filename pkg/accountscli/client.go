// Package accountscli is the client the CBPII (funds) service uses to ask the
// AIS (accounts) service whether a debtor account holds sufficient available
// funds. The accounts service is the single source of truth for balances, so
// funds never stores balance data itself.
package accountscli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/sreeni/openbank-bian/pkg/obie"
)

// ErrAccountNotFound is returned when the accounts service does not recognise
// the supplied account identification.
var ErrAccountNotFound = fmt.Errorf("account not found")

// Client calls the accounts service's internal funds-confirmation endpoint.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client targeting the accounts service base URL.
func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: 5 * time.Second}}
}

// fundsResponse is the internal endpoint's JSON body.
type fundsResponse struct {
	FundsAvailable bool `json:"FundsAvailable"`
}

// FundsAvailable reports whether the account identified by identification has
// at least amount available. It calls
// GET {base}/internal/funds-confirmation?identification=..&amount=..&currency=..
func (c *Client) FundsAvailable(ctx context.Context, identification string, amount obie.Amount) (bool, error) {
	q := url.Values{}
	q.Set("identification", identification)
	q.Set("amount", amount.String())
	q.Set("currency", amount.Currency)
	endpoint := fmt.Sprintf("%s/internal/funds-confirmation?%s", c.baseURL, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("call accounts service: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var fr fundsResponse
		if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
			return false, fmt.Errorf("decode funds response: %w", err)
		}
		return fr.FundsAvailable, nil
	case http.StatusNotFound:
		return false, ErrAccountNotFound
	default:
		return false, fmt.Errorf("accounts service returned %d", resp.StatusCode)
	}
}
