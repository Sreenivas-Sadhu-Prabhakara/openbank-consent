package obie

import (
	"github.com/google/uuid"
)

// OBIE error codes (the UK.OBIE.* namespace) used across the services. These
// are the standard codes an ASPSP returns in the Errors array of an
// OBErrorResponse1.
const (
	ErrFieldInvalid            = "UK.OBIE.Field.Invalid"
	ErrFieldMissing            = "UK.OBIE.Field.Missing"
	ErrFieldExpected           = "UK.OBIE.Field.Expected"
	ErrFieldUnexpected         = "UK.OBIE.Field.Unexpected"
	ErrHeaderInvalid           = "UK.OBIE.Header.Invalid"
	ErrHeaderMissing           = "UK.OBIE.Header.Missing"
	ErrResourceNotFound        = "UK.OBIE.Resource.NotFound"
	ErrResourceInvalid         = "UK.OBIE.Resource.InvalidConsentStatus"
	ErrResourceConsentMismatch = "UK.OBIE.Resource.ConsentMismatch"
	ErrReqObjectInvalid        = "UK.OBIE.Reauthenticate"
	ErrRulesDuplicate          = "UK.OBIE.Rules.DuplicateReference"
	ErrUnexpected              = "UK.OBIE.UnexpectedError"
	ErrInsufficientFunds       = "UK.OBIE.Rules.FailsControl"
)

// ErrorDetail is a single entry in an OBErrorResponse1.Errors array. Path is
// optional and, when present, points at the offending field in JSON dotted
// notation (e.g. "Data.Initiation.InstructedAmount.Amount").
type ErrorDetail struct {
	ErrorCode string `json:"ErrorCode"`
	Message   string `json:"Message"`
	Path      string `json:"Path,omitempty"`
	URL       string `json:"Url,omitempty"`
}

// ErrorResponse is the OBIE OBErrorResponse1 body returned for any 4xx/5xx.
// Code carries the HTTP status reason phrase, Id is a unique correlation id
// for the failure, and Errors holds one or more granular problems.
type ErrorResponse struct {
	Code    string        `json:"Code"`
	ID      string        `json:"Id"`
	Message string        `json:"Message"`
	Errors  []ErrorDetail `json:"Errors"`
}

// NewErrorResponse builds an OBErrorResponse1 with a freshly generated
// correlation Id. code is the HTTP reason phrase (e.g. "400 Bad Request") and
// message is the human-readable summary.
func NewErrorResponse(code, message string, details ...ErrorDetail) ErrorResponse {
	if details == nil {
		details = []ErrorDetail{}
	}
	return ErrorResponse{
		Code:    code,
		ID:      uuid.NewString(),
		Message: message,
		Errors:  details,
	}
}
