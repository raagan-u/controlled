package paymentrequest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/paymentrequest"
)

func TestCreatePersistsPendingRequest(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	repo := &requestRepository{}
	service := paymentrequest.NewService(requestIDs{}, requestClock{now: now}, repo)
	created, err := service.Create(context.Background(), "request-key", "requester", "recipient", 1250, "usd", " Dinner ")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID != "request-id" || created.Status != "PENDING" || created.AmountMinor != 1250 || created.Currency != "USD" || created.Note != "Dinner" || !created.CreatedAt.Equal(now) {
		t.Fatalf("created = %#v", created)
	}
}

func TestCreateRejectsSelfRequest(t *testing.T) {
	t.Parallel()
	service := paymentrequest.NewService(requestIDs{}, requestClock{now: time.Now()}, &requestRepository{})
	_, err := service.Create(context.Background(), "request-key", "same", "same", 100, "USD", "")
	if !errors.Is(err, paymentrequest.ErrInvalidRequest) {
		t.Fatalf("Create() error = %v", err)
	}
}

type requestIDs struct{}

func (requestIDs) NewID() (string, error) { return "request-id", nil }

type requestClock struct{ now time.Time }

func (clock requestClock) Now() time.Time { return clock.now }

type requestRepository struct{}

func (*requestRepository) Create(_ context.Context, request paymentrequest.Request) (paymentrequest.Request, error) {
	return request, nil
}
