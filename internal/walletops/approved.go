package walletops

import (
	"context"
	"strings"

	"github.com/raagan-u/controlled/internal/custody"
	"github.com/raagan-u/controlled/internal/ledger"
)

type approvalContextKey struct{}

type debitApproval struct {
	ApprovedDebit
	ResolutionJournalIDs map[string]ledger.JournalID
}

func approvalFromContext(ctx context.Context) *debitApproval {
	value, _ := ctx.Value(approvalContextKey{}).(*debitApproval)
	return value
}

func (s *Service) ExecuteApproved(ctx context.Context, approved ApprovedDebit) (OperationID, error) {
	if err := s.available(); err != nil {
		return "", err
	}
	approved.Classification = strings.ToUpper(strings.TrimSpace(approved.Classification))
	if approved.RequestID == "" || approved.IntentID == "" || approved.IdempotencyKey == "" || approved.OwnerID == "" ||
		approved.Destination == "" || !approved.Amount.Positive() || len(approved.PactIDs) == 0 ||
		(approved.Classification != "EMERGENCY" && approved.Classification != "CONSCIOUS_BREAK") {
		return "", ErrInvalidOperation
	}
	generated, err := generateIDs(s.ids, 1+len(approved.PactIDs))
	if err != nil {
		return "", err
	}
	approval := &debitApproval{
		ApprovedDebit:        approved,
		ResolutionJournalIDs: make(map[string]ledger.JournalID, len(approved.PactIDs)),
	}
	for index, pactID := range approved.PactIDs {
		approval.ResolutionJournalIDs[pactID] = ledger.JournalID(generated[index+1])
	}
	approvedContext := context.WithValue(ctx, approvalContextKey{}, approval)
	now := s.clock.Now().UTC()
	switch approved.Kind {
	case DebitInternalTransfer:
		transfer, err := s.repo.CreateTransfer(approvedContext, approved.IdempotencyKey, Transfer{
			ID: approved.IntentID, SenderID: approved.OwnerID, RecipientEmail: normalizedEmail(approved.Destination),
			Amount: approved.Amount, CreatedAt: now,
		}, ledger.JournalID(generated[0]))
		if err != nil {
			return "", err
		}
		s.notifyMoneyReceived(ctx, transfer)
		return transfer.ID, nil
	case DebitWithdrawal:
		withdrawal, err := s.repo.ReserveWithdrawal(approvedContext, approved.IdempotencyKey, Withdrawal{
			ID: approved.IntentID, OwnerID: approved.OwnerID, Destination: approved.Destination,
			Amount: approved.Amount, Status: WithdrawalReserved, CreatedAt: now, UpdatedAt: now,
		}, ledger.JournalID(generated[0]))
		if err != nil {
			return "", err
		}
		if withdrawal.Status == WithdrawalReserved {
			payout, err := s.custody.CreatePayout(ctx, custody.CreatePayout{
				IdempotencyKey: "withdrawal:" + string(withdrawal.ID), Amount: withdrawal.Amount,
				Destination: withdrawal.Destination,
			})
			if err != nil {
				return "", err
			}
			if _, err := s.repo.AttachPayout(ctx, approved.OwnerID, withdrawal.ID, payout, s.clock.Now().UTC()); err != nil {
				return "", err
			}
		}
		return withdrawal.ID, nil
	default:
		return "", ErrInvalidOperation
	}
}
