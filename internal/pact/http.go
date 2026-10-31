package pact

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

type Commander interface {
	CreateDraft(context.Context, string, UserID, Terms) (*Pact, bool, error)
	UpdateDraft(context.Context, UserID, PactID, Terms) (*Pact, error)
	Confirm(context.Context, UserID, PactID) (*Pact, error)
	Pact(context.Context, UserID, PactID) (*Pact, error)
	Pacts(context.Context, UserID) ([]*Pact, error)
	ReserveStake(context.Context, string, UserID, PactID) (*Pact, bool, error)
	Cancel(context.Context, UserID, PactID) (*Pact, bool, error)
}

func (h *Handler) ReserveStake(response http.ResponseWriter, request *http.Request) {
	user, ok := pactUser(response, request)
	if !ok {
		return
	}
	value, duplicate, err := h.commands.ReserveStake(
		request.Context(), request.Header.Get("Idempotency-Key"),
		UserID(user.ID()), PactID(request.PathValue("id")),
	)
	if err != nil {
		writePactError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, pactEnvelope(value, duplicate))
}

func (h *Handler) Cancel(response http.ResponseWriter, request *http.Request) {
	user, ok := pactUser(response, request)
	if !ok {
		return
	}
	value, duplicate, err := h.commands.Cancel(
		request.Context(), UserID(user.ID()), PactID(request.PathValue("id")),
	)
	if err != nil {
		writePactError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, pactEnvelope(value, duplicate))
}

type Handler struct {
	commands Commander
}

func NewHandler(commands Commander) *Handler { return &Handler{commands: commands} }

func (h *Handler) CreateDraft(response http.ResponseWriter, request *http.Request) {
	user, ok := pactUser(response, request)
	if !ok {
		return
	}
	terms, ok := readTerms(response, request)
	if !ok {
		return
	}
	created, duplicate, err := h.commands.CreateDraft(
		request.Context(), request.Header.Get("Idempotency-Key"), UserID(user.ID()), terms,
	)
	if err != nil {
		writePactError(response, err)
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	httpapi.WriteJSON(response, status, pactEnvelope(created, duplicate))
}

func (h *Handler) UpdateDraft(response http.ResponseWriter, request *http.Request) {
	user, ok := pactUser(response, request)
	if !ok {
		return
	}
	terms, ok := readTerms(response, request)
	if !ok {
		return
	}
	updated, err := h.commands.UpdateDraft(
		request.Context(), UserID(user.ID()), PactID(request.PathValue("id")), terms,
	)
	if err != nil {
		writePactError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, pactEnvelope(updated, false))
}

func (h *Handler) Confirm(response http.ResponseWriter, request *http.Request) {
	user, ok := pactUser(response, request)
	if !ok {
		return
	}
	confirmed, err := h.commands.Confirm(
		request.Context(), UserID(user.ID()), PactID(request.PathValue("id")),
	)
	if err != nil {
		writePactError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, pactEnvelope(confirmed, false))
}

func (h *Handler) Pact(response http.ResponseWriter, request *http.Request) {
	user, ok := pactUser(response, request)
	if !ok {
		return
	}
	value, err := h.commands.Pact(request.Context(), UserID(user.ID()), PactID(request.PathValue("id")))
	if err != nil {
		writePactError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, pactEnvelope(value, false))
}

func (h *Handler) Pacts(response http.ResponseWriter, request *http.Request) {
	user, ok := pactUser(response, request)
	if !ok {
		return
	}
	values, err := h.commands.Pacts(request.Context(), UserID(user.ID()))
	if err != nil {
		writePactError(response, err)
		return
	}
	items := make([]pactAPIResponse, len(values))
	for index, value := range values {
		items[index] = pactEnvelope(value, false).Pact
	}
	httpapi.WriteJSON(response, http.StatusOK, struct {
		Pacts []pactAPIResponse `json:"pacts"`
	}{Pacts: items})
}

type termsInput struct {
	Goal                   string           `json:"goal"`
	Intent                 string           `json:"intent"`
	SpendingLimitMinor     int64            `json:"spendingLimitMinor"`
	StakeMinor             int64            `json:"stakeMinor"`
	Currency               string           `json:"currency"`
	StartsAt               time.Time        `json:"startsAt"`
	EndsAt                 time.Time        `json:"endsAt"`
	AccountabilityCircleID string           `json:"accountabilityCircleId"`
	BreakConsequence       BreakConsequence `json:"breakConsequence"`
}

func readTerms(response http.ResponseWriter, request *http.Request) (Terms, bool) {
	var input termsInput
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain valid pact terms.")
		return Terms{}, false
	}
	limit, err := NewMoney(input.SpendingLimitMinor, input.Currency)
	if err != nil {
		writePactError(response, err)
		return Terms{}, false
	}
	stake, err := NewMoney(input.StakeMinor, input.Currency)
	if err != nil {
		writePactError(response, err)
		return Terms{}, false
	}
	terms, err := NewTerms(
		input.Goal, input.Intent, limit, stake, input.StartsAt, input.EndsAt,
		CircleID(input.AccountabilityCircleID), input.BreakConsequence,
	)
	if err != nil {
		writePactError(response, err)
		return Terms{}, false
	}
	if terms.SpendingLimit().Currency() != "USD" {
		writePactError(response, ErrInvalidMoney)
		return Terms{}, false
	}
	return terms, true
}

type moneyAPIResponse struct {
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
}

type pactTermsAPIResponse struct {
	Goal                   string           `json:"goal"`
	Intent                 string           `json:"intent"`
	SpendingLimit          moneyAPIResponse `json:"spendingLimit"`
	Stake                  moneyAPIResponse `json:"stake"`
	StartsAt               time.Time        `json:"startsAt"`
	EndsAt                 time.Time        `json:"endsAt"`
	AccountabilityCircleID string           `json:"accountabilityCircleId"`
	BreakConsequence       BreakConsequence `json:"breakConsequence"`
}

type pactAPIResponse struct {
	ID                 string               `json:"id"`
	Terms              pactTermsAPIResponse `json:"terms"`
	Status             Status               `json:"status"`
	Outcome            Outcome              `json:"outcome"`
	StakeReservationID string               `json:"stakeReservationId,omitempty"`
	EmergencyCount     int                  `json:"emergencyCount"`
	Version            uint64               `json:"version"`
	Duplicate          bool                 `json:"duplicate"`
}

func pactEnvelope(value *Pact, duplicate bool) struct {
	Pact pactAPIResponse `json:"pact"`
} {
	terms := value.Terms()
	return struct {
		Pact pactAPIResponse `json:"pact"`
	}{Pact: pactAPIResponse{
		ID: string(value.ID()), Terms: pactTermsAPIResponse{
			Goal: terms.Goal(), Intent: terms.Intent(),
			SpendingLimit: moneyAPIResponse{AmountMinor: terms.SpendingLimit().AmountMinor(), Currency: terms.SpendingLimit().Currency()},
			Stake:         moneyAPIResponse{AmountMinor: terms.Stake().AmountMinor(), Currency: terms.Stake().Currency()},
			StartsAt:      terms.StartsAt(), EndsAt: terms.EndsAt(),
			AccountabilityCircleID: string(terms.AccountabilityCircleID()), BreakConsequence: terms.BreakConsequence(),
		},
		Status: value.Status(), Outcome: value.Outcome(), StakeReservationID: value.StakeReservationID(),
		EmergencyCount: value.EmergencyCount(), Version: value.Version(), Duplicate: duplicate,
	}}
}

func pactUser(response http.ResponseWriter, request *http.Request) (identity.User, bool) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
	}
	return user, ok
}

func writePactError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrIdempotencyKey):
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_idempotency_key", "A non-empty Idempotency-Key header is required.")
	case errors.Is(err, ErrPactNotFound):
		httpapi.WriteError(response, http.StatusNotFound, "pact_not_found", "The pact was not found.")
	case errors.Is(err, ErrInvalidMoney), errors.Is(err, ErrCurrencyMismatch), errors.Is(err, ErrInvalidPeriod),
		errors.Is(err, ErrMissingGoal), errors.Is(err, ErrMissingIntent), errors.Is(err, ErrMissingAccountabilityCircle),
		errors.Is(err, ErrInvalidBreakConsequence):
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "invalid_pact_terms", "The pact terms are invalid.")
	case errors.Is(err, ErrAccountabilityNotReady):
		httpapi.WriteError(response, http.StatusConflict, "accountability_not_ready", "The selected accountability circle is not ready for this pact.")
	case errors.Is(err, ErrInvalidTransition), errors.Is(err, ErrPeriodEnded):
		httpapi.WriteError(response, http.StatusConflict, "invalid_pact_transition", "The pact cannot make this transition.")
	case errors.Is(err, ErrIdempotencyConflict), errors.Is(err, ErrVersionConflict):
		httpapi.WriteError(response, http.StatusConflict, "pact_conflict", "The pact request conflicts with existing state.")
	case errors.Is(err, ledger.ErrInsufficientBalance):
		httpapi.WriteError(response, http.StatusConflict, "insufficient_balance", "The wallet does not have enough available balance for the pact stake.")
	case errors.Is(err, ErrActivePactCannotBeCancelled):
		httpapi.WriteError(response, http.StatusConflict, "active_pact_cannot_be_cancelled", "An active pact can only exit through the conscious-break flow.")
	default:
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "Controlled could not manage the pact.")
	}
}
