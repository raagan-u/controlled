// Package accountability owns partner invitations and immutable ordered circles.
package accountability

import (
	"time"

	"github.com/raagan-u/controlled/internal/identity"
)

type InvitationID string

type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "PENDING"
	InvitationAccepted InvitationStatus = "ACCEPTED"
	InvitationDeclined InvitationStatus = "DECLINED"
)

type Invitation struct {
	id         InvitationID
	ownerID    identity.UserID
	partnerID  identity.UserID
	status     InvitationStatus
	createdAt  time.Time
	resolvedAt time.Time
}

func NewInvitation(
	id InvitationID,
	ownerID identity.UserID,
	partnerID identity.UserID,
	createdAt time.Time,
) (Invitation, error) {
	if id == "" {
		return Invitation{}, ErrInvalidInvitationID
	}
	if ownerID == "" {
		return Invitation{}, ErrInvalidOwnerID
	}
	if partnerID == "" {
		return Invitation{}, ErrInvalidPartnerID
	}
	if ownerID == partnerID {
		return Invitation{}, ErrSelfPartner
	}
	if createdAt.IsZero() {
		return Invitation{}, ErrInvalidTimestamp
	}
	return Invitation{
		id: id, ownerID: ownerID, partnerID: partnerID,
		status: InvitationPending, createdAt: createdAt.UTC(),
	}, nil
}

func RestoreInvitation(
	id InvitationID,
	ownerID identity.UserID,
	partnerID identity.UserID,
	status InvitationStatus,
	createdAt time.Time,
	resolvedAt time.Time,
) (Invitation, error) {
	invitation, err := NewInvitation(id, ownerID, partnerID, createdAt)
	if err != nil {
		return Invitation{}, err
	}
	switch status {
	case InvitationPending:
		if !resolvedAt.IsZero() {
			return Invitation{}, ErrInvalidStatus
		}
	case InvitationAccepted, InvitationDeclined:
		if resolvedAt.IsZero() || resolvedAt.Before(createdAt) {
			return Invitation{}, ErrInvalidStatus
		}
	default:
		return Invitation{}, ErrInvalidStatus
	}
	invitation.status = status
	invitation.resolvedAt = resolvedAt.UTC()
	return invitation, nil
}

func (i *Invitation) Accept(actor identity.UserID, now time.Time) error {
	return i.resolve(actor, InvitationAccepted, now)
}

func (i *Invitation) Decline(actor identity.UserID, now time.Time) error {
	return i.resolve(actor, InvitationDeclined, now)
}

func (i *Invitation) resolve(actor identity.UserID, status InvitationStatus, now time.Time) error {
	if actor != i.partnerID {
		return ErrWrongInvitee
	}
	if now.IsZero() {
		return ErrInvalidTimestamp
	}
	if i.status == status {
		return nil
	}
	if i.status != InvitationPending {
		return ErrInvitationFinal
	}
	i.status = status
	i.resolvedAt = now.UTC()
	return nil
}

func (i Invitation) ID() InvitationID           { return i.id }
func (i Invitation) OwnerID() identity.UserID   { return i.ownerID }
func (i Invitation) PartnerID() identity.UserID { return i.partnerID }
func (i Invitation) Status() InvitationStatus   { return i.status }
func (i Invitation) CreatedAt() time.Time       { return i.createdAt }
func (i Invitation) ResolvedAt() time.Time      { return i.resolvedAt }
