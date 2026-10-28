package identity_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raagan-u/controlled/internal/identity"
)

func TestPasswordRegistrationHandlerReturnsSession(t *testing.T) {
	t.Parallel()

	user := googleTestUser(t)
	handler := identity.NewPasswordAuthenticationHandler(passwordAuthenticator{
		started: googleStartedSession(user),
	})
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/password/register",
		strings.NewReader(`{"email":"founder@example.com","password":"password"}`),
	)
	response := httptest.NewRecorder()
	handler.Register(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		SessionToken string `json:"sessionToken"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.SessionToken != "session-token" {
		t.Fatalf("sessionToken = %q", payload.SessionToken)
	}
}

func TestPasswordLoginHandlerRejectsInvalidCredentials(t *testing.T) {
	t.Parallel()

	handler := identity.NewPasswordAuthenticationHandler(passwordAuthenticator{
		err: identity.ErrInvalidCredentials,
	})
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/password/login",
		strings.NewReader(`{"email":"founder@example.com","password":"wrong"}`),
	)
	response := httptest.NewRecorder()
	handler.Login(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

type passwordAuthenticator struct {
	started identity.StartedSession
	err     error
}

func (a passwordAuthenticator) Register(context.Context, string, string) (identity.StartedSession, error) {
	return a.started, a.err
}

func (a passwordAuthenticator) Login(context.Context, string, string) (identity.StartedSession, error) {
	return a.started, a.err
}
