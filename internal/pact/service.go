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
	ReserveStake(context.Context, UserID, PactID, string, string, string, time.Time) (*Pact, bool, error)
	Cancel(context.Context, UserID, PactID, string, time.Time) (*Pact, bool, error)
}

type MaintenanceResult struct {
	Started   int
	Completed int
}

type LifecycleRepository interface {
	StartDue(context.Context, time.Time) (int, error)
	DueForCompletion(context.Context, time.Time) ([]PactID, error)
	CompleteDue(context.Context, PactID, string, string, time.Time) error
}

func (s *Service) RunMaintenance(ctx context.Context) (MaintenanceResult, error) {
	if err := s.ready(); err != nil {
		return MaintenanceResult{}, err
	}
	repository, ok := s.repository.(LifecycleRepository)
	if !ok {
		return MaintenanceResult{}, ErrUnavailable
	}
	now := s.clock.Now().UTC()
	started, err := repository.StartDue(ctx, now)
	if err != nil {
		return MaintenanceResult{}, err
	}
	due, err := repository.DueForCompletion(ctx, now)
	if err != nil {
		return MaintenanceResult{}, err
	}
	result := MaintenanceResult{Started: started}
	for _, pactID := range due {
		completionJournalID, err := s.ids.NewID()
		if err != nil {
			return result, err
		}
		recoveryJournalID, err := s.ids.NewID()
		if err != nil {
			return result, err
		}
		if err := repository.CompleteDue(ctx, pactID, completionJournalID, recoveryJournalID, now); err != nil {
			return result, err
		}
		result.Completed++
	}
	return result, nil
}

func (s *Service) ReserveStake(
	ctx context.Context,
	idempotencyKey string,
	ownerID UserID,
	id PactID,
) (*Pact, bool, error) {
	if err := s.ready(); err != nil {
		return nil, false, err
	}
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return nil, false, ErrIdempotencyKey
	}
	journalID, err := s.ids.NewID()
	if err != nil {
		return nil, false, err
	}
	vaultCandidateID, err := s.ids.NewID()
	if err != nil {
		return nil, false, err
	}
	return s.repository.ReserveStake(
		ctx, ownerID, id, journalID, vaultCandidateID, key, s.clock.Now(),
	)
}

func (s *Service) Cancel(ctx context.Context, ownerID UserID, id PactID) (*Pact, bool, error) {
	if err := s.ready(); err != nil {
		return nil, false, err
	}
	journalID, err := s.ids.NewID()
	if err != nil {
		return nil, false, err
	}
	return s.repository.Cancel(ctx, ownerID, id, journalID, s.clock.Now())
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
	if terms.Duration() < MinimumPactDuration {
		return nil, false, ErrPactDurationTooShort
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
	if terms.Duration() < MinimumPactDuration {
		return nil, ErrPactDurationTooShort
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
