package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sreeni/openbank-bian/pkg/obie"
)

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"Known":1,"Bogus":2}`))
	var dst struct {
		Known int `json:"Known"`
	}
	err := DecodeJSON(w, r, &dst)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != http.StatusBadRequest {
		t.Fatalf("want 400 APIError, got %#v", err)
	}
}

func TestDecodeJSONRejectsTrailingData(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"Known":1}{"Known":2}`))
	var dst struct {
		Known int `json:"Known"`
	}
	if err := DecodeJSON(w, r, &dst); err == nil {
		t.Fatal("expected error for trailing data")
	}
}

func TestRespondErrorRendersOBIEEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	RespondError(w, NotFound("Account not found",
		Detail(obie.ErrResourceNotFound, "no such account", "")))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	var body obie.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "Not Found" {
		t.Fatalf("Code = %q, want %q", body.Code, "Not Found")
	}
	if body.ID == "" {
		t.Fatal("expected a correlation Id")
	}
	if len(body.Errors) != 1 || body.Errors[0].ErrorCode != obie.ErrResourceNotFound {
		t.Fatalf("unexpected Errors: %+v", body.Errors)
	}
}

func TestRespondErrorHidesInternalDetail(t *testing.T) {
	w := httptest.NewRecorder()
	// A plain error must not leak its message; it becomes a generic 500.
	RespondError(w, errSecret)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("internal detail leaked: %s", w.Body.String())
	}
}

var errSecret = &plainError{"secret connection string failed"}

type plainError struct{ s string }

func (e *plainError) Error() string { return e.s }

func TestFAPIInteractionIDEchoAndGenerate(t *testing.T) {
	handler := FAPIInteractionID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if InteractionID(r.Context()) == "" {
			t.Error("interaction id missing from context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Echo a supplied id.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(FAPIHeader, "abc-123")
	handler.ServeHTTP(w, r)
	if got := w.Header().Get(FAPIHeader); got != "abc-123" {
		t.Fatalf("echoed id = %q, want abc-123", got)
	}

	// Generate one when absent.
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)
	if w.Header().Get(FAPIHeader) == "" {
		t.Fatal("expected generated interaction id")
	}
}
