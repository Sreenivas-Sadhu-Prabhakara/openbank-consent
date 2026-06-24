package consent

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/sreeni/openbank-bian/pkg/consentcli"
	"github.com/sreeni/openbank-bian/pkg/httpx"
	"github.com/sreeni/openbank-bian/pkg/obie"
)

// allowedPermissions is the OBIE Permissions enum subset this estate supports.
var allowedPermissions = map[string]bool{
	"ReadAccountsBasic":       true,
	"ReadAccountsDetail":      true,
	"ReadBalances":            true,
	"ReadTransactionsBasic":   true,
	"ReadTransactionsCredits": true,
	"ReadTransactionsDebits":  true,
	"ReadTransactionsDetail":  true,
	"ReadParty":               true,
}

// Service holds the consent business logic. The clock and id generator are
// injected so tests are deterministic.
type Service struct {
	repo  Repository
	now   func() time.Time
	newID func() string
}

// NewService wires a Service to its repository using a real clock and UUID ids.
func NewService(repo Repository) *Service {
	return &Service{
		repo:  repo,
		now:   time.Now,
		newID: uuid.NewString,
	}
}

// Inputs for creation, kept independent of the OBIE wire shapes.

type AccountAccessInput struct {
	Permissions             []string
	ExpirationDateTime      *time.Time
	TransactionFromDateTime *time.Time
	TransactionToDateTime   *time.Time
}

type DomesticPaymentInput struct {
	InstructionIdentification string
	EndToEndIdentification    string
	InstructedAmount          obie.Amount
	CreditorAccount           Account
	DebtorAccount             *Account
	Reference                 string
}

type FundsConfirmationInput struct {
	DebtorAccount      Account
	ExpirationDateTime *time.Time
}

// CreateAccountAccess validates and stores a new AIS consent in
// AwaitingAuthorisation.
func (s *Service) CreateAccountAccess(ctx context.Context, in AccountAccessInput) (*Consent, error) {
	if len(in.Permissions) == 0 {
		return nil, httpx.BadRequest("At least one permission is required",
			httpx.Detail(obie.ErrFieldMissing, "Permissions must not be empty", "Data.Permissions"))
	}
	for _, p := range in.Permissions {
		if !allowedPermissions[p] {
			return nil, httpx.BadRequest("Unsupported permission",
				httpx.Detail(obie.ErrFieldInvalid, "unsupported permission: "+p, "Data.Permissions"))
		}
	}
	if err := s.validateExpiry(in.ExpirationDateTime); err != nil {
		return nil, err
	}
	if in.TransactionFromDateTime != nil && in.TransactionToDateTime != nil &&
		in.TransactionToDateTime.Before(*in.TransactionFromDateTime) {
		return nil, httpx.BadRequest("TransactionToDateTime must not precede TransactionFromDateTime",
			httpx.Detail(obie.ErrFieldInvalid, "invalid transaction window", "Data.TransactionToDateTime"))
	}

	now := s.now()
	c := &Consent{
		ID:                      s.newID(),
		Type:                    TypeAccountAccess,
		Status:                  StatusAwaitingAuthorisation,
		CreationDateTime:        now,
		StatusUpdateDateTime:    now,
		Permissions:             in.Permissions,
		ExpirationDateTime:      in.ExpirationDateTime,
		TransactionFromDateTime: in.TransactionFromDateTime,
		TransactionToDateTime:   in.TransactionToDateTime,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, httpx.Internal("could not persist consent")
	}
	return c, nil
}

// CreateDomesticPayment validates and stores a new PIS consent.
func (s *Service) CreateDomesticPayment(ctx context.Context, in DomesticPaymentInput) (*Consent, error) {
	if in.InstructionIdentification == "" || len(in.InstructionIdentification) > 35 {
		return nil, httpx.BadRequest("InstructionIdentification is required (max 35 chars)",
			httpx.Detail(obie.ErrFieldInvalid, "invalid InstructionIdentification", "Data.Initiation.InstructionIdentification"))
	}
	if in.EndToEndIdentification == "" {
		return nil, httpx.BadRequest("EndToEndIdentification is required",
			httpx.Detail(obie.ErrFieldMissing, "missing EndToEndIdentification", "Data.Initiation.EndToEndIdentification"))
	}
	if err := in.InstructedAmount.Validate(); err != nil {
		return nil, httpx.BadRequest("Invalid InstructedAmount",
			httpx.Detail(obie.ErrFieldInvalid, err.Error(), "Data.Initiation.InstructedAmount"))
	}
	if err := validateAccount(in.CreditorAccount, "Data.Initiation.CreditorAccount"); err != nil {
		return nil, err
	}
	if in.DebtorAccount != nil {
		if err := validateAccount(*in.DebtorAccount, "Data.Initiation.DebtorAccount"); err != nil {
			return nil, err
		}
	}

	now := s.now()
	amt := in.InstructedAmount
	c := &Consent{
		ID:                        s.newID(),
		Type:                      TypeDomesticPayment,
		Status:                    StatusAwaitingAuthorisation,
		CreationDateTime:          now,
		StatusUpdateDateTime:      now,
		InstructionIdentification: in.InstructionIdentification,
		EndToEndIdentification:    in.EndToEndIdentification,
		InstructedAmount:          &amt,
		CreditorAccount:           &in.CreditorAccount,
		DebtorAccount:             in.DebtorAccount,
		Reference:                 in.Reference,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, httpx.Internal("could not persist consent")
	}
	return c, nil
}

// CreateFundsConfirmation validates and stores a new CBPII consent.
func (s *Service) CreateFundsConfirmation(ctx context.Context, in FundsConfirmationInput) (*Consent, error) {
	if err := validateAccount(in.DebtorAccount, "Data.DebtorAccount"); err != nil {
		return nil, err
	}
	if err := s.validateExpiry(in.ExpirationDateTime); err != nil {
		return nil, err
	}

	now := s.now()
	acc := in.DebtorAccount
	c := &Consent{
		ID:                   s.newID(),
		Type:                 TypeFundsConfirmation,
		Status:               StatusAwaitingAuthorisation,
		CreationDateTime:     now,
		StatusUpdateDateTime: now,
		DebtorAccount:        &acc,
		ExpirationDateTime:   in.ExpirationDateTime,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, httpx.Internal("could not persist consent")
	}
	return c, nil
}

// Get returns a consent, asserting it is of the expected type so e.g. a
// payment-consent id cannot be read through the account-access endpoint.
func (s *Service) Get(ctx context.Context, id string, expect Type) (*Consent, error) {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, s.mapNotFound(err)
	}
	if c.Type != expect {
		return nil, httpx.NotFound("Consent not found",
			httpx.Detail(obie.ErrResourceNotFound, "no such consent", ""))
	}
	return c, nil
}

// Authorise simulates PSU authentication, moving the consent to Authorised.
func (s *Service) Authorise(ctx context.Context, id string) (*Consent, error) {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, s.mapNotFound(err)
	}
	if err := c.Authorise(s.now()); err != nil {
		return nil, httpx.Conflict(err.Error(),
			httpx.Detail(obie.ErrResourceInvalid, err.Error(), ""))
	}
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, httpx.Internal("could not update consent")
	}
	return c, nil
}

// Consume marks a domestic-payment consent as used (called by the payments
// service once it accepts the payment).
func (s *Service) Consume(ctx context.Context, id string) (*Consent, error) {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, s.mapNotFound(err)
	}
	if err := c.Consume(s.now()); err != nil {
		return nil, httpx.Conflict(err.Error(),
			httpx.Detail(obie.ErrResourceInvalid, err.Error(), ""))
	}
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, httpx.Internal("could not update consent")
	}
	return c, nil
}

// Delete removes a consent of the expected type (OBIE DELETE semantics).
func (s *Service) Delete(ctx context.Context, id string, expect Type) error {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return s.mapNotFound(err)
	}
	if c.Type != expect {
		return httpx.NotFound("Consent not found",
			httpx.Detail(obie.ErrResourceNotFound, "no such consent", ""))
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return httpx.Internal("could not delete consent")
	}
	return nil
}

// View returns the internal projection used by other services to authorise
// requests.
func (s *Service) View(ctx context.Context, id string) (*consentcli.View, error) {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, s.mapNotFound(err)
	}
	v := &consentcli.View{
		ConsentID:   c.ID,
		Type:        string(c.Type),
		Status:      string(c.Status),
		Permissions: c.Permissions,
	}
	if c.DebtorAccount != nil {
		v.DebtorAccountID = c.DebtorAccount.Identification
	}
	if c.InstructedAmount != nil {
		amt := *c.InstructedAmount
		v.InstructedAmount = &amt
	}
	if c.CreditorAccount != nil {
		v.CreditorName = c.CreditorAccount.Name
	}
	if c.ExpirationDateTime != nil {
		v.ExpiresAt = c.ExpirationDateTime.UTC().Format(time.RFC3339)
	}
	return v, nil
}

func (s *Service) validateExpiry(exp *time.Time) error {
	if exp != nil && !exp.After(s.now()) {
		return httpx.BadRequest("ExpirationDateTime must be in the future",
			httpx.Detail(obie.ErrFieldInvalid, "expiry already elapsed", "Data.ExpirationDateTime"))
	}
	return nil
}

func (s *Service) mapNotFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound("Consent not found",
			httpx.Detail(obie.ErrResourceNotFound, "no such consent", ""))
	}
	return httpx.Internal("could not load consent")
}

func validateAccount(a Account, path string) error {
	if a.SchemeName == "" {
		return httpx.BadRequest("Account SchemeName is required",
			httpx.Detail(obie.ErrFieldMissing, "missing SchemeName", path+".SchemeName"))
	}
	if a.Identification == "" {
		return httpx.BadRequest("Account Identification is required",
			httpx.Detail(obie.ErrFieldMissing, "missing Identification", path+".Identification"))
	}
	return nil
}
