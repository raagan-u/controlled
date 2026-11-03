package accountability

import (
	"context"
	"strings"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/money"
	"github.com/raagan-u/controlled/internal/walletops"
)

const (
	assignmentLifetime = 6 * time.Hour
	requestLifetime    = 24 * time.Hour
)

type RequestPact struct {
	PactID        string
	Goal          string
	Intent        string
	LimitMinor    int64
	UsedMinor     int64
	ProposedMinor int64
	OverageMinor  int64
}

type RequestReviewer struct {
	PartnerID   identity.UserID
	Position    int
	Status      string
	Decision    string
	AssignedAt  time.Time
	ExpiresAt   time.Time
	RespondedAt time.Time
}

type Request struct {
	ID                  string
	IntentID            walletops.OperationID
	OwnerID             identity.UserID
	OperationType       walletops.DebitKind
	IdempotencyKey      string
	Destination         string
	AmountMinor         int64
	Currency            string
	Reason              string
	Status              string
	Classification      string
	AssignedPartnerID   identity.UserID
	Pacts               []RequestPact
	Reviewers           []RequestReviewer
	CreatedAt           time.Time
	ExpiresAt           time.Time
	ResolvedAt          time.Time
	ExecutedOperationID walletops.OperationID
	Duplicate           bool
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

type ExecutionRepository interface {
	MarkIntentExecuted(context.Context, string, walletops.OperationID, time.Time) error
	MarkIntentFailed(context.Context, string, time.Time) error
}

type MaintenanceResult struct {
	TimedOut  int
	Expired   int
	Recovered int
}

type MaintenanceRepository interface {
	MaintainRequests(context.Context, time.Time) (MaintenanceResult, error)
}

type ExecutionRecoveryRepository interface {
	ApprovedDebits(context.Context) ([]walletops.ApprovedDebit, error)
}

func (s *Service) RunMaintenance(ctx context.Context) (MaintenanceResult, error) {
	if err := s.ready(); err != nil {
		return MaintenanceResult{}, err
	}
	repository, ok := s.repository.(MaintenanceRepository)
	if !ok {
		return MaintenanceResult{}, ErrUnavailable
	}
	result, err := repository.MaintainRequests(ctx, s.clock.Now().UTC())
	if err != nil || s.executor == nil {
		return result, err
	}
	recovery, ok := s.repository.(ExecutionRecoveryRepository)
	if !ok {
		return result, ErrUnavailable
	}
	approved, err := recovery.ApprovedDebits(ctx)
	if err != nil {
		return result, err
	}
	executionRepository, ok := s.repository.(ExecutionRepository)
	if !ok {
		return result, ErrUnavailable
	}
	for _, debit := range approved {
		executedID, executeErr := s.executor.ExecuteApproved(ctx, debit)
		if executeErr != nil {
			if markErr := executionRepository.MarkIntentFailed(ctx, debit.RequestID, s.clock.Now().UTC()); markErr != nil {
				return result, markErr
			}
			continue
		}
		if err := executionRepository.MarkIntentExecuted(ctx, debit.RequestID, executedID, s.clock.Now().UTC()); err != nil {
			return result, err
		}
		result.Recovered++
	}
	return result, nil
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
	item, err := s.Request(ctx, actorID, requestID)
	if err != nil || !approve {
		return item, err
	}
	if s.executor == nil {
		return Request{}, ErrUnavailable
	}
	pactIDs := make([]string, len(item.Pacts))
	for index, pact := range item.Pacts {
		pactIDs[index] = pact.PactID
	}
	amount, err := money.New(item.AmountMinor, item.Currency)
	if err != nil {
		return Request{}, err
	}
	executedID, executionErr := s.executor.ExecuteApproved(ctx, walletops.ApprovedDebit{
		RequestID: item.ID, IntentID: item.IntentID, IdempotencyKey: item.IdempotencyKey,
		OwnerID: item.OwnerID, Kind: item.OperationType, Destination: item.Destination,
		Amount: amount, Classification: item.Classification, Reason: item.Reason, PactIDs: pactIDs,
	})
	executionRepository, ok := s.repository.(ExecutionRepository)
	if !ok {
		return Request{}, ErrUnavailable
	}
	if executionErr != nil {
		_ = executionRepository.MarkIntentFailed(ctx, item.ID, s.clock.Now().UTC())
		return Request{}, executionErr
	}
	if err := executionRepository.MarkIntentExecuted(ctx, item.ID, executedID, s.clock.Now().UTC()); err != nil {
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
