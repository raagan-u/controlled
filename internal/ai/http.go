package ai

import (
	"errors"
	"net/http"

	"github.com/raagan-u/controlled/internal/accountability"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/pact"
	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) SuggestPact(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	var input struct {
		Goal   string `json:"goal"`
		Intent string `json:"intent"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain a pact goal and reason.")
		return
	}
	guidance, err := h.service.SuggestPact(request.Context(), user.ID(), input.Goal, input.Intent)
	if err != nil {
		writeError(response, err)
		return
	}
	writeGuidance(response, guidance)
}

func (h *Handler) AssessTransaction(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	var input struct {
		AmountMinor           int64  `json:"amountMinor"`
		Remark                string `json:"remark"`
		IsPurchase            bool   `json:"isPurchase"`
		TimezoneOffsetMinutes int    `json:"timezoneOffsetMinutes"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain an amount and transaction remark.")
		return
	}
	assessment, err := h.service.AssessTransaction(request.Context(), user.ID(), input.AmountMinor, input.Remark, input.IsPurchase, input.TimezoneOffsetMinutes)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			httpapi.WriteError(response, http.StatusUnprocessableEntity, "invalid_transaction_remark", "Add a meaningful remark so Controlled can guide this payment.")
			return
		}
		writeError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, struct {
		Assessment TransactionAssessment `json:"assessment"`
	}{Assessment: assessment})
}

func (h *Handler) PactInsight(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	guidance, err := h.service.PactInsight(request.Context(), user.ID(), pact.PactID(request.PathValue("id")))
	if err != nil {
		writeError(response, err)
		return
	}
	writeGuidance(response, guidance)
}

func (h *Handler) AccountabilityBrief(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	guidance, err := h.service.AccountabilityBrief(request.Context(), user.ID(), request.PathValue("id"))
	if err != nil {
		writeError(response, err)
		return
	}
	writeGuidance(response, guidance)
}

func writeGuidance(response http.ResponseWriter, guidance Guidance) {
	httpapi.WriteJSON(response, http.StatusOK, struct {
		Guidance Guidance `json:"guidance"`
	}{Guidance: guidance})
}

func writeError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "invalid_guidance_input", "The guidance request is incomplete or invalid.")
	case errors.Is(err, pact.ErrPactNotFound):
		httpapi.WriteError(response, http.StatusNotFound, "pact_not_found", "The pact was not found.")
	case errors.Is(err, accountability.ErrRequestNotFound):
		httpapi.WriteError(response, http.StatusNotFound, "accountability_request_not_found", "The accountability request was not found.")
	default:
		httpapi.WriteError(response, http.StatusInternalServerError, "guidance_unavailable", "Controlled guidance is temporarily unavailable.")
	}
}
