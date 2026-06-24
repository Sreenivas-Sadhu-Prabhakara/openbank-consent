package consent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sreeni/openbank-bian/pkg/obie"
)

func newTestHandler() http.Handler {
	return NewHandler(newTestService(), "http://consent.test").Routes()
}

// do issues a request to the handler and returns the recorder.
func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestAccountAccessConsentFlow(t *testing.T) {
	h := newTestHandler()

	// Create.
	w := do(t, h, http.MethodPost, "/account-access-consents", `{
		"Data": {"Permissions": ["ReadAccountsBasic","ReadBalances","ReadTransactionsDetail"]},
		"Risk": {}
	}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", w.Code, w.Body)
	}
	var created struct {
		Data  accountAccessRespData `json:"Data"`
		Links obie.Links            `json:"Links"`
	}
	mustDecode(t, w, &created)
	if created.Data.Status != "AwaitingAuthorisation" {
		t.Fatalf("status = %s", created.Data.Status)
	}
	id := created.Data.ConsentID
	if id == "" {
		t.Fatal("missing ConsentId")
	}
	if created.Links.Self != "http://consent.test/account-access-consents/"+id {
		t.Fatalf("Self = %s", created.Links.Self)
	}

	// Get.
	w = do(t, h, http.MethodGet, "/account-access-consents/"+id, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d", w.Code)
	}

	// Authorise (simulated PSU) then confirm via internal view.
	w = do(t, h, http.MethodPost, "/internal/consents/"+id+"/authorise", "")
	if w.Code != http.StatusOK {
		t.Fatalf("authorise status = %d", w.Code)
	}
	w = do(t, h, http.MethodGet, "/internal/consents/"+id, "")
	var view struct {
		Status      string   `json:"Status"`
		Permissions []string `json:"Permissions"`
	}
	mustDecode(t, w, &view)
	if view.Status != "Authorised" {
		t.Fatalf("view status = %s", view.Status)
	}

	// Delete then expect 404 with an OBIE error body.
	w = do(t, h, http.MethodDelete, "/account-access-consents/"+id, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", w.Code)
	}
	w = do(t, h, http.MethodGet, "/account-access-consents/"+id, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("get-after-delete status = %d", w.Code)
	}
	var errBody obie.ErrorResponse
	mustDecode(t, w, &errBody)
	if errBody.Code != "Not Found" || len(errBody.Errors) == 0 {
		t.Fatalf("unexpected error body %+v", errBody)
	}
}

func TestCreateAccountAccessValidationError(t *testing.T) {
	h := newTestHandler()
	w := do(t, h, http.MethodPost, "/account-access-consents", `{"Data":{"Permissions":[]},"Risk":{}}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
	var errBody obie.ErrorResponse
	mustDecode(t, w, &errBody)
	if len(errBody.Errors) == 0 || errBody.Errors[0].Path != "Data.Permissions" {
		t.Fatalf("unexpected error body %+v", errBody)
	}
}

func TestUnknownFieldRejected(t *testing.T) {
	h := newTestHandler()
	w := do(t, h, http.MethodPost, "/account-access-consents",
		`{"Data":{"Permissions":["ReadBalances"]},"Risk":{},"Bogus":true}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body)
	}
}

func TestDomesticPaymentConsentFlow(t *testing.T) {
	h := newTestHandler()
	w := do(t, h, http.MethodPost, "/domestic-payment-consents", `{
		"Data": {"Initiation": {
			"InstructionIdentification": "ID412",
			"EndToEndIdentification": "E2E412",
			"InstructedAmount": {"Amount": "165.88", "Currency": "GBP"},
			"CreditorAccount": {"SchemeName": "UK.OBIE.SortCodeAccountNumber", "Identification": "08080021325698", "Name": "ACME Inc"},
			"RemittanceInformation": {"Reference": "FRESCO-101"}
		}},
		"Risk": {"PaymentContextCode": "EcommerceGoods"}
	}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body)
	}
	var resp struct {
		Data domesticPaymentRespData `json:"Data"`
	}
	mustDecode(t, w, &resp)
	if resp.Data.Initiation.InstructedAmount.String() != "165.88" {
		t.Fatalf("amount = %s", resp.Data.Initiation.InstructedAmount)
	}
	if resp.Data.Status != "AwaitingAuthorisation" {
		t.Fatalf("status = %s", resp.Data.Status)
	}
}

func TestFundsConfirmationConsentFlow(t *testing.T) {
	h := newTestHandler()
	w := do(t, h, http.MethodPost, "/funds-confirmation-consents", `{
		"Data": {"DebtorAccount": {"SchemeName": "UK.OBIE.SortCodeAccountNumber", "Identification": "70000170000001", "Name": "A Smith"}}
	}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body)
	}
}

func mustDecode(t *testing.T, w *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
}
