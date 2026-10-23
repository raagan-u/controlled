// Package walletops owns money-moving wallet commands.
package walletops

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/raagan-u/pay-pact/internal/identity"
	"github.com/raagan-u/pay-pact/internal/ledger"
	"github.com/raagan-u/pay-pact/internal/money"
)

var (
	ErrInvalidOperation      = errors.New("wallet operation is invalid")
	ErrInvalidIdempotencyKey = errors.New("wallet operation idempotency key is required")
	ErrIdempotencyConflict   = errors.New("wallet operation idempotency key was reused with different input")
	ErrOperationNotFound     = errors.New("wallet operation was not found")
	ErrRecipientNotFound     = errors.New("transfer recipient was not found")
	ErrSelfTransfer          = errors.New("a wallet cannot transfer to itself")
	ErrCurrencyMismatch      = errors.New("wallet operation currency does not match the wallet")
	ErrInvalidState          = errors.New("wallet operation is in an invalid state")
	ErrPolicyDenied          = errors.New("wallet operation was denied by policy")
	ErrUnavailable           = errors.New("wallet operation dependencies are unavailable")
	ErrPersistenceID         = errors.New("wallet operation persistence requires UUID identifiers")
)

type OperationID string

type FundingStatus string

const (
	FundingCreated              FundingStatus = "CREATED"
	FundingAwaitingConfirmation FundingStatus = "AWAITING_CONFIRMATION"
	FundingCompleted            FundingStatus = "COMPLETED"
	FundingFailed               FundingStatus = "FAILED"
)

type FundingIntent struct {
	ID                  OperationID
	OwnerID             identity.UserID
	WalletID            string
	Amount              money.Money
	Status              FundingStatus
	ProviderOrderID     string
	ApprovalURL         string
	LedgerTransactionID ledger.JournalID
	CreatedAt           time.Time
	UpdatedAt           time.Time
	CompletedAt         time.Time
	Duplicate           bool
}

type Transfer struct {
	ID                  OperationID
	SenderID            identity.UserID
	RecipientID         identity.UserID
	RecipientEmail      string
	Amount              money.Money
	LedgerTransactionID ledger.JournalID
	CreatedAt           time.Time
	Duplicate           bool
}

type WithdrawalStatus string

const (
	WithdrawalReserved  WithdrawalStatus = "RESERVED"
	WithdrawalPending   WithdrawalStatus = "PENDING"
	WithdrawalCompleted WithdrawalStatus = "COMPLETED"
	WithdrawalFailed    WithdrawalStatus = "FAILED"
)

type Withdrawal struct {
	ID                       OperationID
	OwnerID                  identity.UserID
	WalletID                 string
	Destination              string
	Amount                   money.Money
	Status                   WithdrawalStatus
	ProviderPayoutID         string
	ReservationTransactionID ledger.JournalID
	SettlementTransactionID  ledger.JournalID
	CreatedAt                time.Time
	UpdatedAt                time.Time
	CompletedAt              time.Time
	Duplicate                bool
}

type DebitKind string

const (
	DebitInternalTransfer DebitKind = "INTERNAL_TRANSFER"
	DebitWithdrawal       DebitKind = "WITHDRAWAL"
)

type DebitRequest struct {
	OwnerID      identity.UserID
	Kind         DebitKind
	ReferenceKey string
	Amount       money.Money
}

type Policy interface {
	Authorize(context.Context, DebitRequest) error
}

type AllowAllPolicy struct{}

func (AllowAllPolicy) Authorize(_ context.Context, request DebitRequest) error {
	if request.OwnerID == "" || request.ReferenceKey == "" || !request.Amount.Positive() ||
		(request.Kind != DebitInternalTransfer && request.Kind != DebitWithdrawal) {
		return ErrInvalidOperation
	}
	return nil
}

func normalizeKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrInvalidIdempotencyKey
	}
	return value, nil
}
