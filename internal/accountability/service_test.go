package accountability_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/accountability"
	"github.com/raagan-u/controlled/internal/identity"
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
	users       map[string]identity.User
	invitations map[accountability.InvitationID]accountability.Invitation
	accepted    map[identity.UserID]bool
	circle      accountability.Circle
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
