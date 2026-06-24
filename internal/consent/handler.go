package consent

import (
	"net/http"

	"github.com/sreeni/openbank-bian/pkg/httpx"
)

// Handler exposes the consent service over HTTP using OBIE request/response
// shapes. baseURL is used to build absolute Self links.
type Handler struct {
	svc     *Service
	baseURL string
}

// NewHandler constructs the HTTP handler.
func NewHandler(svc *Service, baseURL string) *Handler {
	return &Handler{svc: svc, baseURL: baseURL}
}

// Routes registers every consent route on a ServeMux and returns it.
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	// Account-access consents (AIS).
	mux.HandleFunc("POST /account-access-consents", h.createAccountAccess)
	mux.HandleFunc("GET /account-access-consents/{consentId}", h.getAccountAccess)
	mux.HandleFunc("DELETE /account-access-consents/{consentId}", h.deleteAccountAccess)

	// Domestic-payment consents (PIS).
	mux.HandleFunc("POST /domestic-payment-consents", h.createDomesticPayment)
	mux.HandleFunc("GET /domestic-payment-consents/{consentId}", h.getDomesticPayment)

	// Funds-confirmation consents (CBPII).
	mux.HandleFunc("POST /funds-confirmation-consents", h.createFundsConfirmation)
	mux.HandleFunc("GET /funds-confirmation-consents/{consentId}", h.getFundsConfirmation)
	mux.HandleFunc("DELETE /funds-confirmation-consents/{consentId}", h.deleteFundsConfirmation)

	// Internal API used by other services and to drive the demo PSU flow.
	mux.HandleFunc("GET /internal/consents/{consentId}", h.internalView)
	mux.HandleFunc("POST /internal/consents/{consentId}/authorise", h.internalAuthorise)
	mux.HandleFunc("POST /internal/consents/{consentId}/consume", h.internalConsume)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func (h *Handler) self(r *http.Request) string { return h.baseURL + r.URL.Path }

// ---- account-access ----

func (h *Handler) createAccountAccess(w http.ResponseWriter, r *http.Request) {
	var req accountAccessReq
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.RespondError(w, err)
		return
	}
	exp, err := parseTimePtr(req.Data.ExpirationDateTime, "Data.ExpirationDateTime")
	if err != nil {
		httpx.RespondError(w, err)
		return
	}
	from, err := parseTimePtr(req.Data.TransactionFromDateTime, "Data.TransactionFromDateTime")
	if err != nil {
		httpx.RespondError(w, err)
		return
	}
	to, err := parseTimePtr(req.Data.TransactionToDateTime, "Data.TransactionToDateTime")
	if err != nil {
		httpx.RespondError(w, err)
		return
	}

	c, err := h.svc.CreateAccountAccess(r.Context(), AccountAccessInput{
		Permissions:             req.Data.Permissions,
		ExpirationDateTime:      exp,
		TransactionFromDateTime: from,
		TransactionToDateTime:   to,
	})
	if err != nil {
		httpx.RespondError(w, err)
		return
	}
	self := h.baseURL + "/account-access-consents/" + c.ID
	httpx.WriteJSON(w, http.StatusCreated, newEnvelope(self, accountAccessData(c)))
}

func (h *Handler) getAccountAccess(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Get(r.Context(), r.PathValue("consentId"), TypeAccountAccess)
	if err != nil {
		httpx.RespondError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, newEnvelope(h.self(r), accountAccessData(c)))
}

func (h *Handler) deleteAccountAccess(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("consentId"), TypeAccountAccess); err != nil {
		httpx.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- domestic-payment ----

func (h *Handler) createDomesticPayment(w http.ResponseWriter, r *http.Request) {
	var req domesticPaymentReq
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.RespondError(w, err)
		return
	}
	init := req.Data.Initiation
	in := DomesticPaymentInput{
		InstructionIdentification: init.InstructionIdentification,
		EndToEndIdentification:    init.EndToEndIdentification,
		InstructedAmount:          init.InstructedAmount,
		CreditorAccount:           init.CreditorAccount.toDomain(),
	}
	if init.DebtorAccount != nil {
		da := init.DebtorAccount.toDomain()
		in.DebtorAccount = &da
	}
	if init.RemittanceInformation != nil {
		in.Reference = init.RemittanceInformation.Reference
	}

	c, err := h.svc.CreateDomesticPayment(r.Context(), in)
	if err != nil {
		httpx.RespondError(w, err)
		return
	}
	self := h.baseURL + "/domestic-payment-consents/" + c.ID
	httpx.WriteJSON(w, http.StatusCreated, newEnvelope(self, domesticPaymentData(c)))
}

func (h *Handler) getDomesticPayment(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Get(r.Context(), r.PathValue("consentId"), TypeDomesticPayment)
	if err != nil {
		httpx.RespondError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, newEnvelope(h.self(r), domesticPaymentData(c)))
}

// ---- funds-confirmation ----

func (h *Handler) createFundsConfirmation(w http.ResponseWriter, r *http.Request) {
	var req fundsConfirmationReq
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.RespondError(w, err)
		return
	}
	exp, err := parseTimePtr(req.Data.ExpirationDateTime, "Data.ExpirationDateTime")
	if err != nil {
		httpx.RespondError(w, err)
		return
	}
	c, err := h.svc.CreateFundsConfirmation(r.Context(), FundsConfirmationInput{
		DebtorAccount:      req.Data.DebtorAccount.toDomain(),
		ExpirationDateTime: exp,
	})
	if err != nil {
		httpx.RespondError(w, err)
		return
	}
	self := h.baseURL + "/funds-confirmation-consents/" + c.ID
	httpx.WriteJSON(w, http.StatusCreated, newEnvelope(self, fundsConfirmationData(c)))
}

func (h *Handler) getFundsConfirmation(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Get(r.Context(), r.PathValue("consentId"), TypeFundsConfirmation)
	if err != nil {
		httpx.RespondError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, newEnvelope(h.self(r), fundsConfirmationData(c)))
}

func (h *Handler) deleteFundsConfirmation(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("consentId"), TypeFundsConfirmation); err != nil {
		httpx.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- internal ----

func (h *Handler) internalView(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.View(r.Context(), r.PathValue("consentId"))
	if err != nil {
		httpx.RespondError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (h *Handler) internalAuthorise(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Authorise(r.Context(), r.PathValue("consentId"))
	if err != nil {
		httpx.RespondError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"ConsentId": c.ID,
		"Status":    string(c.Status),
	})
}

func (h *Handler) internalConsume(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Consume(r.Context(), r.PathValue("consentId"))
	if err != nil {
		httpx.RespondError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"ConsentId": c.ID,
		"Status":    string(c.Status),
	})
}
