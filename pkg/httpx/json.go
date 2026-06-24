// Package httpx contains the small set of HTTP utilities shared by every
// service: JSON encode/decode helpers, OBIE-aware error responses and the
// common middleware chain. It is intentionally built on net/http only so each
// service stays free to choose its own router.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// maxBodyBytes caps request bodies to protect services from oversized payloads.
const maxBodyBytes = 1 << 20 // 1 MiB

// DecodeJSON reads and decodes a JSON request body into dst. It enforces a body
// size limit and rejects unknown fields, which matches OBIE's strict request
// validation. A decode failure is returned as a 400 APIError so handlers can
// pass it straight to RespondError.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return BadRequest("Request body too large", fieldError("", "request body exceeds 1MiB"))
		}
		return BadRequest("Malformed request body", fieldError("", err.Error()))
	}
	// Reject trailing data after the first JSON value.
	if dec.More() {
		return BadRequest("Request body must contain a single JSON object", fieldError("", "unexpected trailing data"))
	}
	return nil
}

// WriteJSON serialises v as JSON with the given status code and the OBIE
// content type.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// drainAndClose is used by clients to allow connection reuse.
func drainAndClose(rc io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(rc, maxBodyBytes))
	_ = rc.Close()
}
