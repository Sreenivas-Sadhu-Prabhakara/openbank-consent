// Package consent implements the BIAN "Consent" service domain. It owns the
// full OBIE consent lifecycle for all three consent types used across the
// estate: account-access (AIS), domestic-payment (PIS) and funds-confirmation
// (CBPII). Functional services never store consent themselves — they call this
// service to create, authorise and validate it.
package consent

import (
	"fmt"
	"time"

	"github.com/sreeni/openbank-bian/pkg/obie"
)

// Type enumerates the consent kinds this service manages.
type Type string

const (
	TypeAccountAccess     Type = "account-access"
	TypeDomesticPayment   Type = "domestic-payment"
	TypeFundsConfirmation Type = "funds-confirmation"
)

// Status is the OBIE consent status. The lifecycle is:
//
//	AwaitingAuthorisation ──▶ Authorised ──▶ Consumed   (payment consents)
//	          │                    │
//	          ▼                    ▼
//	       Rejected             Revoked
type Status string

const (
	StatusAwaitingAuthorisation Status = "AwaitingAuthorisation"
	StatusAuthorised            Status = "Authorised"
	StatusRejected              Status = "Rejected"
	StatusRevoked               Status = "Revoked"
	StatusConsumed              Status = "Consumed"
)

// Account is the OBIE account identifier block shared by debtor and creditor
// accounts on payment and funds-confirmation consents.
type Account struct {
	SchemeName     string
	Identification string
	Name           string
}

// Consent is the aggregate root. It is a union across the three consent types:
// the AIS fields apply to account-access, the payment fields to
// domestic-payment, and DebtorAccount+ExpirationDateTime to funds-confirmation.
type Consent struct {
	ID                   string
	Type                 Type
	Status               Status
	CreationDateTime     time.Time
	StatusUpdateDateTime time.Time

	// Account-access (AIS) fields.
	Permissions             []string
	ExpirationDateTime      *time.Time
	TransactionFromDateTime *time.Time
	TransactionToDateTime   *time.Time

	// Domestic-payment (PIS) initiation fields.
	InstructionIdentification string
	EndToEndIdentification    string
	InstructedAmount          *obie.Amount
	CreditorAccount           *Account
	Reference                 string

	// Shared by payment + funds-confirmation: the account being debited.
	DebtorAccount *Account
}

// IsActive reports whether the consent is currently usable to authorise access
// or a payment: it must be Authorised and not past its expiry.
func (c *Consent) IsActive(now time.Time) bool {
	if c.Status != StatusAuthorised {
		return false
	}
	if c.ExpirationDateTime != nil && now.After(*c.ExpirationDateTime) {
		return false
	}
	return true
}

// Authorise moves an awaiting consent to Authorised. In production this is the
// result of the PSU authenticating at the ASPSP; here it is driven by the
// internal authorise endpoint.
func (c *Consent) Authorise(now time.Time) error {
	if c.Status != StatusAwaitingAuthorisation {
		return fmt.Errorf("cannot authorise consent in status %s", c.Status)
	}
	c.Status = StatusAuthorised
	c.StatusUpdateDateTime = now
	return nil
}

// Reject moves an awaiting consent to Rejected (PSU declined).
func (c *Consent) Reject(now time.Time) error {
	if c.Status != StatusAwaitingAuthorisation {
		return fmt.Errorf("cannot reject consent in status %s", c.Status)
	}
	c.Status = StatusRejected
	c.StatusUpdateDateTime = now
	return nil
}

// Revoke withdraws an authorised consent (PSU or TPP revocation).
func (c *Consent) Revoke(now time.Time) error {
	switch c.Status {
	case StatusAwaitingAuthorisation, StatusAuthorised:
		c.Status = StatusRevoked
		c.StatusUpdateDateTime = now
		return nil
	default:
		return fmt.Errorf("cannot revoke consent in status %s", c.Status)
	}
}

// Consume marks an authorised single-payment consent as used, preventing
// replay. Only meaningful for domestic-payment consents.
func (c *Consent) Consume(now time.Time) error {
	if c.Type != TypeDomesticPayment {
		return fmt.Errorf("only domestic-payment consents can be consumed")
	}
	if c.Status != StatusAuthorised {
		return fmt.Errorf("cannot consume consent in status %s", c.Status)
	}
	c.Status = StatusConsumed
	c.StatusUpdateDateTime = now
	return nil
}
