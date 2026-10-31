package identity

import (
	"context"
	"net/http"
	"strings"

	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

type RecipientDirectory interface {
	Recipients(context.Context, UserID, string, int) ([]User, error)
}

type RecipientHandler struct {
	directory RecipientDirectory
}

func NewRecipientHandler(directory RecipientDirectory) *RecipientHandler {
	return &RecipientHandler{directory: directory}
}

func (h *RecipientHandler) Recipients(response http.ResponseWriter, request *http.Request) {
	current, ok := CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	query := strings.TrimSpace(request.URL.Query().Get("query"))
	users, err := h.directory.Recipients(request.Context(), current.ID(), query, 50)
	if err != nil {
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "Controlled could not load recipients.")
		return
	}
	items := make([]userResponse, 0, len(users))
	for _, user := range users {
		items = append(items, newUserResponse(user))
	}
	httpapi.WriteJSON(response, http.StatusOK, struct {
		Users []userResponse `json:"users"`
	}{Users: items})
}
