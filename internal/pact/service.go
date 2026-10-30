package pact

import (
	"context"
	"strings"
	"time"
)

type IDGenerator interface {
	NewID() (string, error)
}

type Clock interface {
	Now() time.Time
}

type Repository interface {
	Create(context.Context, string, *Pact) (*Pact, bool, error)
	Save(context.Context, *Pact, uint64) error
	ByID(context.Context, UserID, PactID) (*Pact, error)
	ForOwner(context.Context, UserID) ([]*Pact, error)
	CircleFacts(context.Context, UserID, CircleID) (ConfirmationFacts, error)
}

type Service struct {
	ids        IDGenerator
	clock      Clock
	repository Repository
}

func NewService(ids IDGenerator, clock Clock, repository Repository) *Service {
	return &Service{ids: ids, clock: clock, repository: repository}
}

func (s *Service) CreateDraft(
	ctx context.Context,
	idempotencyKey string,
	ownerID UserID,
	terms Terms,
) (*Pact, bool, error) {
	if err := s.ready(); err != nil {
		return nil, false, err
	}
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return nil, false, ErrIdempotencyKey
	}
	if _, err := s.repository.CircleFacts(ctx, ownerID, terms.AccountabilityCircleID()); err != nil {
		return nil, false, err
	}
	id, err := s.ids.NewID()
	if err != nil {
		return nil, false, err
	}
	draft, err := CreateDraft(PactID(id), ownerID, terms, s.clock.Now())
	if err != nil {
		return nil, false, err
	}
	return s.repository.Create(ctx, key, draft)
}

func (s *Service) UpdateDraft(ctx context.Context, ownerID UserID, id PactID, terms Terms) (*Pact, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if _, err := s.repository.CircleFacts(ctx, ownerID, terms.AccountabilityCircleID()); err != nil {
		return nil, err
	}
	current, err := s.repository.ByID(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	if current.Status() == StatusDraft && current.Terms().Equal(terms) {
		return current, nil
	}
	expectedVersion := current.Version()
	if err := current.UpdateDraft(terms, s.clock.Now()); err != nil {
		return nil, err
	}
	if err := s.repository.Save(ctx, current, expectedVersion); err != nil {
		return nil, err
	}
	return current, nil
}

func (s *Service) Confirm(ctx context.Context, ownerID UserID, id PactID) (*Pact, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	current, err := s.repository.ByID(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	if current.Status() == StatusAwaitingStake {
		return current, nil
	}
	facts, err := s.repository.CircleFacts(ctx, ownerID, current.Terms().AccountabilityCircleID())
	if err != nil {
		return nil, err
	}
	expectedVersion := current.Version()
	if err := current.Confirm(facts, s.clock.Now()); err != nil {
		return nil, err
	}
	if err := s.repository.Save(ctx, current, expectedVersion); err != nil {
		return nil, err
	}
	return current, nil
}

func (s *Service) Pact(ctx context.Context, ownerID UserID, id PactID) (*Pact, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.repository.ByID(ctx, ownerID, id)
}

func (s *Service) Pacts(ctx context.Context, ownerID UserID) ([]*Pact, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.repository.ForOwner(ctx, ownerID)
}

func (s *Service) ready() error {
	if s == nil || s.ids == nil || s.clock == nil || s.repository == nil {
		return ErrUnavailable
	}
	return nil
}
