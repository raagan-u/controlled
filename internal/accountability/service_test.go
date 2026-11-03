package accountability_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/accountability"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/walletops"
)

type fixedIDs struct{ values []string }

func (g *fixedIDs) NewID() (string, error) {
	value := g.values[0]
	g.values = g.values[1:]
	return value, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type memoryRepository struct {
	users          map[string]identity.User
	invitations    map[accountability.InvitationID]accountability.Invitation
	accepted       map[identity.UserID]bool
	circle         accountability.Circle
	decisionReason string
	request        accountability.Request
}

func (r *memoryRepository) CreateDebitRequest(context.Context, walletops.PendingDebit, string, string) (walletops.ReviewRequest, error) {
	return walletops.ReviewRequest{}, nil
}

func (r *memoryRepository) RequestsForUser(context.Context, identity.UserID) ([]accountability.Request, error) {
	return []accountability.Request{r.request}, nil
}

func (r *memoryRepository) RequestForUser(context.Context, identity.UserID, string) (accountability.Request, error) {
	return r.request, nil
}

func (r *memoryRepository) DecideRequest(_ context.Context, _ identity.UserID, _ string, _ bool, _ string, reason, _ string, _ time.Time) error {
	r.decisionReason = reason
	return nil
}

func (r *memoryRepository) CancelRequest(context.Context, identity.UserID, string, time.Time) error {
	return nil
}

func (r *memoryRepository) UserByEmail(_ context.Context, email string) (identity.User, error) {
	user, ok := r.users[email]
	if !ok {
		return identity.User{}, identity.ErrInvalidCredentials
	}
	return user, nil
}

func (r *memoryRepository) CreateInvitation(_ context.Context, _ string, invitation accountability.Invitation) (accountability.Invitation, bool, error) {
	r.invitations[invitation.ID()] = invitation
	return invitation, false, nil
}

func (r *memoryRepository) InvitationByID(_ context.Context, id accountability.InvitationID) (accountability.Invitation, error) {
	invitation, ok := r.invitations[id]
	if !ok {
		return accountability.Invitation{}, accountability.ErrInvitationNotFound
	}
	return invitation, nil
}

func (r *memoryRepository) SaveInvitation(_ context.Context, invitation accountability.Invitation) error {
	r.invitations[invitation.ID()] = invitation
	return nil
}

func (r *memoryRepository) InvitationsForUser(_ context.Context, _ identity.UserID) ([]accountability.Invitation, error) {
	result := make([]accountability.Invitation, 0, len(r.invitations))
	for _, invitation := range r.invitations {
		result = append(result, invitation)
	}
	return result, nil
}

func (r *memoryRepository) AcceptedPartners(_ context.Context, _ identity.UserID, partners []identity.UserID) (map[identity.UserID]bool, error) {
	result := make(map[identity.UserID]bool, len(partners))
	for _, partnerID := range partners {
		result[partnerID] = r.accepted[partnerID]
	}
	return result, nil
}

func (r *memoryRepository) CreateCircle(_ context.Context, _ string, circle accountability.Circle) (accountability.Circle, bool, error) {
	r.circle = circle
	return circle, false, nil
}

func (r *memoryRepository) CirclesForOwner(_ context.Context, _ identity.UserID) ([]accountability.Circle, error) {
	if r.circle.ID() == "" {
		return nil, nil
	}
	return []accountability.Circle{r.circle}, nil
}

func TestServiceInvitesRegisteredPartner(t *testing.T) {
	now := time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)
	partner, err := identity.NewUser("partner", "partner@example.com", "Partner", now)
	if err != nil {
		t.Fatalf("new user: %v", err)
	}
	repository := &memoryRepository{
		users:       map[string]identity.User{"partner@example.com": partner},
		invitations: make(map[accountability.InvitationID]accountability.Invitation),
	}
	service := accountability.NewService(&fixedIDs{values: []string{"invite"}}, fixedClock{now}, repository)

	invitation, duplicate, err := service.Invite(t.Context(), "invite-key", "owner", " PARTNER@example.com ")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if duplicate || invitation.PartnerID() != partner.ID() || invitation.Status() != accountability.InvitationPending {
		t.Fatalf("invitation = %#v, duplicate = %v", invitation, duplicate)
	}
}

func TestServiceAcceptsInvitation(t *testing.T) {
	now := time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)
	invitation, err := accountability.NewInvitation("invite", "owner", "partner", now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("new invitation: %v", err)
	}
	repository := &memoryRepository{invitations: map[accountability.InvitationID]accountability.Invitation{"invite": invitation}}
	service := accountability.NewService(&fixedIDs{}, fixedClock{now}, repository)

	resolved, err := service.Respond(t.Context(), "partner", "invite", true)
	if err != nil {
		t.Fatalf("respond: %v", err)
	}
	if resolved.Status() != accountability.InvitationAccepted {
		t.Fatalf("status = %s", resolved.Status())
	}
}

func TestServiceRequiresAcceptedPartnersForCircle(t *testing.T) {
	now := time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)
	repository := &memoryRepository{accepted: map[identity.UserID]bool{"accepted": true}}
	service := accountability.NewService(&fixedIDs{values: []string{"circle-1", "circle-2"}}, fixedClock{now}, repository)

	_, _, err := service.CreateCircle(t.Context(), "circle-key", "owner", "Friends", []identity.UserID{"accepted", "pending"})
	if !errors.Is(err, accountability.ErrPartnerNotAccepted) {
		t.Fatalf("expected partner not accepted, got %v", err)
	}
	circle, duplicate, err := service.CreateCircle(t.Context(), "circle-key-2", "owner", "Friends", []identity.UserID{"accepted"})
	if err != nil {
		t.Fatalf("create circle: %v", err)
	}
	if duplicate || len(circle.Partners()) != 1 || circle.Partners()[0] != "accepted" {
		t.Fatalf("circle = %#v, duplicate = %v", circle, duplicate)
	}
}

func TestDenialRequiresAndPersistsReason(t *testing.T) {
	now := time.Date(2026, time.November, 3, 12, 0, 0, 0, time.UTC)
	repository := &memoryRepository{request: accountability.Request{ID: "request-1", Status: "DENIED"}}
	service := accountability.NewService(&fixedIDs{values: []string{"assignment-1"}}, fixedClock{now}, repository)

	_, err := service.Decide(t.Context(), "partner", "request-1", false, "", "   ")
	if !errors.Is(err, accountability.ErrMissingDenialReason) {
		t.Fatalf("empty denial reason error = %v", err)
	}
	result, err := service.Decide(t.Context(), "partner", "request-1", false, "", "  Not necessary right now  ")
	if err != nil {
		t.Fatalf("deny request: %v", err)
	}
	if result.Status != "DENIED" || repository.decisionReason != "Not necessary right now" {
		t.Fatalf("result = %+v, persisted reason = %q", result, repository.decisionReason)
	}
}
