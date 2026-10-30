package accountability

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

type Commander interface {
	Invite(context.Context, string, identity.UserID, string) (Invitation, bool, error)
	Respond(context.Context, identity.UserID, InvitationID, bool) (Invitation, error)
	CreateCircle(context.Context, string, identity.UserID, string, []identity.UserID) (Circle, bool, error)
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
	default:
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "Controlled could not manage accountability relationships.")
	}
}
