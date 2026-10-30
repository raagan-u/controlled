package paymentrequest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/paymentrequest"
	"github.com/raagan-u/controlled/internal/walletops"
)

func TestCreatePersistsExpiringPendingRequest(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	repo := &requestRepository{}
	service := paymentrequest.NewService(requestIDs{}, requestClock{now}, repo, transferer{})
	created, err := service.Create(context.Background(), "key", "requester", "recipient", 1250, "usd", " Dinner ")
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != "PENDING" || created.Note != "Dinner" || !created.ExpiresAt.Equal(now.Add(7*24*time.Hour)) {
		t.Fatalf("created=%#v", created)
	}
}

func TestPayCompletesImmediateTransfer(t *testing.T) {
	now := time.Now().UTC()
	repo := &requestRepository{item: paymentrequest.Request{ID: "request-id", RequesterID: "requester", RecipientID: "recipient", Requester: paymentrequest.Party{Email: "requester@example.com", DisplayName: "Requester"}, AmountMinor: 500, Currency: "USD", Status: "PENDING"}}
	mover := transferer{attempt: walletops.DebitAttempt{Transfer: &walletops.Transfer{ID: "transfer-id"}}}
	service := paymentrequest.NewService(requestIDs{}, requestClock{now}, repo, mover)
	result, err := service.Pay(context.Background(), "pay-key", "recipient", "request-id")
	if err != nil {
		t.Fatal(err)
	}
	if repo.completedTransfer != "transfer-id" || result.Request.Status != "PAID" {
		t.Fatalf("result=%#v transfer=%s", result, repo.completedTransfer)
	}
}

func TestPayWaitsForAccountabilityApproval(t *testing.T) {
	now := time.Now().UTC()
	repo := &requestRepository{item: paymentrequest.Request{ID: "request-id", RequesterID: "requester", RecipientID: "recipient", Requester: paymentrequest.Party{Email: "requester@example.com", DisplayName: "Requester"}, AmountMinor: 500, Currency: "USD", Status: "PENDING"}}
	review := &walletops.ReviewRequest{ID: "review-id", Status: "ASSIGNED"}
	service := paymentrequest.NewService(requestIDs{}, requestClock{now}, repo, transferer{attempt: walletops.DebitAttempt{Request: review}})
	result, err := service.Pay(context.Background(), "pay-key", "recipient", "request-id")
	if err != nil {
		t.Fatal(err)
	}
	if result.Review == nil || result.Review.ID != "review-id" || result.Request.Status != "AWAITING_APPROVAL" {
		t.Fatalf("result=%#v", result)
	}
}

func TestFailedPaymentReturnsRequestToPending(t *testing.T) {
	repo := &requestRepository{item: paymentrequest.Request{ID: "request-id", Requester: paymentrequest.Party{Email: "requester@example.com"}, AmountMinor: 500, Currency: "USD", Status: "PENDING"}}
	service := paymentrequest.NewService(requestIDs{}, requestClock{time.Now()}, repo, transferer{err: errors.New("insufficient")})
	if _, err := service.Pay(context.Background(), "pay-key", "recipient", "request-id"); err == nil {
		t.Fatal("Pay() error = nil")
	}
	if repo.item.Status != "PENDING" {
		t.Fatalf("status=%s", repo.item.Status)
	}
}

func TestCreateRejectsSelfRequest(t *testing.T) {
	service := paymentrequest.NewService(requestIDs{}, requestClock{time.Now()}, &requestRepository{}, transferer{})
	_, err := service.Create(context.Background(), "key", "same", "same", 100, "USD", "")
	if !errors.Is(err, paymentrequest.ErrInvalidRequest) {
		t.Fatalf("err=%v", err)
	}
}

type requestIDs struct{}

func (requestIDs) NewID() (string, error) { return "request-id", nil }

type requestClock struct{ now time.Time }

func (c requestClock) Now() time.Time { return c.now }

type transferer struct {
	attempt walletops.DebitAttempt
	err     error
}

func (t transferer) TransferWithReason(context.Context, string, identity.UserID, string, int64, string, string) (walletops.DebitAttempt, error) {
	return t.attempt, t.err
}

type requestRepository struct {
	item              paymentrequest.Request
	completedTransfer string
}

func (r *requestRepository) Create(_ context.Context, item paymentrequest.Request) (paymentrequest.Request, error) {
	r.item = item
	return item, nil
}
func (r *requestRepository) ForUser(context.Context, identity.UserID) ([]paymentrequest.Request, error) {
	return []paymentrequest.Request{r.item}, nil
}
func (r *requestRepository) ByID(context.Context, identity.UserID, string) (paymentrequest.Request, error) {
	return r.item, nil
}
func (r *requestRepository) PreparePay(_ context.Context, _ identity.UserID, _ string, key string, _ time.Time) (paymentrequest.Request, error) {
	r.item.PaymentKey = key
	r.item.Status = "PROCESSING"
	return r.item, nil
}
func (r *requestRepository) CompletePay(_ context.Context, _ string, transfer string, _ time.Time) error {
	r.completedTransfer = transfer
	r.item.Status = "PAID"
	return nil
}
func (r *requestRepository) AwaitApproval(context.Context, string, string, time.Time) error {
	r.item.Status = "AWAITING_APPROVAL"
	return nil
}
func (r *requestRepository) ReleasePay(context.Context, string, time.Time) error {
	r.item.Status = "PENDING"
	return nil
}
func (r *requestRepository) Resolve(_ context.Context, _ identity.UserID, _ string, status string, _ time.Time) error {
	r.item.Status = status
	return nil
}
func (*requestRepository) Maintain(context.Context, time.Time) error { return nil }
