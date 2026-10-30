package paymentrequest

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
)

var (
	ErrInvalidRequest      = errors.New("payment request is invalid")
	ErrIdempotencyConflict = errors.New("idempotency key conflicts with another payment request")
	ErrUnavailable         = errors.New("payment requests are unavailable")
)

type IDGenerator interface{ NewID() (string, error) }
type Clock interface{ Now() time.Time }

type Request struct {
	ID             string
	RequesterID    identity.UserID
	RecipientID    identity.UserID
	AmountMinor    int64
	Currency       string
	Note           string
	Status         string
	CreatedAt      time.Time
	Duplicate      bool
	IdempotencyKey string
}

type Repository interface {
	Create(context.Context, Request) (Request, error)
}

type Service struct {
	ids   IDGenerator
	clock Clock
	repo  Repository
}

func NewService(ids IDGenerator, clock Clock, repo Repository) *Service {
	return &Service{ids: ids, clock: clock, repo: repo}
}

func (s *Service) Create(ctx context.Context, key string, requester, recipient identity.UserID, amount int64, currency, note string) (Request, error) {
	key = strings.TrimSpace(key)
	currency = strings.ToUpper(strings.TrimSpace(currency))
	note = strings.TrimSpace(note)
	if s == nil || s.ids == nil || s.clock == nil || s.repo == nil {
		return Request{}, ErrUnavailable
	}
	if key == "" || requester == "" || recipient == "" || requester == recipient || amount <= 0 || currency != "USD" || len(note) > 80 {
		return Request{}, ErrInvalidRequest
	}
	id, err := s.ids.NewID()
	if err != nil {
		return Request{}, err
	}
	return s.repo.Create(ctx, Request{ID: id, RequesterID: requester, RecipientID: recipient, AmountMinor: amount, Currency: currency, Note: note, Status: "PENDING", CreatedAt: s.clock.Now(), IdempotencyKey: key})
}
