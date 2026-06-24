package consent

import (
	"encoding/json"
	"time"

	"github.com/sreeni/openbank-bian/pkg/httpx"
	"github.com/sreeni/openbank-bian/pkg/obie"
)

// envelope is the OBIE consent response shape. It mirrors obie.Response but
// adds the Risk object that consent and payment resources carry.
type envelope struct {
	Data  any             `json:"Data"`
	Risk  json.RawMessage `json:"Risk"`
	Links obie.Links      `json:"Links"`
	Meta  obie.Meta       `json:"Meta"`
}

// emptyRisk is returned where we accept but do not persist the Risk block.
var emptyRisk = json.RawMessage(`{}`)

func newEnvelope(self string, data any) envelope {
	return envelope{Data: data, Risk: emptyRisk, Links: obie.Links{Self: self}, Meta: obie.Meta{}}
}

// accountDTO is the OBIE account identifier block on the wire.
type accountDTO struct {
	SchemeName     string `json:"SchemeName"`
	Identification string `json:"Identification"`
	Name           string `json:"Name,omitempty"`
}

func (d accountDTO) toDomain() Account {
	return Account{SchemeName: d.SchemeName, Identification: d.Identification, Name: d.Name}
}

func accountToDTO(a *Account) *accountDTO {
	if a == nil {
		return nil
	}
	return &accountDTO{SchemeName: a.SchemeName, Identification: a.Identification, Name: a.Name}
}

// ---- account-access (AIS) ----

type accountAccessReq struct {
	Data struct {
		Permissions             []string `json:"Permissions"`
		ExpirationDateTime      *string  `json:"ExpirationDateTime"`
		TransactionFromDateTime *string  `json:"TransactionFromDateTime"`
		TransactionToDateTime   *string  `json:"TransactionToDateTime"`
	} `json:"Data"`
	Risk json.RawMessage `json:"Risk"`
}

type accountAccessRespData struct {
	ConsentID               string   `json:"ConsentId"`
	Status                  string   `json:"Status"`
	CreationDateTime        string   `json:"CreationDateTime"`
	StatusUpdateDateTime    string   `json:"StatusUpdateDateTime"`
	Permissions             []string `json:"Permissions"`
	ExpirationDateTime      string   `json:"ExpirationDateTime,omitempty"`
	TransactionFromDateTime string   `json:"TransactionFromDateTime,omitempty"`
	TransactionToDateTime   string   `json:"TransactionToDateTime,omitempty"`
}

func accountAccessData(c *Consent) accountAccessRespData {
	return accountAccessRespData{
		ConsentID:               c.ID,
		Status:                  string(c.Status),
		CreationDateTime:        rfc3339(c.CreationDateTime),
		StatusUpdateDateTime:    rfc3339(c.StatusUpdateDateTime),
		Permissions:             c.Permissions,
		ExpirationDateTime:      rfc3339Ptr(c.ExpirationDateTime),
		TransactionFromDateTime: rfc3339Ptr(c.TransactionFromDateTime),
		TransactionToDateTime:   rfc3339Ptr(c.TransactionToDateTime),
	}
}

// ---- domestic-payment (PIS) ----

type initiationDTO struct {
	InstructionIdentification string      `json:"InstructionIdentification"`
	EndToEndIdentification    string      `json:"EndToEndIdentification"`
	InstructedAmount          obie.Amount `json:"InstructedAmount"`
	CreditorAccount           accountDTO  `json:"CreditorAccount"`
	DebtorAccount             *accountDTO `json:"DebtorAccount,omitempty"`
	RemittanceInformation     *struct {
		Reference    string `json:"Reference,omitempty"`
		Unstructured string `json:"Unstructured,omitempty"`
	} `json:"RemittanceInformation,omitempty"`
}

type domesticPaymentReq struct {
	Data struct {
		Initiation initiationDTO `json:"Initiation"`
	} `json:"Data"`
	Risk json.RawMessage `json:"Risk"`
}

type domesticPaymentRespData struct {
	ConsentID            string        `json:"ConsentId"`
	Status               string        `json:"Status"`
	CreationDateTime     string        `json:"CreationDateTime"`
	StatusUpdateDateTime string        `json:"StatusUpdateDateTime"`
	Initiation           initiationDTO `json:"Initiation"`
}

func domesticPaymentData(c *Consent) domesticPaymentRespData {
	init := initiationDTO{
		InstructionIdentification: c.InstructionIdentification,
		EndToEndIdentification:    c.EndToEndIdentification,
		CreditorAccount:           *accountToDTO(c.CreditorAccount),
		DebtorAccount:             accountToDTO(c.DebtorAccount),
	}
	if c.InstructedAmount != nil {
		init.InstructedAmount = *c.InstructedAmount
	}
	if c.Reference != "" {
		init.RemittanceInformation = &struct {
			Reference    string `json:"Reference,omitempty"`
			Unstructured string `json:"Unstructured,omitempty"`
		}{Reference: c.Reference}
	}
	return domesticPaymentRespData{
		ConsentID:            c.ID,
		Status:               string(c.Status),
		CreationDateTime:     rfc3339(c.CreationDateTime),
		StatusUpdateDateTime: rfc3339(c.StatusUpdateDateTime),
		Initiation:           init,
	}
}

// ---- funds-confirmation (CBPII) ----

type fundsConfirmationReq struct {
	Data struct {
		DebtorAccount      accountDTO `json:"DebtorAccount"`
		ExpirationDateTime *string    `json:"ExpirationDateTime"`
	} `json:"Data"`
}

type fundsConfirmationRespData struct {
	ConsentID            string     `json:"ConsentId"`
	Status               string     `json:"Status"`
	CreationDateTime     string     `json:"CreationDateTime"`
	StatusUpdateDateTime string     `json:"StatusUpdateDateTime"`
	DebtorAccount        accountDTO `json:"DebtorAccount"`
	ExpirationDateTime   string     `json:"ExpirationDateTime,omitempty"`
}

func fundsConfirmationData(c *Consent) fundsConfirmationRespData {
	return fundsConfirmationRespData{
		ConsentID:            c.ID,
		Status:               string(c.Status),
		CreationDateTime:     rfc3339(c.CreationDateTime),
		StatusUpdateDateTime: rfc3339(c.StatusUpdateDateTime),
		DebtorAccount:        *accountToDTO(c.DebtorAccount),
		ExpirationDateTime:   rfc3339Ptr(c.ExpirationDateTime),
	}
}

// ---- shared helpers ----

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func rfc3339Ptr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// parseTimePtr parses an optional RFC3339 timestamp, returning a 400 APIError
// with the given JSON path on malformed input.
func parseTimePtr(s *string, path string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, httpx.BadRequest("Invalid timestamp",
			httpx.Detail(obie.ErrFieldInvalid, "expected RFC3339 datetime", path))
	}
	return &t, nil
}
