package identity_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
)

func TestRecipientHandlerListsExistingUsers(t *testing.T) {
	t.Parallel()

	current := testUser(t)
	recipient, err := identity.NewUser("user-2", "friend@example.com", "Friend", time.Now())
	if err != nil {
		t.Fatalf("NewUser() error = %v", err)
	}
	recipient, err = recipient.WithCtrldID("ctrld12345")
	if err != nil {
		t.Fatalf("WithCtrldID() error = %v", err)
	}
	directory := &recipientDirectory{users: []identity.User{recipient}}
	handler := identity.NewRecipientHandler(directory)
	protected := identity.RequireSession(
		stubSessionManager{user: current},
		http.HandlerFunc(handler.Recipients),
	)
	request := httptest.NewRequest(http.MethodGet, "/v1/users/recipients?query=ctrld12345", nil)
	request.Header.Set("Authorization", "Bearer secret-token")
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if directory.currentUserID != current.ID() || directory.query != "ctrld12345" {
		t.Fatalf("directory args = %q, %q", directory.currentUserID, directory.query)
	}
	var payload struct {
		Users []struct {
			CtrldID string `json:"ctrldId"`
		} `json:"users"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Users) != 1 || payload.Users[0].CtrldID != "ctrld12345" {
		t.Fatalf("users = %#v", payload.Users)
	}
}

type recipientDirectory struct {
	users         []identity.User
	currentUserID identity.UserID
	query         string
}

func (d *recipientDirectory) Recipients(
	_ context.Context,
	currentUserID identity.UserID,
	query string,
	_ int,
) ([]identity.User, error) {
	d.currentUserID = currentUserID
	d.query = query
	return d.users, nil
}
