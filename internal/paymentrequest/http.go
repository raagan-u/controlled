package paymentrequest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/notification"
	"github.com/raagan-u/controlled/internal/platform/httpapi"
	"github.com/raagan-u/controlled/internal/walletops"
)

type Commands interface {
	Create(context.Context, string, identity.UserID, identity.UserID, int64, string, string) (Request, error)
	Requests(context.Context, identity.UserID) ([]Request, error)
	Request(context.Context, identity.UserID, string) (Request, error)
	Pay(context.Context, string, identity.UserID, string) (PayResult, error)
	Decline(context.Context, identity.UserID, string) (Request, error)
	Cancel(context.Context, identity.UserID, string) (Request, error)
}
type Handler struct {
	commands      Commands
	notifications notification.Notifier
}

func NewHandler(commands Commands) *Handler { return &Handler{commands: commands} }

func (h *Handler) EnableNotifications(notifier notification.Notifier) *Handler {
	if h != nil {
		h.notifications = notifier
	}
	return h
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.CurrentUser(r.Context())
	if !ok {
		writeAuth(w)
		return
	}
	var input struct {
		RecipientID string `json:"recipientId"`
		AmountMinor int64  `json:"amountMinor"`
		Currency    string `json:"currency"`
		Note        string `json:"note"`
	}
	if httpapi.ReadJSON(w, r, &input, httpapi.DefaultMaxBodyBytes) != nil {
		httpapi.WriteError(w, 400, "invalid_request", "Request body must contain a recipient and amount.")
		return
	}
	item, err := h.commands.Create(r.Context(), r.Header.Get("Idempotency-Key"), user.ID(), identity.UserID(input.RecipientID), input.AmountMinor, input.Currency, input.Note)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if item.Duplicate {
		status = http.StatusOK
	}
	if !item.Duplicate {
		h.notify(item.RecipientID, notification.Message{
			Title: "Payment request", Body: fmt.Sprintf("%s requested $%.2f", user.DisplayName(), float64(item.AmountMinor)/100),
			Kind: notification.KindPaymentRequest, EntityID: item.ID,
		})
	}
	writeItem(w, status, item, user.ID())
}
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.CurrentUser(r.Context())
	if !ok {
		writeAuth(w)
		return
	}
	items, err := h.commands.Requests(r.Context(), user.ID())
	if err != nil {
		writeError(w, err)
		return
	}
	output := make([]responseItem, len(items))
	for i, item := range items {
		output[i] = newResponse(item, user.ID())
	}
	httpapi.WriteJSON(w, 200, struct {
		PaymentRequests []responseItem `json:"paymentRequests"`
	}{output})
}
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.CurrentUser(r.Context())
	if !ok {
		writeAuth(w)
		return
	}
	item, err := h.commands.Request(r.Context(), user.ID(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeItem(w, 200, item, user.ID())
}
func (h *Handler) Pay(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.CurrentUser(r.Context())
	if !ok {
		writeAuth(w)
		return
	}
	result, err := h.commands.Pay(r.Context(), r.Header.Get("Idempotency-Key"), user.ID(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	payload := struct {
		PaymentRequest        responseItem    `json:"paymentRequest"`
		AccountabilityRequest *reviewResponse `json:"accountabilityRequest,omitempty"`
	}{PaymentRequest: newResponse(result.Request, user.ID())}
	if result.Review != nil {
		payload.AccountabilityRequest = &reviewResponse{ID: result.Review.ID, Status: result.Review.Status, ExpiresAt: result.Review.ExpiresAt}
		if !result.Review.Duplicate {
			h.notify(result.Review.AssignedPartnerID, notification.Message{
				Title: "Approval needed", Body: fmt.Sprintf("%s needs approval to pay $%.2f", user.DisplayName(), float64(result.Request.AmountMinor)/100),
				Kind: notification.KindAccountabilityRequest, EntityID: result.Review.ID,
			})
		}
	}
	httpapi.WriteJSON(w, 200, payload)
}
func (h *Handler) Decline(w http.ResponseWriter, r *http.Request) { h.resolve(w, r, false) }
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request)  { h.resolve(w, r, true) }
func (h *Handler) resolve(w http.ResponseWriter, r *http.Request, cancel bool) {
	user, ok := identity.CurrentUser(r.Context())
	if !ok {
		writeAuth(w)
		return
	}
	var item Request
	var err error
	if cancel {
		item, err = h.commands.Cancel(r.Context(), user.ID(), r.PathValue("id"))
	} else {
		item, err = h.commands.Decline(r.Context(), user.ID(), r.PathValue("id"))
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if cancel {
		h.notify(item.RecipientID, notification.Message{Title: "Request cancelled", Body: fmt.Sprintf("%s cancelled a $%.2f request", user.DisplayName(), float64(item.AmountMinor)/100), Kind: notification.KindPaymentRequest, EntityID: item.ID})
	} else {
		h.notify(item.RequesterID, notification.Message{Title: "Request declined", Body: fmt.Sprintf("%s declined your $%.2f request", user.DisplayName(), float64(item.AmountMinor)/100), Kind: notification.KindPaymentRequest, EntityID: item.ID})
	}
	writeItem(w, 200, item, user.ID())
}

func (h *Handler) notify(recipient identity.UserID, message notification.Message) {
	if h != nil && h.notifications != nil {
		h.notifications.Notify(recipient, message)
	}
}

type partyResponse struct {
	ID          string `json:"id"`
	CtrldID     string `json:"ctrldId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}
type responseItem struct {
	ID                      string        `json:"id"`
	Requester               partyResponse `json:"requester"`
	Recipient               partyResponse `json:"recipient"`
	AmountMinor             int64         `json:"amountMinor"`
	Currency                string        `json:"currency"`
	Note                    string        `json:"note"`
	Status                  string        `json:"status"`
	CreatedAt               time.Time     `json:"createdAt"`
	UpdatedAt               time.Time     `json:"updatedAt"`
	ExpiresAt               time.Time     `json:"expiresAt"`
	ResolvedAt              *time.Time    `json:"resolvedAt,omitempty"`
	TransferID              string        `json:"transferId,omitempty"`
	AccountabilityRequestID string        `json:"accountabilityRequestId,omitempty"`
	Duplicate               bool          `json:"duplicate"`
}
type reviewResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func newParty(p Party) partyResponse { return partyResponse{p.ID, p.CtrldID, p.Email, p.DisplayName} }
func newResponse(item Request, viewer identity.UserID) responseItem {
	result := responseItem{ID: item.ID, Requester: newParty(item.Requester), Recipient: newParty(item.Recipient), AmountMinor: item.AmountMinor, Currency: item.Currency, Note: item.Note, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, ExpiresAt: item.ExpiresAt, TransferID: item.TransferID, AccountabilityRequestID: item.AccountabilityRequestID, Duplicate: item.Duplicate}
	// A payer's pact and approval flow are private. The requester only needs the payment status.
	if viewer == item.RequesterID {
		result.AccountabilityRequestID = ""
	}
	if !item.ResolvedAt.IsZero() {
		result.ResolvedAt = &item.ResolvedAt
	}
	return result
}
func writeItem(w http.ResponseWriter, status int, item Request, viewer identity.UserID) {
	httpapi.WriteJSON(w, status, struct {
		PaymentRequest responseItem `json:"paymentRequest"`
	}{newResponse(item, viewer)})
}
func writeAuth(w http.ResponseWriter) {
	httpapi.WriteError(w, 401, "authentication_required", "A valid Controlled session is required.")
}
func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		httpapi.WriteError(w, 422, "invalid_request", "The payment request details are invalid.")
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, 404, "payment_request_not_found", "The payment request was not found.")
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, 403, "payment_request_forbidden", "You cannot perform that action on this request.")
	case errors.Is(err, ErrInvalidState):
		httpapi.WriteError(w, 409, "invalid_request_state", "The payment request can no longer perform that action.")
	case errors.Is(err, ErrIdempotencyConflict), errors.Is(err, walletops.ErrIdempotencyConflict):
		httpapi.WriteError(w, 409, "idempotency_conflict", "This action key was already used with different details.")
	case errors.Is(err, ledger.ErrInsufficientBalance):
		httpapi.WriteError(w, 409, "insufficient_balance", "You do not have enough available balance. The request is still pending.")
	default:
		httpapi.WriteError(w, 500, "internal_error", "Controlled could not complete the payment request.")
	}
}
