package accountability

import (
	"context"
	"strings"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/walletops"
)

const (
	assignmentLifetime = 6 * time.Hour
	requestLifetime    = 24 * time.Hour
)

type RequestPact struct {
	PactID        string
	LimitMinor    int64
	UsedMinor     int64
	ProposedMinor int64
	OverageMinor  int64
}

type Request struct {
	ID                string
	IntentID          walletops.OperationID
	OwnerID           identity.UserID
	OperationType     walletops.DebitKind
	Destination       string
	AmountMinor       int64
	Currency          string
	Reason            string
	Status            string
	Classification    string
	AssignedPartnerID identity.UserID
	Pacts             []RequestPact
	CreatedAt         time.Time
	ExpiresAt         time.Time
	ResolvedAt        time.Time
	Duplicate         bool
}

type RequestRepository interface {
	CreateDebitRequest(context.Context, walletops.PendingDebit, string, string) (walletops.ReviewRequest, error)
	RequestsForUser(context.Context, identity.UserID) ([]Request, error)
	RequestForUser(context.Context, identity.UserID, string) (Request, error)
}

type DecisionRepository interface {
	DecideRequest(context.Context, identity.UserID, string, bool, string, string, time.Time) error
	CancelRequest(context.Context, identity.UserID, string, time.Time) error
}

func (s *Service) RequestDebit(ctx context.Context, debit walletops.PendingDebit) (walletops.ReviewRequest, error) {
	if err := s.ready(); err != nil {
		return walletops.ReviewRequest{}, err
	}
	debit.Reason = strings.TrimSpace(debit.Reason)
	if debit.Reason == "" {
		return walletops.ReviewRequest{}, ErrMissingReason
	}
	if debit.IntentID == "" || debit.OwnerID == "" || debit.IdempotencyKey == "" ||
		debit.Destination == "" || !debit.Amount.Positive() || len(debit.Blocks) == 0 || debit.CreatedAt.IsZero() {
		return walletops.ReviewRequest{}, ErrInvalidStatus
	}
	repository, ok := s.repository.(RequestRepository)
	if !ok {
		return walletops.ReviewRequest{}, ErrUnavailable
	}
	requestID, err := s.ids.NewID()
	if err != nil {
		return walletops.ReviewRequest{}, err
	}
	assignmentID, err := s.ids.NewID()
	if err != nil {
		return walletops.ReviewRequest{}, err
	}
	return repository.CreateDebitRequest(ctx, debit, requestID, assignmentID)
}

func (s *Service) Requests(ctx context.Context, userID identity.UserID) ([]Request, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	repository, ok := s.repository.(RequestRepository)
	if !ok {
		return nil, ErrUnavailable
	}
	return repository.RequestsForUser(ctx, userID)
}

func (s *Service) Request(ctx context.Context, userID identity.UserID, requestID string) (Request, error) {
	if err := s.ready(); err != nil {
		return Request{}, err
	}
	repository, ok := s.repository.(RequestRepository)
	if !ok {
		return Request{}, ErrUnavailable
	}
	return repository.RequestForUser(ctx, userID, requestID)
}

func (s *Service) Decide(
	ctx context.Context,
	actorID identity.UserID,
	requestID string,
	approve bool,
	classification string,
) (Request, error) {
	if err := s.ready(); err != nil {
		return Request{}, err
	}
	classification = strings.ToUpper(strings.TrimSpace(classification))
	if approve && classification != "EMERGENCY" && classification != "CONSCIOUS_BREAK" {
		return Request{}, ErrInvalidDecision
	}
	if !approve {
		classification = ""
	}
	repository, ok := s.repository.(DecisionRepository)
	if !ok {
		return Request{}, ErrUnavailable
	}
	nextAssignmentID, err := s.ids.NewID()
	if err != nil {
		return Request{}, err
	}
	if err := repository.DecideRequest(
		ctx, actorID, requestID, approve, classification, nextAssignmentID, s.clock.Now().UTC(),
	); err != nil {
		return Request{}, err
	}
	return s.Request(ctx, actorID, requestID)
}

func (s *Service) CancelRequest(ctx context.Context, ownerID identity.UserID, requestID string) (Request, error) {
	if err := s.ready(); err != nil {
		return Request{}, err
	}
	repository, ok := s.repository.(DecisionRepository)
	if !ok {
		return Request{}, ErrUnavailable
	}
	if err := repository.CancelRequest(ctx, ownerID, requestID, s.clock.Now().UTC()); err != nil {
		return Request{}, err
	}
	return s.Request(ctx, ownerID, requestID)
}
