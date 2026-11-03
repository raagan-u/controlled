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
	"github.com/raagan-u/controlled/internal/notification"
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
	mux.Handle("GET /v1/accountability/invitations", identity.RequireSession(sessions, http.HandlerFunc(handler.Invitations)))
	mux.Handle("GET /v1/accountability/circles", identity.RequireSession(sessions, http.HandlerFunc(handler.Circles)))

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{name: "invite", method: http.MethodPost, path: "/v1/accountability/invitations", body: `{"partnerEmail":"partner@example.com"}`, want: http.StatusCreated},
		{name: "accept", method: http.MethodPost, path: "/v1/accountability/invitations/invite/accept", body: `{}`, want: http.StatusOK},
		{name: "decline", method: http.MethodPost, path: "/v1/accountability/invitations/invite/decline", body: `{}`, want: http.StatusOK},
		{name: "circle", method: http.MethodPost, path: "/v1/accountability/circles", body: `{"name":"Friends","partnerIds":["partner"]}`, want: http.StatusCreated},
		{name: "list invitations", method: http.MethodGet, path: "/v1/accountability/invitations", want: http.StatusOK},
		{name: "list circles", method: http.MethodGet, path: "/v1/accountability/circles", want: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
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

func TestDenialNotifiesOwnerBeforeRequestMovesToNextPartner(t *testing.T) {
	t.Parallel()

	reviewer, err := identity.NewUser("reviewer", "reviewer@example.com", "Riley", time.Now())
	if err != nil {
		t.Fatalf("new reviewer: %v", err)
	}
	notifier := &recordingAccountabilityNotifier{}
	handler := accountability.NewHandler(stubAccountabilityCommands{now: time.Now()}).EnableNotifications(notifier)
	mux := http.NewServeMux()
	mux.Handle("POST /v1/accountability/requests/{id}/deny", identity.RequireSession(accountabilitySessionManager{user: reviewer}, http.HandlerFunc(handler.DenyRequest)))
	request := httptest.NewRequest(http.MethodPost, "/v1/accountability/requests/request-1/deny", strings.NewReader(`{"reason":"Not necessary right now"}`))
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if len(notifier.deliveries) != 2 {
		t.Fatalf("notifications = %#v", notifier.deliveries)
	}
	owner := notifier.deliveries[0]
	if owner.recipient != "owner" || owner.message.Title != "Approval request denied" ||
		owner.message.Kind != notification.KindAccountabilityRequest || owner.message.EntityID != "request-1" ||
		!strings.Contains(owner.message.Body, "Riley denied") || !strings.Contains(owner.message.Body, "sent to another") {
		t.Fatalf("owner notification = %#v", owner)
	}
	if next := notifier.deliveries[1]; next.recipient != "next-reviewer" || next.message.Title != "Approval needed" {
		t.Fatalf("next reviewer notification = %#v", next)
	}
}

func (s stubAccountabilityCommands) Invitations(context.Context, identity.UserID) ([]accountability.Invitation, error) {
	invitation, err := accountability.NewInvitation("invite", "owner", "partner", s.now)
	return []accountability.Invitation{invitation}, err
}

func (s stubAccountabilityCommands) Circles(context.Context, identity.UserID) ([]accountability.Circle, error) {
	circle, err := accountability.NewCircle("circle", "owner", "Friends", []identity.UserID{"partner"}, s.now)
	return []accountability.Circle{circle}, err
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

func (s stubAccountabilityCommands) Requests(context.Context, identity.UserID) ([]accountability.Request, error) {
	return nil, nil
}

func (s stubAccountabilityCommands) Request(context.Context, identity.UserID, string) (accountability.Request, error) {
	return accountability.Request{}, nil
}

func (s stubAccountabilityCommands) Decide(_ context.Context, _ identity.UserID, requestID string, _ bool, _, _ string) (accountability.Request, error) {
	return accountability.Request{
		ID: requestID, OwnerID: "owner", AmountMinor: 1250, Currency: "USD", Status: "ASSIGNED", AssignedPartnerID: "next-reviewer",
	}, nil
}

func (s stubAccountabilityCommands) CancelRequest(context.Context, identity.UserID, string) (accountability.Request, error) {
	return accountability.Request{}, nil
}

type accountabilityDelivery struct {
	recipient identity.UserID
	message   notification.Message
}

type recordingAccountabilityNotifier struct{ deliveries []accountabilityDelivery }

func (notifier *recordingAccountabilityNotifier) Notify(recipient identity.UserID, message notification.Message) {
	notifier.deliveries = append(notifier.deliveries, accountabilityDelivery{recipient: recipient, message: message})
}

type accountabilitySessionManager struct{ user identity.User }

func (s accountabilitySessionManager) Start(context.Context, string) (identity.StartedSession, error) {
	return identity.StartedSession{}, nil
}

func (s accountabilitySessionManager) Authenticate(context.Context, string) (identity.User, error) {
	return s.user, nil
}

func (s accountabilitySessionManager) End(context.Context, string) error { return nil }
