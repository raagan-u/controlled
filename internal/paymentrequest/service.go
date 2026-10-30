package paymentrequest

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/walletops"
)

const Lifetime = 7 * 24 * time.Hour

var (
	ErrInvalidRequest      = errors.New("payment request is invalid")
	ErrIdempotencyConflict = errors.New("idempotency key conflicts with another payment request")
	ErrNotFound            = errors.New("payment request was not found")
	ErrForbidden           = errors.New("payment request action is forbidden")
	ErrInvalidState        = errors.New("payment request cannot transition from its current state")
	ErrUnavailable         = errors.New("payment requests are unavailable")
)

type IDGenerator interface{ NewID() (string, error) }
type Clock interface{ Now() time.Time }

type Party struct {
	ID, CtrldID, Email, DisplayName string
}

type Request struct {
	ID                      string
	RequesterID             identity.UserID
	RecipientID             identity.UserID
	Requester               Party
	Recipient               Party
	AmountMinor             int64
	Currency                string
	Note                    string
	Status                  string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	ExpiresAt               time.Time
	ResolvedAt              time.Time
	TransferID              string
	AccountabilityRequestID string
	Duplicate               bool
	IdempotencyKey          string
	PaymentKey              string
}

type Repository interface {
	Create(context.Context, Request) (Request, error)
	ForUser(context.Context, identity.UserID) ([]Request, error)
	ByID(context.Context, identity.UserID, string) (Request, error)
	PreparePay(context.Context, identity.UserID, string, string, time.Time) (Request, error)
	CompletePay(context.Context, string, string, time.Time) error
	AwaitApproval(context.Context, string, string, time.Time) error
	ReleasePay(context.Context, string, time.Time) error
	Resolve(context.Context, identity.UserID, string, string, time.Time) error
	Maintain(context.Context, time.Time) error
}

type Transferer interface {
	TransferWithReason(context.Context, string, identity.UserID, string, int64, string, string) (walletops.DebitAttempt, error)
}

type Service struct {
	ids       IDGenerator
	clock     Clock
	repo      Repository
	transfers Transferer
}

func NewService(ids IDGenerator, clock Clock, repo Repository, transfers Transferer) *Service {
	return &Service{ids: ids, clock: clock, repo: repo, transfers: transfers}
}

func (s *Service) Create(ctx context.Context, key string, requester, recipient identity.UserID, amount int64, currency, note string) (Request, error) {
	key, currency, note = strings.TrimSpace(key), strings.ToUpper(strings.TrimSpace(currency)), strings.TrimSpace(note)
	if !s.available() {
		return Request{}, ErrUnavailable
	}
	if key == "" || requester == "" || recipient == "" || requester == recipient || amount <= 0 || currency != "USD" || len(note) > 80 {
		return Request{}, ErrInvalidRequest
	}
	id, err := s.ids.NewID()
	if err != nil {
		return Request{}, err
	}
	now := s.clock.Now().UTC()
	return s.repo.Create(ctx, Request{ID: id, RequesterID: requester, RecipientID: recipient, AmountMinor: amount, Currency: currency, Note: note, Status: "PENDING", CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(Lifetime), IdempotencyKey: key})
}

func (s *Service) Requests(ctx context.Context, user identity.UserID) ([]Request, error) {
	if !s.available() || user == "" {
		return nil, ErrUnavailable
	}
	if err := s.repo.Maintain(ctx, s.clock.Now().UTC()); err != nil {
		return nil, err
	}
	return s.repo.ForUser(ctx, user)
}

func (s *Service) Request(ctx context.Context, user identity.UserID, id string) (Request, error) {
	if !s.available() || user == "" || strings.TrimSpace(id) == "" {
		return Request{}, ErrInvalidRequest
	}
	if err := s.repo.Maintain(ctx, s.clock.Now().UTC()); err != nil {
		return Request{}, err
	}
	return s.repo.ByID(ctx, user, id)
}

type PayResult struct {
	Request Request
	Review  *walletops.ReviewRequest
}

func (s *Service) Pay(ctx context.Context, key string, actor identity.UserID, id string) (PayResult, error) {
	key = strings.TrimSpace(key)
	if !s.available() || s.transfers == nil {
		return PayResult{}, ErrUnavailable
	}
	if key == "" || actor == "" || strings.TrimSpace(id) == "" {
		return PayResult{}, ErrInvalidRequest
	}
	now := s.clock.Now().UTC()
	if err := s.repo.Maintain(ctx, now); err != nil {
		return PayResult{}, err
	}
	paymentKey := "payment-request:" + id + ":" + key
	item, err := s.repo.PreparePay(ctx, actor, id, paymentKey, now)
	if err != nil {
		return PayResult{}, err
	}
	reason := item.Note
	if reason == "" {
		reason = "Payment request from " + item.Requester.DisplayName
	}
	attempt, err := s.transfers.TransferWithReason(ctx, paymentKey, actor, item.Requester.Email, item.AmountMinor, item.Currency, reason)
	if err != nil {
		_ = s.repo.ReleasePay(ctx, id, s.clock.Now().UTC())
		return PayResult{}, err
	}
	if attempt.Transfer != nil {
		if err := s.repo.CompletePay(ctx, id, string(attempt.Transfer.ID), s.clock.Now().UTC()); err != nil {
			return PayResult{}, err
		}
	} else if attempt.Request != nil {
		if err := s.repo.AwaitApproval(ctx, id, attempt.Request.ID, s.clock.Now().UTC()); err != nil {
			return PayResult{}, err
		}
	} else {
		_ = s.repo.ReleasePay(ctx, id, s.clock.Now().UTC())
		return PayResult{}, ErrInvalidState
	}
	updated, err := s.repo.ByID(ctx, actor, id)
	return PayResult{Request: updated, Review: attempt.Request}, err
}

func (s *Service) Decline(ctx context.Context, actor identity.UserID, id string) (Request, error) {
	return s.resolve(ctx, actor, id, "DECLINED")
}
func (s *Service) Cancel(ctx context.Context, actor identity.UserID, id string) (Request, error) {
	return s.resolve(ctx, actor, id, "CANCELLED")
}

func (s *Service) resolve(ctx context.Context, actor identity.UserID, id, status string) (Request, error) {
	if !s.available() || actor == "" || strings.TrimSpace(id) == "" {
		return Request{}, ErrInvalidRequest
	}
	now := s.clock.Now().UTC()
	if err := s.repo.Maintain(ctx, now); err != nil {
		return Request{}, err
	}
	if err := s.repo.Resolve(ctx, actor, id, status, now); err != nil {
		return Request{}, err
	}
	return s.repo.ByID(ctx, actor, id)
}

func (s *Service) Maintain(ctx context.Context) error {
	if !s.available() {
		return ErrUnavailable
	}
	return s.repo.Maintain(ctx, s.clock.Now().UTC())
}
func (s *Service) available() bool {
	return s != nil && s.ids != nil && s.clock != nil && s.repo != nil
}
