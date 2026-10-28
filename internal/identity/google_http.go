package identity

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

type GoogleAuthenticator interface {
	Authenticate(context.Context, string) (StartedSession, error)
}

type GoogleAuthenticationHandler struct {
	authenticator GoogleAuthenticator
}

func NewGoogleAuthenticationHandler(authenticator GoogleAuthenticator) *GoogleAuthenticationHandler {
	return &GoogleAuthenticationHandler{authenticator: authenticator}
}

func (h *GoogleAuthenticationHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	var input struct {
		IDToken string `json:"idToken"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain a Google ID token.")
		return
	}
	started, err := h.authenticator.Authenticate(request.Context(), input.IDToken)
	if err != nil {
		if errors.Is(err, ErrInvalidGoogleToken) {
			httpapi.WriteError(response, http.StatusUnauthorized, "invalid_google_token", "Google could not verify this sign-in.")
			return
		}
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "Controlled could not complete Google sign-in.")
		return
	}

	setSessionCookie(response, started.Token, started.ExpiresAt)
	httpapi.WriteJSON(response, http.StatusOK, struct {
		SessionToken string       `json:"sessionToken"`
		User         userResponse `json:"user"`
		ExpiresAt    time.Time    `json:"expiresAt"`
	}{SessionToken: started.Token, User: newUserResponse(started.User), ExpiresAt: started.ExpiresAt})
}
