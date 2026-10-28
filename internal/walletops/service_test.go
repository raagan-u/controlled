package walletops_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/custody"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/walletops"
)

func TestFundingCreditsOnlyAfterConfirmation(t *testing.T) {
	t.Parallel()

	ids := &sequenceIDs{}
	repo := &memoryRepository{}
	provider := custody.NewFake(ids)
	service := walletops.NewService(ids, fixedClock{}, repo, provider, walletops.AllowAllPolicy{})

	created, err := service.CreateFunding(t.Context(), "fund-1", "user-1", 5000, "USD")
	if err != nil {
		t.Fatalf("CreateFunding() error = %v", err)
	}
	if created.Status != walletops.FundingAwaitingConfirmation || repo.fundingCompleted {
		t.Fatalf("created = %+v, completed = %v", created, repo.fundingCompleted)
	}
	completed, err := service.ConfirmFunding(t.Context(), "user-1", created.ID)
	if err != nil {
		t.Fatalf("ConfirmFunding() error = %v", err)
	}
	if completed.Status != walletops.FundingCompleted || !repo.fundingCompleted {
		t.Fatalf("completed = %+v, repository completed = %v", completed, repo.fundingCompleted)
	}
}

func TestOutboundOperationsPassThroughPolicy(t *testing.T) {
	t.Parallel()

	ids := &sequenceIDs{}
	repo := &memoryRepository{}
	provider := custody.NewFake(ids)
	policy := &recordingPolicy{}
	service := walletops.NewService(ids, fixedClock{}, repo, provider, policy)

	if _, err := service.Transfer(t.Context(), "send-1", "user-1", "friend@example.com", 1200, "USD"); err != nil {
		t.Fatalf("Transfer() error = %v", err)
	}
	withdrawal, err := service.CreateWithdrawal(t.Context(), "withdraw-1", "user-1", "paypal@example.com", 800, "USD")
	if err != nil {
		t.Fatalf("CreateWithdrawal() error = %v", err)
	}
	if len(policy.requests) != 2 || policy.requests[0].Kind != walletops.DebitInternalTransfer ||
		policy.requests[1].Kind != walletops.DebitWithdrawal {
		t.Fatalf("policy requests = %+v", policy.requests)
	}
	if withdrawal.Status != walletops.WithdrawalPending || !repo.withdrawalReserved {
		t.Fatalf("withdrawal = %+v", withdrawal)
	}
	settled, err := service.SimulateWithdrawalSettlement(t.Context(), "user-1", withdrawal.ID, true)
	if err != nil {
		t.Fatalf("SimulateWithdrawalSettlement() error = %v", err)
	}
	if settled.Status != walletops.WithdrawalCompleted || !repo.withdrawalSettled {
		t.Fatalf("settled = %+v", settled)
	}
}

type sequenceIDs struct{ next int }

func (s *sequenceIDs) NewID() (string, error) {
	s.next++
	return "id-" + strconv.Itoa(s.next), nil
}

type fixedClock struct{}

func (fixedClock) Now() time.Time {
	return time.Date(2026, time.October, 23, 10, 0, 0, 0, time.UTC)
}

type recordingPolicy struct {
	requests []walletops.DebitRequest
}

func (p *recordingPolicy) Authorize(_ context.Context, request walletops.DebitRequest) error {
	p.requests = append(p.requests, request)
	return nil
}

type memoryRepository struct {
	funding            walletops.FundingIntent
	fundingCompleted   bool
	withdrawal         walletops.Withdrawal
	withdrawalReserved bool
	withdrawalSettled  bool
}

func (r *memoryRepository) CreateFunding(_ context.Context, _ string, value walletops.FundingIntent) (walletops.FundingIntent, error) {
	value.WalletID = "wallet-1"
	r.funding = value
	return value, nil
}

func (r *memoryRepository) AttachFundingOrder(_ context.Context, _ identity.UserID, _ walletops.OperationID, order custody.FundingOrder, now time.Time) (walletops.FundingIntent, error) {
	r.funding.Status = walletops.FundingAwaitingConfirmation
	r.funding.ProviderOrderID = order.ID
	r.funding.ApprovalURL = order.ApprovalURL
	r.funding.UpdatedAt = now
	return r.funding, nil
}

func (r *memoryRepository) Funding(context.Context, identity.UserID, walletops.OperationID) (walletops.FundingIntent, error) {
	return r.funding, nil
}

func (r *memoryRepository) CompleteFunding(_ context.Context, _ identity.UserID, _ walletops.OperationID, _ string, journalID ledger.JournalID, _ ledger.AccountID, now time.Time) (walletops.FundingIntent, error) {
	r.fundingCompleted = true
	r.funding.Status = walletops.FundingCompleted
	r.funding.LedgerTransactionID = journalID
	r.funding.CompletedAt = now
	return r.funding, nil
}

func (r *memoryRepository) CreateTransfer(_ context.Context, _ string, value walletops.Transfer, journalID ledger.JournalID) (walletops.Transfer, error) {
	value.RecipientID = "user-2"
	value.LedgerTransactionID = journalID
	return value, nil
}

func (r *memoryRepository) ReserveWithdrawal(_ context.Context, _ string, value walletops.Withdrawal, journalID ledger.JournalID) (walletops.Withdrawal, error) {
	r.withdrawalReserved = true
	value.WalletID = "wallet-1"
	value.Status = walletops.WithdrawalReserved
	value.ReservationTransactionID = journalID
	r.withdrawal = value
	return value, nil
}

func (r *memoryRepository) AttachPayout(_ context.Context, _ identity.UserID, _ walletops.OperationID, payout custody.Payout, now time.Time) (walletops.Withdrawal, error) {
	r.withdrawal.ProviderPayoutID = payout.ID
	r.withdrawal.Status = walletops.WithdrawalPending
	r.withdrawal.UpdatedAt = now
	return r.withdrawal, nil
}

func (r *memoryRepository) Withdrawal(context.Context, identity.UserID, walletops.OperationID) (walletops.Withdrawal, error) {
	return r.withdrawal, nil
}

func (r *memoryRepository) SettleWithdrawal(_ context.Context, _ identity.UserID, _ walletops.OperationID, succeeded bool, journalID ledger.JournalID, _ ledger.AccountID, now time.Time) (walletops.Withdrawal, error) {
	r.withdrawalSettled = true
	if succeeded {
		r.withdrawal.Status = walletops.WithdrawalCompleted
	} else {
		r.withdrawal.Status = walletops.WithdrawalFailed
	}
	r.withdrawal.SettlementTransactionID = journalID
	r.withdrawal.CompletedAt = now
	return r.withdrawal, nil
}
