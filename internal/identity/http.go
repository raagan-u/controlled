package identity

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/raagan-u/pay-pact/internal/platform/httpapi"
)

const SessionCookieName = "paypact_session"

type SessionManager interface {
	Start(context.Context, string) (StartedSession, error)
	Authenticate(context.Context, string) (User, error)
	End(context.Context, string) error
}

type SessionHandler struct {
	sessions SessionManager
}

func NewSessionHandler(sessions SessionManager) *SessionHandler {
	return &SessionHandler{sessions: sessions}
}

func (h *SessionHandler) Start(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain an email address.")
		return
	}
	started, err := h.sessions.Start(request.Context(), input.Email)
	if err != nil {
		writeSessionError(response, err)
		return
	}
	setSessionCookie(response, started.Token, started.ExpiresAt)
	httpapi.WriteJSON(response, http.StatusCreated, struct {
		User      userResponse `json:"user"`
		ExpiresAt time.Time    `json:"expiresAt"`
	}{User: newUserResponse(started.User), ExpiresAt: started.ExpiresAt})
}

func (h *SessionHandler) End(response http.ResponseWriter, request *http.Request) {
	token, ok := sessionToken(request)
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid PayPact session is required.")
		return
	}
	if err := h.sessions.End(request.Context(), token); err != nil && !errors.Is(err, ErrSessionNotFound) {
		writeSessionError(response, err)
		return
	}
	clearSessionCookie(response)
	response.WriteHeader(http.StatusNoContent)
}

type userResponse struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
}

func newUserResponse(user User) userResponse {
	return userResponse{
		ID:          string(user.ID()),
		Email:       user.Email(),
		DisplayName: user.DisplayName(),
		CreatedAt:   user.CreatedAt(),
	}
}

type userContextKey struct{}

func RequireSession(sessions SessionManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		token, ok := sessionToken(request)
		if !ok {
			httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid PayPact session is required.")
			return
		}
		user, err := sessions.Authenticate(request.Context(), token)
		if err != nil {
			if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrInvalidSessionToken) {
				httpapi.WriteError(response, http.StatusUnauthorized, "invalid_session", "The PayPact session is invalid or expired.")
				return
			}
			httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "PayPact could not validate the session.")
			return
		}
		ctx := context.WithValue(request.Context(), userContextKey{}, user)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func CurrentUser(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userContextKey{}).(User)
	return user, ok
}

func sessionToken(request *http.Request) (string, bool) {
	authorization := strings.TrimSpace(request.Header.Get("Authorization"))
	if strings.HasPrefix(authorization, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
		if token != "" {
			return token, true
		}
	}
	cookie, err := request.Cookie(SessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return "", false
	}
	return cookie.Value, true
}

func setSessionCookie(response http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(response, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookie(response http.ResponseWriter) {
	http.SetCookie(response, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func writeSessionError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidEmail):
		httpapi.WriteError(response, http.StatusUnprocessableEntity, "invalid_email", "A valid email address is required.")
	case errors.Is(err, ErrInvalidCredentials):
		httpapi.WriteError(response, http.StatusUnauthorized, "invalid_credentials", "No PayPact demo user exists for that email.")
	case errors.Is(err, ErrInvalidSessionToken), errors.Is(err, ErrSessionNotFound):
		httpapi.WriteError(response, http.StatusUnauthorized, "invalid_session", "The PayPact session is invalid or expired.")
	default:
		httpapi.WriteError(response, http.StatusInternalServerError, "internal_error", "PayPact could not manage the session.")
	}
}
