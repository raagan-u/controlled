package identity_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
)

func TestSessionHandlerStartsSecureCookie(t *testing.T) {
	t.Parallel()

	user := testUser(t)
	expiresAt := time.Now().UTC().Add(time.Hour)
	handler := identity.NewSessionHandler(stubSessionManager{
		started: identity.StartedSession{User: user, Token: "secret-token", ExpiresAt: expiresAt},
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/demo/sessions", strings.NewReader(`{"email":"founder@example.com"}`))
	response := httptest.NewRecorder()
	handler.Start(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != identity.SessionCookieName ||
		cookies[0].Value != "secret-token" || !cookies[0].HttpOnly || !cookies[0].Secure ||
		cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie = %#v", cookies)
	}
	var payload struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.User.ID != string(user.ID()) {
		t.Fatalf("user ID = %q, want %q", payload.User.ID, user.ID())
	}
}

func TestRequireSessionAcceptsBearerToken(t *testing.T) {
	t.Parallel()

	user := testUser(t)
	manager := stubSessionManager{user: user}
	protected := identity.RequireSession(manager, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		current, ok := identity.CurrentUser(request.Context())
		if !ok || current.ID() != user.ID() {
			t.Fatalf("CurrentUser() = %#v, %v", current, ok)
		}
		response.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/v1/users/current", nil)
	request.Header.Set("Authorization", "Bearer secret-token")
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestRequireSessionRejectsMissingToken(t *testing.T) {
	t.Parallel()

	protected := identity.RequireSession(stubSessionManager{}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler was called")
	}))
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/users/current", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestCurrentUserHandler(t *testing.T) {
	t.Parallel()

	user := testUser(t)
	protected := identity.RequireSession(
		stubSessionManager{user: user},
		http.HandlerFunc(identity.CurrentUserHandler),
	)
	request := httptest.NewRequest(http.MethodGet, "/v1/users/current", nil)
	request.Header.Set("Authorization", "Bearer secret-token")
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func testUser(t *testing.T) identity.User {
	t.Helper()
	user, err := identity.NewUser("user-1", "founder@example.com", "Raagan", time.Now())
	if err != nil {
		t.Fatalf("NewUser() error = %v", err)
	}
	return user
}

type stubSessionManager struct {
	started identity.StartedSession
	user    identity.User
}

func (s stubSessionManager) Start(context.Context, string) (identity.StartedSession, error) {
	return s.started, nil
}

func (s stubSessionManager) Authenticate(_ context.Context, token string) (identity.User, error) {
	if token != "secret-token" {
		return identity.User{}, identity.ErrInvalidCredentials
	}
	return s.user, nil
}

func (s stubSessionManager) End(context.Context, string) error { return nil }
