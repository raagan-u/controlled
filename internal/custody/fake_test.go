package custody_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/raagan-u/pay-pact/internal/custody"
	"github.com/raagan-u/pay-pact/internal/money"
)

func TestFakeFundingLifecycleIsIdempotent(t *testing.T) {
	t.Parallel()

	provider := custody.NewFake(&sequenceIDs{})
	amount, _ := money.New(5000, "USD")
	request := custody.CreateFundingOrder{IdempotencyKey: "funding-1", Amount: amount}
	first, err := provider.CreateFundingOrder(t.Context(), request)
	if err != nil {
		t.Fatalf("CreateFundingOrder() error = %v", err)
	}
	retry, err := provider.CreateFundingOrder(t.Context(), request)
	if err != nil {
		t.Fatalf("retry error = %v", err)
	}
	if retry.ID != first.ID || retry.Status != custody.FundingAwaitingApproval {
		t.Fatalf("retry = %+v", retry)
	}
	captured, err := provider.CaptureFundingOrder(t.Context(), first.ID)
	if err != nil {
		t.Fatalf("CaptureFundingOrder() error = %v", err)
	}
	if captured.Status != custody.FundingCaptured {
		t.Fatalf("status = %s", captured.Status)
	}
}

func TestFakePayoutLifecycle(t *testing.T) {
	t.Parallel()

	provider := custody.NewFake(&sequenceIDs{})
	amount, _ := money.New(2500, "USD")
	payout, err := provider.CreatePayout(t.Context(), custody.CreatePayout{
		IdempotencyKey: "payout-1",
		Amount:         amount,
		Destination:    "wallet@example.com",
	})
	if err != nil {
		t.Fatalf("CreatePayout() error = %v", err)
	}
	settled, err := provider.SettlePayout(t.Context(), payout.ID, true)
	if err != nil {
		t.Fatalf("SettlePayout() error = %v", err)
	}
	if settled.Status != custody.PayoutSucceeded {
		t.Fatalf("status = %s", settled.Status)
	}
	if _, err := provider.SettlePayout(t.Context(), payout.ID, false); !errors.Is(err, custody.ErrPayoutSettled) {
		t.Fatalf("second settlement error = %v, want %v", err, custody.ErrPayoutSettled)
	}
}

type sequenceIDs struct{ next int }

func (s *sequenceIDs) NewID() (string, error) {
	s.next++
	return "fake-" + strconv.Itoa(s.next), nil
}
