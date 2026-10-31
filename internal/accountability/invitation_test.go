package accountability_test

import (
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/accountability"
	"github.com/raagan-u/controlled/internal/identity"
)

func TestInvitationCanOnlyBeResolvedByInvitee(t *testing.T) {
	createdAt := time.Date(2026, time.October, 30, 10, 0, 0, 0, time.UTC)
	invitation, err := accountability.NewInvitation(
		"00000000-0000-4000-8000-000000000001",
		identity.UserID("owner"),
		identity.UserID("partner"),
		createdAt,
	)
	if err != nil {
		t.Fatalf("new invitation: %v", err)
	}

	if err := invitation.Accept(identity.UserID("owner"), createdAt.Add(time.Minute)); !errors.Is(err, accountability.ErrWrongInvitee) {
		t.Fatalf("expected wrong invitee, got %v", err)
	}
	if err := invitation.Accept(identity.UserID("partner"), createdAt.Add(time.Minute)); err != nil {
		t.Fatalf("accept invitation: %v", err)
	}
	if invitation.Status() != accountability.InvitationAccepted {
		t.Fatalf("status = %s, want accepted", invitation.Status())
	}
	if invitation.ResolvedAt() != createdAt.Add(time.Minute) {
		t.Fatalf("resolved at = %s", invitation.ResolvedAt())
	}
}

func TestInvitationDecisionIsIdempotentAndFinal(t *testing.T) {
	now := time.Date(2026, time.October, 30, 10, 0, 0, 0, time.UTC)
	invitation, err := accountability.NewInvitation("invite", "owner", "partner", now)
	if err != nil {
		t.Fatalf("new invitation: %v", err)
	}
	if err := invitation.Decline("partner", now.Add(time.Minute)); err != nil {
		t.Fatalf("decline invitation: %v", err)
	}
	if err := invitation.Decline("partner", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("repeat decline: %v", err)
	}
	if err := invitation.Accept("partner", now.Add(3*time.Minute)); !errors.Is(err, accountability.ErrInvitationFinal) {
		t.Fatalf("expected final invitation, got %v", err)
	}
}

func TestInvitationRejectsOwnerAsPartner(t *testing.T) {
	_, err := accountability.NewInvitation("invite", "same", "same", time.Now())
	if !errors.Is(err, accountability.ErrSelfPartner) {
		t.Fatalf("expected self partner error, got %v", err)
	}
}
