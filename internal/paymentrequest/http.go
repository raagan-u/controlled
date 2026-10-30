package paymentrequest

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

type Creator interface {
	Create(context.Context, string, identity.UserID, identity.UserID, int64, string, string) (Request, error)
}

type Handler struct{ creator Creator }

func NewHandler(creator Creator) *Handler { return &Handler{creator: creator} }

func (h *Handler) Create(response http.ResponseWriter, request *http.Request) {
	current, ok := identity.CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	var input struct {
		RecipientID string `json:"recipientId"`
		AmountMinor int64  `json:"amountMinor"`
		Currency    string `json:"currency"`
		Note        string `json:"note"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain a recipient and amount.")
		return
	}
	created, err := h.creator.Create(request.Context(), request.Header.Get("Idempotency-Key"), current.ID(), identity.UserID(input.RecipientID), input.AmountMinor, input.Currency, input.Note)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidRequest):
			httpapi.WriteError(response, http.StatusUnprocessableEntity, "invalid_request", "Choose an existing recipient and a positive USD amount.")
		case errors.Is(err, ErrIdempotencyConflict):
			httpapi.WriteError(response, http.StatusConflict, "idempotency_conflict", "That request key was already used for different details.")
		default:
			httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "Controlled could not create the payment request.")
		}
		return
	}
	status := http.StatusCreated
	if created.Duplicate {
		status = http.StatusOK
	}
	httpapi.WriteJSON(response, status, struct {
		PaymentRequest responseItem `json:"paymentRequest"`
	}{PaymentRequest: responseItem{ID: created.ID, RecipientID: string(created.RecipientID), AmountMinor: created.AmountMinor, Currency: created.Currency, Note: created.Note, Status: created.Status, CreatedAt: created.CreatedAt, Duplicate: created.Duplicate}})
}

type responseItem struct {
	ID          string    `json:"id"`
	RecipientID string    `json:"recipientId"`
	AmountMinor int64     `json:"amountMinor"`
	Currency    string    `json:"currency"`
	Note        string    `json:"note"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	Duplicate   bool      `json:"duplicate"`
}
