package accountability

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/raagan-u/controlled/internal/custody"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/platform/httpapi"
	"github.com/raagan-u/controlled/internal/walletops"
)

type Commander interface {
	Invite(context.Context, string, identity.UserID, string) (Invitation, bool, error)
	Respond(context.Context, identity.UserID, InvitationID, bool) (Invitation, error)
	CreateCircle(context.Context, string, identity.UserID, string, []identity.UserID) (Circle, bool, error)
	Invitations(context.Context, identity.UserID) ([]Invitation, error)
	Circles(context.Context, identity.UserID) ([]Circle, error)
}

type requestCommander interface {
	Requests(context.Context, identity.UserID) ([]Request, error)
	Request(context.Context, identity.UserID, string) (Request, error)
	Decide(context.Context, identity.UserID, string, bool, string) (Request, error)
	CancelRequest(context.Context, identity.UserID, string) (Request, error)
}

func (h *Handler) Requests(response http.ResponseWriter, request *http.Request) {
	user, ok := currentUser(response, request)
	if !ok {
		return
	}
	commands, ok := h.commands.(requestCommander)
	if !ok {
		writeError(response, ErrUnavailable)
		return
	}
	items, err := commands.Requests(request.Context(), user.ID())
	if err != nil {
		writeError(response, err)
		return
	}
	output := make([]requestAPIResponse, len(items))
	for index, item := range items {
		output[index] = requestResponse(item).AccountabilityRequest
	}
	httpapi.WriteJSON(response, http.StatusOK, struct {
		Requests []requestAPIResponse `json:"accountabilityRequests"`
	}{Requests: output})
}

func (h *Handler) Request(response http.ResponseWriter, request *http.Request) {
	user, ok := currentUser(response, request)
	if !ok {
		return
	}
	commands, ok := h.commands.(requestCommander)
	if !ok {
		writeError(response, ErrUnavailable)
		return
	}
	item, err := commands.Request(request.Context(), user.ID(), request.PathValue("id"))
	if err != nil {
		writeError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, requestResponse(item))
}

func (h *Handler) ApproveRequest(response http.ResponseWriter, request *http.Request) {
	user, ok := currentUser(response, request)
	if !ok {
		return
	}
	var input struct {
		Classification string `json:"classification"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain an approval classification.")
		return
	}
	commands, ok := h.commands.(requestCommander)
	if !ok {
		writeError(response, ErrUnavailable)
		return
	}
	item, err := commands.Decide(request.Context(), user.ID(), request.PathValue("id"), true, input.Classification)
	if err != nil {
		writeError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, requestResponse(item))
}

func (h *Handler) DenyRequest(response http.ResponseWriter, request *http.Request) {
	h.decideWithoutBody(response, request, false)
}

func (h *Handler) CancelRequest(response http.ResponseWriter, request *http.Request) {
	user, ok := currentUser(response, request)
	if !ok {
		return
	}
	commands, ok := h.commands.(requestCommander)
	if !ok {
		writeError(response, ErrUnavailable)
		return
	}
	item, err := commands.CancelRequest(request.Context(), user.ID(), request.PathValue("id"))
	if err != nil {
		writeError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, requestResponse(item))
}

func (h *Handler) decideWithoutBody(response http.ResponseWriter, request *http.Request, approve bool) {
	user, ok := currentUser(response, request)
	if !ok {
		return
	}
	commands, ok := h.commands.(requestCommander)
	if !ok {
		writeError(response, ErrUnavailable)
		return
	}
	item, err := commands.Decide(request.Context(), user.ID(), request.PathValue("id"), approve, "")
	if err != nil {
		writeError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, requestResponse(item))
}

func (h *Handler) Invitations(response http.ResponseWriter, request *http.Request) {
	user, ok := currentUser(response, request)
	if !ok {
		return
	}
	invitations, err := h.commands.Invitations(request.Context(), user.ID())
	if err != nil {
		writeError(response, err)
		return
	}
	items := make([]invitationAPIResponse, len(invitations))
	for index, invitation := range invitations {
		items[index] = invitationEnvelope(invitation, false).Invitation
	}
	httpapi.WriteJSON(response, http.StatusOK, struct {
		Invitations []invitationAPIResponse `json:"invitations"`
	}{Invitations: items})
}

func (h *Handler) Circles(response http.ResponseWriter, request *http.Request) {
	user, ok := currentUser(response, request)
	if !ok {
		return
	}
	circles, err := h.commands.Circles(request.Context(), user.ID())
	if err != nil {
		writeError(response, err)
		return
	}
	items := make([]circleAPIResponse, len(circles))
	for index, circle := range circles {
		items[index] = circleEnvelope(circle, false).Circle
	}
	httpapi.WriteJSON(response, http.StatusOK, struct {
		Circles []circleAPIResponse `json:"circles"`
	}{Circles: items})
}

type Handler struct {
	commands Commander
}

func NewHandler(commands Commander) *Handler {
	return &Handler{commands: commands}
}

func (h *Handler) Invite(response http.ResponseWriter, request *http.Request) {
	user, ok := currentUser(response, request)
	if !ok {
		return
	}
	var input struct {
		PartnerEmail string `json:"partnerEmail"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain a partner email.")
		return
	}
	invitation, duplicate, err := h.commands.Invite(
		request.Context(), request.Header.Get("Idempotency-Key"), user.ID(), input.PartnerEmail,
	)
	if err != nil {
		writeError(response, err)
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	httpapi.WriteJSON(response, status, invitationEnvelope(invitation, duplicate))
}

func (h *Handler) Accept(response http.ResponseWriter, request *http.Request) {
	h.respond(response, request, true)
}

func (h *Handler) Decline(response http.ResponseWriter, request *http.Request) {
	h.respond(response, request, false)
}

func (h *Handler) respond(response http.ResponseWriter, request *http.Request, accept bool) {
	user, ok := currentUser(response, request)
	if !ok {
		return
	}
	invitation, err := h.commands.Respond(
		request.Context(), user.ID(), InvitationID(request.PathValue("id")), accept,
	)
	if err != nil {
		writeError(response, err)
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, invitationEnvelope(invitation, false))
}

func (h *Handler) CreateCircle(response http.ResponseWriter, request *http.Request) {
	user, ok := currentUser(response, request)
	if !ok {
		return
	}
	var input struct {
		Name       string   `json:"name"`
		PartnerIDs []string `json:"partnerIds"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain a circle name and ordered partner IDs.")
		return
	}
	partners := make([]identity.UserID, len(input.PartnerIDs))
	for index, partnerID := range input.PartnerIDs {
		partners[index] = identity.UserID(partnerID)
	}
	circle, duplicate, err := h.commands.CreateCircle(
		request.Context(), request.Header.Get("Idempotency-Key"), user.ID(), input.Name, partners,
	)
	if err != nil {
		writeError(response, err)
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	httpapi.WriteJSON(response, status, circleEnvelope(circle, duplicate))
}

type invitationAPIResponse struct {
	ID         string           `json:"id"`
	OwnerID    string           `json:"ownerId"`
	PartnerID  string           `json:"partnerId"`
	Status     InvitationStatus `json:"status"`
	CreatedAt  time.Time        `json:"createdAt"`
	ResolvedAt *time.Time       `json:"resolvedAt,omitempty"`
	Duplicate  bool             `json:"duplicate"`
}

func invitationEnvelope(invitation Invitation, duplicate bool) struct {
	Invitation invitationAPIResponse `json:"invitation"`
} {
	item := invitationAPIResponse{
		ID: string(invitation.ID()), OwnerID: string(invitation.OwnerID()),
		PartnerID: string(invitation.PartnerID()), Status: invitation.Status(),
		CreatedAt: invitation.CreatedAt(), Duplicate: duplicate,
	}
	if !invitation.ResolvedAt().IsZero() {
		resolvedAt := invitation.ResolvedAt()
		item.ResolvedAt = &resolvedAt
	}
	return struct {
		Invitation invitationAPIResponse `json:"invitation"`
	}{Invitation: item}
}

type circleAPIResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	PartnerIDs []string  `json:"partnerIds"`
	CreatedAt  time.Time `json:"createdAt"`
	Duplicate  bool      `json:"duplicate"`
}

type requestPactAPIResponse struct {
	PactID        string `json:"pactId"`
	LimitMinor    int64  `json:"limitMinor"`
	UsedMinor     int64  `json:"usedMinor"`
	ProposedMinor int64  `json:"proposedMinor"`
	OverageMinor  int64  `json:"overageMinor"`
}

type requestAPIResponse struct {
	ID                  string                   `json:"id"`
	IntentID            string                   `json:"intentId"`
	ExecutedOperationID string                   `json:"executedOperationId,omitempty"`
	OwnerID             string                   `json:"ownerId"`
	OperationType       string                   `json:"operationType"`
	Destination         string                   `json:"destination"`
	AmountMinor         int64                    `json:"amountMinor"`
	Currency            string                   `json:"currency"`
	Reason              string                   `json:"reason"`
	Status              string                   `json:"status"`
	Classification      string                   `json:"classification,omitempty"`
	AssignedPartnerID   string                   `json:"assignedPartnerId,omitempty"`
	Pacts               []requestPactAPIResponse `json:"blockingPacts"`
	CreatedAt           time.Time                `json:"createdAt"`
	ExpiresAt           time.Time                `json:"expiresAt"`
	ResolvedAt          *time.Time               `json:"resolvedAt,omitempty"`
}

func requestResponse(item Request) struct {
	AccountabilityRequest requestAPIResponse `json:"accountabilityRequest"`
} {
	pacts := make([]requestPactAPIResponse, len(item.Pacts))
	for index, pact := range item.Pacts {
		pacts[index] = requestPactAPIResponse{
			PactID: pact.PactID, LimitMinor: pact.LimitMinor, UsedMinor: pact.UsedMinor,
			ProposedMinor: pact.ProposedMinor, OverageMinor: pact.OverageMinor,
		}
	}
	result := requestAPIResponse{
		ID: item.ID, IntentID: string(item.IntentID), ExecutedOperationID: string(item.ExecutedOperationID), OwnerID: string(item.OwnerID),
		OperationType: string(item.OperationType), Destination: item.Destination,
		AmountMinor: item.AmountMinor, Currency: item.Currency, Reason: item.Reason,
		Status: item.Status, Classification: item.Classification,
		AssignedPartnerID: string(item.AssignedPartnerID), Pacts: pacts,
		CreatedAt: item.CreatedAt, ExpiresAt: item.ExpiresAt,
	}
	if !item.ResolvedAt.IsZero() {
		resolvedAt := item.ResolvedAt
		result.ResolvedAt = &resolvedAt
	}
	return struct {
		AccountabilityRequest requestAPIResponse `json:"accountabilityRequest"`
	}{AccountabilityRequest: result}
}

func circleEnvelope(circle Circle, duplicate bool) struct {
	Circle circleAPIResponse `json:"circle"`
} {
	partners := circle.Partners()
	partnerIDs := make([]string, len(partners))
	for index, partnerID := range partners {
		partnerIDs[index] = string(partnerID)
	}
	return struct {
		Circle circleAPIResponse `json:"circle"`
	}{Circle: circleAPIResponse{
		ID: string(circle.ID()), Name: circle.Name(), PartnerIDs: partnerIDs,
		CreatedAt: circle.CreatedAt(), Duplicate: duplicate,
	}}
}

func currentUser(response http.ResponseWriter, request *http.Request) (identity.User, bool) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
	}
	return user, ok
}

func writeError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrIdempotencyKey):
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_idempotency_key", "A non-empty Idempotency-Key header is required.")
	case errors.Is(err, ErrPartnerNotFound), errors.Is(err, ErrInvitationNotFound):
		httpapi.WriteError(response, http.StatusNotFound, "accountability_not_found", "The partner or invitation was not found.")
	case errors.Is(err, ErrSelfPartner), errors.Is(err, ErrMissingCircleName), errors.Is(err, ErrMissingPartners),
		errors.Is(err, ErrDuplicatePartner), errors.Is(err, ErrInvalidPartnerID):
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "invalid_accountability_relationship", "The accountability relationship details are invalid.")
	case errors.Is(err, ErrPartnerNotAccepted):
		httpapi.WriteError(response, http.StatusConflict, "partner_not_accepted", "Every circle partner must first accept an invitation.")
	case errors.Is(err, ErrInvitationExists):
		httpapi.WriteError(response, http.StatusConflict, "invitation_exists", "An active invitation or accepted relationship already exists.")
	case errors.Is(err, ErrInvitationFinal), errors.Is(err, ErrWrongInvitee):
		httpapi.WriteError(response, http.StatusConflict, "invitation_not_resolvable", "This user cannot resolve the invitation in its current state.")
	case errors.Is(err, ErrIdempotencyConflict):
		httpapi.WriteError(response, http.StatusConflict, "idempotency_conflict", "This Idempotency-Key was already used with different input.")
	case errors.Is(err, ErrRequestNotFound):
		httpapi.WriteError(response, http.StatusNotFound, "accountability_request_not_found", "The accountability request was not found.")
	case errors.Is(err, ErrMissingReason):
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "accountability_reason_required", "A reason is required for an accountability request.")
	case errors.Is(err, ErrInvalidDecision):
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "invalid_accountability_decision", "Approval must be classified as EMERGENCY or CONSCIOUS_BREAK.")
	case errors.Is(err, ErrWrongReviewer):
		httpapi.WriteError(response, http.StatusForbidden, "not_current_reviewer", "Only the currently assigned partner can decide this request.")
	case errors.Is(err, ErrRequestFinal):
		httpapi.WriteError(response, http.StatusConflict, "accountability_request_final", "This accountability request is already resolved.")
	case errors.Is(err, ledger.ErrInsufficientBalance):
		httpapi.WriteError(response, http.StatusConflict, "insufficient_balance", "The wallet no longer has enough available balance for the approved transaction.")
	case errors.Is(err, walletops.ErrPolicyDenied):
		httpapi.WriteError(response, http.StatusConflict, "approval_stale", "The approved transaction no longer satisfies the current pact state.")
	case errors.Is(err, custody.ErrUnavailable):
		httpapi.WriteError(response, http.StatusBadGateway, "custody_unavailable", "The custody provider is unavailable.")
	default:
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "Controlled could not manage accountability relationships.")
	}
}
