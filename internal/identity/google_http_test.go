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

func TestGoogleAuthenticationHandlerReturnsNativeSessionToken(t *testing.T) {
	t.Parallel()

	user := googleTestUser(t)
	handler := identity.NewGoogleAuthenticationHandler(googleAuthenticator{
		started: identity.StartedSession{
			User: user, Token: "controlled-session", ExpiresAt: time.Now().Add(time.Hour),
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/google", strings.NewReader(`{"idToken":"google-id-token"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		SessionToken string `json:"sessionToken"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.SessionToken != "controlled-session" {
		t.Fatalf("sessionToken = %q", payload.SessionToken)
	}
}

func TestGoogleAuthenticationHandlerRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	handler := identity.NewGoogleAuthenticationHandler(googleAuthenticator{err: identity.ErrInvalidGoogleToken})
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/google", strings.NewReader(`{"idToken":"bad"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

type googleAuthenticator struct {
	started identity.StartedSession
	err     error
}

func (a googleAuthenticator) Authenticate(context.Context, string) (identity.StartedSession, error) {
	return a.started, a.err
}
