package accountability

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
)

type IDGenerator interface {
	NewID() (string, error)
}

type Clock interface {
	Now() time.Time
}

type Repository interface {
	UserByEmail(context.Context, string) (identity.User, error)
	CreateInvitation(context.Context, string, Invitation) (Invitation, bool, error)
	InvitationByID(context.Context, InvitationID) (Invitation, error)
	SaveInvitation(context.Context, Invitation) error
	InvitationsForUser(context.Context, identity.UserID) ([]Invitation, error)
	AcceptedPartners(context.Context, identity.UserID, []identity.UserID) (map[identity.UserID]bool, error)
	CreateCircle(context.Context, string, Circle) (Circle, bool, error)
	CirclesForOwner(context.Context, identity.UserID) ([]Circle, error)
}

func (s *Service) Invitations(ctx context.Context, userID identity.UserID) ([]Invitation, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if userID == "" {
		return nil, ErrInvalidPartnerID
	}
	return s.repository.InvitationsForUser(ctx, userID)
}

func (s *Service) Circles(ctx context.Context, ownerID identity.UserID) ([]Circle, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if ownerID == "" {
		return nil, ErrInvalidOwnerID
	}
	return s.repository.CirclesForOwner(ctx, ownerID)
}

type Service struct {
	ids        IDGenerator
	clock      Clock
	repository Repository
}

func NewService(ids IDGenerator, clock Clock, repository Repository) *Service {
	return &Service{ids: ids, clock: clock, repository: repository}
}

func (s *Service) Invite(
	ctx context.Context,
	idempotencyKey string,
	ownerID identity.UserID,
	partnerEmail string,
) (Invitation, bool, error) {
	if err := s.ready(); err != nil {
		return Invitation{}, false, err
	}
	key, err := normalizedKey(idempotencyKey)
	if err != nil {
		return Invitation{}, false, err
	}
	if ownerID == "" {
		return Invitation{}, false, ErrInvalidOwnerID
	}
	partner, err := s.repository.UserByEmail(ctx, strings.ToLower(strings.TrimSpace(partnerEmail)))
	if err != nil {
		if errors.Is(err, identity.ErrInvalidCredentials) {
			return Invitation{}, false, ErrPartnerNotFound
		}
		return Invitation{}, false, err
	}
	id, err := s.ids.NewID()
	if err != nil {
		return Invitation{}, false, err
	}
	invitation, err := NewInvitation(InvitationID(id), ownerID, partner.ID(), s.clock.Now())
	if err != nil {
		return Invitation{}, false, err
	}
	return s.repository.CreateInvitation(ctx, key, invitation)
}

func (s *Service) Respond(
	ctx context.Context,
	actorID identity.UserID,
	invitationID InvitationID,
	accept bool,
) (Invitation, error) {
	if err := s.ready(); err != nil {
		return Invitation{}, err
	}
	if invitationID == "" {
		return Invitation{}, ErrInvalidInvitationID
	}
	invitation, err := s.repository.InvitationByID(ctx, invitationID)
	if err != nil {
		return Invitation{}, err
	}
	if accept {
		err = invitation.Accept(actorID, s.clock.Now())
	} else {
		err = invitation.Decline(actorID, s.clock.Now())
	}
	if err != nil {
		return Invitation{}, err
	}
	if err := s.repository.SaveInvitation(ctx, invitation); err != nil {
		return Invitation{}, err
	}
	return invitation, nil
}

func (s *Service) CreateCircle(
	ctx context.Context,
	idempotencyKey string,
	ownerID identity.UserID,
	name string,
	partners []identity.UserID,
) (Circle, bool, error) {
	if err := s.ready(); err != nil {
		return Circle{}, false, err
	}
	key, err := normalizedKey(idempotencyKey)
	if err != nil {
		return Circle{}, false, err
	}
	id, err := s.ids.NewID()
	if err != nil {
		return Circle{}, false, err
	}
	circle, err := NewCircle(CircleID(id), ownerID, name, partners, s.clock.Now())
	if err != nil {
		return Circle{}, false, err
	}
	accepted, err := s.repository.AcceptedPartners(ctx, ownerID, circle.Partners())
	if err != nil {
		return Circle{}, false, err
	}
	for _, partnerID := range circle.Partners() {
		if !accepted[partnerID] {
			return Circle{}, false, ErrPartnerNotAccepted
		}
	}
	return s.repository.CreateCircle(ctx, key, circle)
}

func (s *Service) ready() error {
	if s == nil || s.ids == nil || s.clock == nil || s.repository == nil {
		return ErrUnavailable
	}
	return nil
}

func normalizedKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrIdempotencyKey
	}
	return value, nil
}
