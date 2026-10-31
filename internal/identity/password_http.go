package identity

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

type PasswordAuthenticator interface {
	Register(context.Context, string, string, string) (StartedSession, error)
	Login(context.Context, string, string) (StartedSession, error)
}

type PasswordAuthenticationHandler struct {
	authenticator PasswordAuthenticator
}

func NewPasswordAuthenticationHandler(authenticator PasswordAuthenticator) *PasswordAuthenticationHandler {
	return &PasswordAuthenticationHandler{authenticator: authenticator}
}

func (h *PasswordAuthenticationHandler) Register(response http.ResponseWriter, request *http.Request) {
	h.authenticate(response, request, true)
}

func (h *PasswordAuthenticationHandler) Login(response http.ResponseWriter, request *http.Request) {
	h.authenticate(response, request, false)
}

func (h *PasswordAuthenticationHandler) authenticate(
	response http.ResponseWriter,
	request *http.Request,
	register bool,
) {
	var input struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain an email and password.")
		return
	}

	var started StartedSession
	var err error
	status := http.StatusOK
	if register {
		started, err = h.authenticator.Register(request.Context(), input.Email, input.Password, input.DisplayName)
		status = http.StatusCreated
	} else {
		started, err = h.authenticator.Login(request.Context(), input.Email, input.Password)
	}
	if err != nil {
		writePasswordAuthenticationError(response, err)
		return
	}
	setSessionCookie(response, started.Token, started.ExpiresAt)
	httpapi.WriteJSON(response, status, struct {
		SessionToken string       `json:"sessionToken"`
		User         userResponse `json:"user"`
		ExpiresAt    time.Time    `json:"expiresAt"`
	}{SessionToken: started.Token, User: newUserResponse(started.User), ExpiresAt: started.ExpiresAt})
}

func writePasswordAuthenticationError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidEmail), errors.Is(err, ErrInvalidPassword), errors.Is(err, ErrInvalidDisplayName):
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "invalid_credentials", "A display name, valid email, and non-empty password are required.")
	case errors.Is(err, ErrEmailAlreadyRegistered), errors.Is(err, ErrPasswordCredentialExists):
		httpapi.WriteError(response, http.StatusConflict, "email_registered", "An account already exists for this email.")
	case errors.Is(err, ErrInvalidCredentials):
		httpapi.WriteError(response, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect.")
	default:
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "Controlled could not complete password authentication.")
	}
}
