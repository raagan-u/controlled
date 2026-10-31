package identity_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
)

type profileStore struct {
	userID identity.UserID
	name   string
	calls  int
}

func (s *profileStore) UpdateDisplayName(_ context.Context, userID identity.UserID, name string) (identity.User, error) {
	s.calls++
	s.userID = userID
	s.name = name
	return identity.NewUser(userID, "founder@example.com", name, time.Now())
}

func updateName(t *testing.T, store *profileStore, body string) *httptest.ResponseRecorder {
	t.Helper()
	handler := identity.NewProfileHandler(store)
	protected := identity.RequireSession(stubSessionManager{user: testUser(t)}, http.HandlerFunc(handler.Update))
	request := httptest.NewRequest(http.MethodPatch, "/v1/users/current", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer secret-token")
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	return response
}

func TestProfileHandlerSavesTrimmedName(t *testing.T) {
	t.Parallel()

	store := &profileStore{}
	response := updateName(t, store, `{"displayName":"  Ada Lovelace  "}`)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if store.name != "Ada Lovelace" || store.userID != testUser(t).ID() {
		t.Fatalf("store got %q for %q", store.name, store.userID)
	}
	if !strings.Contains(response.Body.String(), `"displayName":"Ada Lovelace"`) || !strings.Contains(response.Body.String(), `"namePending":false`) {
		t.Fatalf("body = %s", response.Body.String())
	}
}

func TestProfileHandlerRejectsBlankAndOversizedNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "   ", strings.Repeat("a", 61)} {
		store := &profileStore{}
		response := updateName(t, store, `{"displayName":"`+name+`"}`)
		if response.Code != http.StatusUnprocessableEntity || store.calls != 0 {
			t.Fatalf("name %q: status = %d, store calls = %d", name, response.Code, store.calls)
		}
	}
}

func TestProfileHandlerRequiresSession(t *testing.T) {
	t.Parallel()

	handler := identity.NewProfileHandler(&profileStore{})
	response := httptest.NewRecorder()
	handler.Update(response, httptest.NewRequest(http.MethodPatch, "/v1/users/current", strings.NewReader(`{"displayName":"Ada"}`)))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}
