package httpx

import (
	"errors"
	"net/http"

	"github.com/sreeni/openbank-bian/pkg/obie"
)

// APIError is the transport-level error returned by handlers and the service
// layer. It carries everything needed to render an OBIE OBErrorResponse1: the
// HTTP status, a human-readable message and zero or more granular Errors. The
// service layer returns APIErrors so handlers can stay thin.
type APIError struct {
	Status  int
	Message string
	Details []obie.ErrorDetail
}

func (e *APIError) Error() string { return e.Message }

// fieldError is a convenience for building an OBIE ErrorDetail with the
// Field.Invalid code and an optional JSON path.
func fieldError(path, message string) obie.ErrorDetail {
	d := obie.ErrorDetail{ErrorCode: obie.ErrFieldInvalid, Message: message}
	if path != "" {
		d.Path = path
	}
	return d
}

// Detail builds an ErrorDetail with an explicit OBIE error code.
func Detail(code, message, path string) obie.ErrorDetail {
	return obie.ErrorDetail{ErrorCode: code, Message: message, Path: path}
}

// The following constructors map common situations to their HTTP status. Each
// accepts optional granular details.

func BadRequest(msg string, details ...obie.ErrorDetail) *APIError {
	return &APIError{Status: http.StatusBadRequest, Message: msg, Details: details}
}

func Unauthorized(msg string, details ...obie.ErrorDetail) *APIError {
	return &APIError{Status: http.StatusUnauthorized, Message: msg, Details: details}
}

func Forbidden(msg string, details ...obie.ErrorDetail) *APIError {
	return &APIError{Status: http.StatusForbidden, Message: msg, Details: details}
}

func NotFound(msg string, details ...obie.ErrorDetail) *APIError {
	return &APIError{Status: http.StatusNotFound, Message: msg, Details: details}
}

func Conflict(msg string, details ...obie.ErrorDetail) *APIError {
	return &APIError{Status: http.StatusConflict, Message: msg, Details: details}
}

func Unprocessable(msg string, details ...obie.ErrorDetail) *APIError {
	return &APIError{Status: http.StatusUnprocessableEntity, Message: msg, Details: details}
}

func Internal(msg string) *APIError {
	return &APIError{Status: http.StatusInternalServerError, Message: msg}
}

// RespondError writes err as an OBIE error response. *APIError values are
// rendered with their status and details; any other error is treated as an
// opaque 500 so internal failure detail never leaks to the caller.
func RespondError(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		apiErr = Internal("An internal error occurred")
	}
	body := obie.NewErrorResponse(
		http.StatusText(apiErr.Status),
		apiErr.Message,
		apiErr.Details...,
	)
	WriteJSON(w, apiErr.Status, body)
}
