package identity

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

const maxDisplayNameLength = 60

type ProfileStore interface {
	UpdateDisplayName(context.Context, UserID, string) (User, error)
}

type ProfileHandler struct {
	store ProfileStore
}

func NewProfileHandler(store ProfileStore) *ProfileHandler {
	return &ProfileHandler{store: store}
}

// Update lets the signed-in user choose their display name.
func (h *ProfileHandler) Update(response http.ResponseWriter, request *http.Request) {
	current, ok := CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	var input struct {
		DisplayName string `json:"displayName"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain a display name.")
		return
	}
	name := strings.TrimSpace(input.DisplayName)
	if name == "" || utf8.RuneCountInString(name) > maxDisplayNameLength {
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "invalid_display_name", "Enter a name up to 60 characters.")
		return
	}
	user, err := h.store.UpdateDisplayName(request.Context(), current.ID(), name)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			httpapi.WriteError(response, http.StatusUnauthorized, "invalid_session", "The Controlled session is invalid or expired.")
			return
		}
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "Controlled could not save the name.")
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, struct {
		User userResponse `json:"user"`
	}{User: newUserResponse(user)})
}
