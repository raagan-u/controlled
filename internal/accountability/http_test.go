package accountability_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/accountability"
	"github.com/raagan-u/controlled/internal/identity"
)

func TestAccountabilityHandlers(t *testing.T) {
	user, err := identity.NewUser("owner", "owner@example.com", "Owner", time.Now())
	if err != nil {
		t.Fatalf("new user: %v", err)
	}
	handler := accountability.NewHandler(stubAccountabilityCommands{now: time.Now()})
	sessions := accountabilitySessionManager{user: user}
	mux := http.NewServeMux()
	mux.Handle("POST /v1/accountability/invitations", identity.RequireSession(sessions, http.HandlerFunc(handler.Invite)))
	mux.Handle("POST /v1/accountability/invitations/{id}/accept", identity.RequireSession(sessions, http.HandlerFunc(handler.Accept)))
	mux.Handle("POST /v1/accountability/invitations/{id}/decline", identity.RequireSession(sessions, http.HandlerFunc(handler.Decline)))
	mux.Handle("POST /v1/accountability/circles", identity.RequireSession(sessions, http.HandlerFunc(handler.CreateCircle)))

	tests := []struct {
		name string
		path string
		body string
		want int
	}{
		{name: "invite", path: "/v1/accountability/invitations", body: `{"partnerEmail":"partner@example.com"}`, want: http.StatusCreated},
		{name: "accept", path: "/v1/accountability/invitations/invite/accept", body: `{}`, want: http.StatusOK},
		{name: "decline", path: "/v1/accountability/invitations/invite/decline", body: `{}`, want: http.StatusOK},
		{name: "circle", path: "/v1/accountability/circles", body: `{"name":"Friends","partnerIds":["partner"]}`, want: http.StatusCreated},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer token")
			request.Header.Set("Idempotency-Key", test.name)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.want, response.Body.String())
			}
		})
	}
}

type stubAccountabilityCommands struct{ now time.Time }

func (s stubAccountabilityCommands) Invite(context.Context, string, identity.UserID, string) (accountability.Invitation, bool, error) {
	invitation, err := accountability.NewInvitation("invite", "owner", "partner", s.now)
	return invitation, false, err
}

func (s stubAccountabilityCommands) Respond(_ context.Context, actor identity.UserID, _ accountability.InvitationID, accept bool) (accountability.Invitation, error) {
	invitation, err := accountability.NewInvitation("invite", "different-owner", actor, s.now)
	if err != nil {
		return accountability.Invitation{}, err
	}
	if accept {
		err = invitation.Accept(actor, s.now.Add(time.Minute))
	} else {
		err = invitation.Decline(actor, s.now.Add(time.Minute))
	}
	return invitation, err
}

func (s stubAccountabilityCommands) CreateCircle(_ context.Context, _ string, owner identity.UserID, name string, partners []identity.UserID) (accountability.Circle, bool, error) {
	circle, err := accountability.NewCircle("circle", owner, name, partners, s.now)
	return circle, false, err
}

type accountabilitySessionManager struct{ user identity.User }

func (s accountabilitySessionManager) Start(context.Context, string) (identity.StartedSession, error) {
	return identity.StartedSession{}, nil
}

func (s accountabilitySessionManager) Authenticate(context.Context, string) (identity.User, error) {
	return s.user, nil
}

func (s accountabilitySessionManager) End(context.Context, string) error { return nil }
