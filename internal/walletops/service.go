package walletops

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/raagan-u/controlled/internal/custody"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/money"
	"github.com/raagan-u/controlled/internal/notification"
)

type IDGenerator interface {
	NewID() (string, error)
}

type Clock interface {
	Now() time.Time
}

type UserDirectory interface {
	UserByID(context.Context, identity.UserID) (identity.User, error)
}

type Repository interface {
	CreateFunding(context.Context, string, FundingIntent) (FundingIntent, error)
	AttachFundingOrder(context.Context, identity.UserID, OperationID, custody.FundingOrder, time.Time) (FundingIntent, error)
	Funding(context.Context, identity.UserID, OperationID) (FundingIntent, error)
	CompleteFunding(context.Context, identity.UserID, OperationID, string, money.Money, ledger.JournalID, ledger.AccountID, time.Time) (FundingIntent, error)
	CreateTransfer(context.Context, string, Transfer, ledger.JournalID) (Transfer, error)
	ReserveWithdrawal(context.Context, string, Withdrawal, ledger.JournalID) (Withdrawal, error)
	AttachPayout(context.Context, identity.UserID, OperationID, custody.Payout, time.Time) (Withdrawal, error)
	Withdrawal(context.Context, identity.UserID, OperationID) (Withdrawal, error)
	PendingWithdrawals(context.Context, int) ([]Withdrawal, error)
	SettleWithdrawal(context.Context, identity.UserID, OperationID, bool, ledger.JournalID, ledger.AccountID, time.Time) (Withdrawal, error)
}

type Service struct {
	ids      IDGenerator
	clock    Clock
	repo     Repository
	custody  custody.Provider
	policy   Policy
	requests RequestCoordinator
	notifier notification.Notifier
	users    UserDirectory
}

type ReconciliationResult struct {
	Completed int
	Failed    int
	Pending   int
}

func (s *Service) EnableAccountability(requests RequestCoordinator) *Service {
	if s != nil {
		s.requests = requests
	}
	return s
}

func (s *Service) EnableNotifications(notifier notification.Notifier) *Service {
	if s != nil {
		s.notifier = notifier
	}
	return s
}

func (s *Service) EnableUserDirectory(users UserDirectory) *Service {
	if s != nil {
		s.users = users
	}
	return s
}

func NewService(
	ids IDGenerator,
	clock Clock,
	repo Repository,
	provider custody.Provider,
	policy Policy,
) *Service {
	return &Service{ids: ids, clock: clock, repo: repo, custody: provider, policy: policy}
}

func (s *Service) CreateFunding(
	ctx context.Context,
	key string,
	ownerID identity.UserID,
	amountMinor int64,
	currency string,
) (FundingIntent, error) {
	if err := s.available(); err != nil {
		return FundingIntent{}, err
	}
	key, err := normalizeKey(key)
	if err != nil {
		return FundingIntent{}, err
	}
	amount, err := money.New(amountMinor, strings.ToUpper(strings.TrimSpace(currency)))
	if err != nil || !amount.Positive() || ownerID == "" {
		return FundingIntent{}, ErrInvalidOperation
	}
	id, err := s.ids.NewID()
	if err != nil {
		return FundingIntent{}, err
	}
	now := s.clock.Now().UTC()
	intent, err := s.repo.CreateFunding(ctx, key, FundingIntent{
		ID: OperationID(id), OwnerID: ownerID, Amount: amount, Status: FundingCreated,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return FundingIntent{}, err
	}
	if intent.Status != FundingCreated {
		return intent, nil
	}
	order, err := s.custody.CreateFundingOrder(ctx, custody.CreateFundingOrder{
		IdempotencyKey: "funding:" + string(intent.ID),
		Amount:         intent.Amount,
	})
	if err != nil {
		return FundingIntent{}, err
	}
	attached, err := s.repo.AttachFundingOrder(ctx, ownerID, intent.ID, order, s.clock.Now().UTC())
	if err != nil {
		return FundingIntent{}, err
	}
	attached.Duplicate = intent.Duplicate
	return attached, nil
}

func (s *Service) ConfirmFunding(
	ctx context.Context,
	ownerID identity.UserID,
	id OperationID,
) (FundingIntent, error) {
	if err := s.available(); err != nil {
		return FundingIntent{}, err
	}
	intent, err := s.repo.Funding(ctx, ownerID, id)
	if err != nil {
		return FundingIntent{}, err
	}
	if intent.Status == FundingCompleted {
		intent.Duplicate = true
		return intent, nil
	}
	if intent.Status != FundingAwaitingConfirmation || intent.ProviderOrderID == "" {
		return FundingIntent{}, ErrInvalidState
	}
	order, err := s.custody.CaptureFundingOrder(ctx, intent.ProviderOrderID)
	if err != nil {
		return FundingIntent{}, err
	}
	if order.Status != custody.FundingCaptured || !order.Amount.Equal(intent.Amount) ||
		!order.NetAmount.Positive() || order.NetAmount.Currency() != intent.Amount.Currency() ||
		order.NetAmount.AmountMinor() > intent.Amount.AmountMinor() {
		return FundingIntent{}, ErrInvalidState
	}
	generated, err := generateIDs(s.ids, 2)
	if err != nil {
		return FundingIntent{}, err
	}
	return s.repo.CompleteFunding(
		ctx,
		ownerID,
		id,
		order.ID,
		order.NetAmount,
		ledger.JournalID(generated[0]),
		ledger.AccountID(generated[1]),
		s.clock.Now().UTC(),
	)
}

func (s *Service) Transfer(
	ctx context.Context,
	key string,
	senderID identity.UserID,
	recipientEmail string,
	amountMinor int64,
	currency string,
) (Transfer, error) {
	attempt, err := s.TransferWithReason(ctx, key, senderID, recipientEmail, amountMinor, currency, "")
	if err != nil {
		return Transfer{}, err
	}
	if attempt.Transfer == nil {
		return Transfer{}, ErrPolicyDenied
	}
	return *attempt.Transfer, nil
}

func (s *Service) TransferWithReason(
	ctx context.Context,
	key string,
	senderID identity.UserID,
	recipientEmail string,
	amountMinor int64,
	currency string,
	reason string,
) (DebitAttempt, error) {
	if err := s.available(); err != nil {
		return DebitAttempt{}, err
	}
	key, err := normalizeKey(key)
	if err != nil {
		return DebitAttempt{}, err
	}
	recipientEmail = normalizedEmail(recipientEmail)
	amount, err := money.New(amountMinor, strings.ToUpper(strings.TrimSpace(currency)))
	if err != nil || !amount.Positive() || senderID == "" || recipientEmail == "" {
		return DebitAttempt{}, ErrInvalidOperation
	}
	if err := s.policy.Authorize(ctx, DebitRequest{
		OwnerID: senderID, Kind: DebitInternalTransfer,
		ReferenceKey: "internal-transfer:" + string(senderID) + ":" + key,
		Amount:       amount,
	}); err != nil {
		return DebitAttempt{}, err
	}
	generated, err := generateIDs(s.ids, 2)
	if err != nil {
		return DebitAttempt{}, err
	}
	now := s.clock.Now().UTC()
	transfer, err := s.repo.CreateTransfer(ctx, key, Transfer{
		ID: OperationID(generated[0]), SenderID: senderID,
		RecipientEmail: recipientEmail, Amount: amount, Remark: strings.TrimSpace(reason), CreatedAt: now,
	}, ledger.JournalID(generated[1]))
	if err == nil {
		s.notifyMoneyReceived(ctx, transfer)
		return DebitAttempt{Transfer: &transfer}, nil
	}
	var blocked *PolicyBlockedError
	if !errors.As(err, &blocked) || strings.TrimSpace(reason) == "" || s.requests == nil {
		return DebitAttempt{}, err
	}
	request, requestErr := s.requests.RequestDebit(ctx, PendingDebit{
		IntentID: OperationID(generated[0]), IdempotencyKey: key, OwnerID: senderID,
		Kind: DebitInternalTransfer, Destination: recipientEmail, Amount: amount,
		Reason: reason, Blocks: blocked.Blocks, CreatedAt: now,
	})
	if requestErr != nil {
		return DebitAttempt{}, requestErr
	}
	return DebitAttempt{Request: &request}, nil
}

func (s *Service) notifyMoneyReceived(ctx context.Context, transfer Transfer) {
	if s == nil || s.notifier == nil || transfer.Duplicate || transfer.RecipientID == "" || transfer.LedgerTransactionID == "" {
		return
	}
	amount := fmt.Sprintf("$%.2f", float64(transfer.Amount.AmountMinor())/100)
	body := "You received " + amount
	if s.users != nil {
		if sender, err := s.users.UserByID(ctx, transfer.SenderID); err == nil && strings.TrimSpace(sender.DisplayName()) != "" {
			body = sender.DisplayName() + " paid you " + amount
		}
	}
	s.notifier.Notify(transfer.RecipientID, notification.Message{
		Title:    "Money received",
		Body:     body,
		Kind:     notification.KindMoneyReceived,
		EntityID: string(transfer.LedgerTransactionID),
	})
}

func (s *Service) CreateWithdrawal(
	ctx context.Context,
	key string,
	ownerID identity.UserID,
	destination string,
	amountMinor int64,
	currency string,
) (Withdrawal, error) {
	attempt, err := s.CreateWithdrawalWithReason(ctx, key, ownerID, destination, amountMinor, currency, "")
	if err != nil {
		return Withdrawal{}, err
	}
	if attempt.Withdrawal == nil {
		return Withdrawal{}, ErrPolicyDenied
	}
	return *attempt.Withdrawal, nil
}

func (s *Service) CreateWithdrawalWithReason(
	ctx context.Context,
	key string,
	ownerID identity.UserID,
	destination string,
	amountMinor int64,
	currency string,
	reason string,
) (DebitAttempt, error) {
	if err := s.available(); err != nil {
		return DebitAttempt{}, err
	}
	key, err := normalizeKey(key)
	if err != nil {
		return DebitAttempt{}, err
	}
	destination = strings.TrimSpace(destination)
	amount, err := money.New(amountMinor, strings.ToUpper(strings.TrimSpace(currency)))
	if err != nil || !amount.Positive() || ownerID == "" || destination == "" {
		return DebitAttempt{}, ErrInvalidOperation
	}
	if err := s.policy.Authorize(ctx, DebitRequest{
		OwnerID: ownerID, Kind: DebitWithdrawal,
		ReferenceKey: "withdrawal:" + string(ownerID) + ":" + key,
		Amount:       amount,
	}); err != nil {
		return DebitAttempt{}, err
	}
	generated, err := generateIDs(s.ids, 2)
	if err != nil {
		return DebitAttempt{}, err
	}
	now := s.clock.Now().UTC()
	withdrawal, err := s.repo.ReserveWithdrawal(ctx, key, Withdrawal{
		ID: OperationID(generated[0]), OwnerID: ownerID, Destination: destination,
		Amount: amount, Status: WithdrawalReserved, CreatedAt: now, UpdatedAt: now,
	}, ledger.JournalID(generated[1]))
	if err != nil {
		var blocked *PolicyBlockedError
		if !errors.As(err, &blocked) || strings.TrimSpace(reason) == "" || s.requests == nil {
			return DebitAttempt{}, err
		}
		request, requestErr := s.requests.RequestDebit(ctx, PendingDebit{
			IntentID: OperationID(generated[0]), IdempotencyKey: key, OwnerID: ownerID,
			Kind: DebitWithdrawal, Destination: destination, Amount: amount,
			Reason: reason, Blocks: blocked.Blocks, CreatedAt: now,
		})
		if requestErr != nil {
			return DebitAttempt{}, requestErr
		}
		return DebitAttempt{Request: &request}, nil
	}
	if withdrawal.Status != WithdrawalReserved {
		return DebitAttempt{Withdrawal: &withdrawal}, nil
	}
	payout, err := s.custody.CreatePayout(ctx, custody.CreatePayout{
		IdempotencyKey: "withdrawal:" + string(withdrawal.ID),
		Amount:         withdrawal.Amount,
		Destination:    withdrawal.Destination,
	})
	if err != nil {
		return DebitAttempt{}, err
	}
	attached, err := s.repo.AttachPayout(ctx, ownerID, withdrawal.ID, payout, s.clock.Now().UTC())
	if err != nil {
		return DebitAttempt{}, err
	}
	attached.Duplicate = withdrawal.Duplicate
	return DebitAttempt{Withdrawal: &attached}, nil
}

func (s *Service) ReconcileWithdrawal(
	ctx context.Context,
	ownerID identity.UserID,
	id OperationID,
) (Withdrawal, error) {
	if err := s.available(); err != nil {
		return Withdrawal{}, err
	}
	withdrawal, err := s.repo.Withdrawal(ctx, ownerID, id)
	if err != nil {
		return Withdrawal{}, err
	}
	if withdrawal.Status == WithdrawalCompleted || withdrawal.Status == WithdrawalFailed {
		withdrawal.Duplicate = true
		return withdrawal, nil
	}
	if withdrawal.Status != WithdrawalPending || withdrawal.ProviderPayoutID == "" {
		return Withdrawal{}, ErrInvalidState
	}
	payout, err := s.custody.GetPayout(ctx, withdrawal.ProviderPayoutID)
	if err != nil {
		return Withdrawal{}, err
	}
	if payout.Status == custody.PayoutPending {
		return withdrawal, nil
	}
	generated, err := generateIDs(s.ids, 2)
	if err != nil {
		return Withdrawal{}, err
	}
	return s.repo.SettleWithdrawal(
		ctx,
		ownerID,
		id,
		payout.Status == custody.PayoutSucceeded,
		ledger.JournalID(generated[0]),
		ledger.AccountID(generated[1]),
		s.clock.Now().UTC(),
	)
}

func (s *Service) ReconcilePendingWithdrawals(ctx context.Context, limit int) (ReconciliationResult, error) {
	if err := s.available(); err != nil {
		return ReconciliationResult{}, err
	}
	if limit <= 0 {
		return ReconciliationResult{}, ErrInvalidOperation
	}
	withdrawals, err := s.repo.PendingWithdrawals(ctx, limit)
	if err != nil {
		return ReconciliationResult{}, err
	}
	var result ReconciliationResult
	var reconciliationErrors []error
	for _, withdrawal := range withdrawals {
		settled, reconcileErr := s.ReconcileWithdrawal(ctx, withdrawal.OwnerID, withdrawal.ID)
		if reconcileErr != nil {
			reconciliationErrors = append(reconciliationErrors, reconcileErr)
			continue
		}
		switch settled.Status {
		case WithdrawalCompleted:
			result.Completed++
		case WithdrawalFailed:
			result.Failed++
		default:
			result.Pending++
		}
	}
	return result, errors.Join(reconciliationErrors...)
}

func (s *Service) SimulateWithdrawalSettlement(
	ctx context.Context,
	ownerID identity.UserID,
	id OperationID,
	succeed bool,
) (Withdrawal, error) {
	if err := s.available(); err != nil {
		return Withdrawal{}, err
	}
	simulator, ok := s.custody.(custody.PayoutSimulator)
	if !ok {
		return Withdrawal{}, ErrUnavailable
	}
	withdrawal, err := s.repo.Withdrawal(ctx, ownerID, id)
	if err != nil {
		return Withdrawal{}, err
	}
	if withdrawal.Status == WithdrawalCompleted || withdrawal.Status == WithdrawalFailed {
		withdrawal.Duplicate = true
		return withdrawal, nil
	}
	if withdrawal.Status != WithdrawalPending || withdrawal.ProviderPayoutID == "" {
		return Withdrawal{}, ErrInvalidState
	}
	payout, err := s.custody.GetPayout(ctx, withdrawal.ProviderPayoutID)
	if err != nil {
		return Withdrawal{}, err
	}
	if payout.Status == custody.PayoutPending {
		if _, err := simulator.SettlePayout(ctx, payout.ID, succeed); err != nil {
			return Withdrawal{}, err
		}
	}
	return s.ReconcileWithdrawal(ctx, ownerID, id)
}

func (s *Service) available() error {
	if s == nil || s.ids == nil || s.clock == nil || s.repo == nil || s.custody == nil || s.policy == nil {
		return ErrUnavailable
	}
	return nil
}

func generateIDs(generator IDGenerator, count int) ([]string, error) {
	values := make([]string, 0, count)
	for range count {
		value, err := generator.NewID()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}
