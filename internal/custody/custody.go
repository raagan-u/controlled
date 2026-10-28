// Package custody defines the boundary between Controlled's internal ledger and
// external providers that hold or move its backing funds.
package custody

import (
	"context"
	"errors"

	"github.com/raagan-u/controlled/internal/money"
)

var (
	ErrInvalidRequest      = errors.New("custody request is invalid")
	ErrIdempotencyConflict = errors.New("custody idempotency key was reused with different input")
	ErrOrderNotFound       = errors.New("custody funding order was not found")
	ErrPayoutNotFound      = errors.New("custody payout was not found")
	ErrPayoutSettled       = errors.New("custody payout is already settled")
	ErrUnavailable         = errors.New("custody provider is unavailable")
)

type FundingStatus string

const (
	FundingAwaitingApproval FundingStatus = "AWAITING_APPROVAL"
	FundingCaptured         FundingStatus = "CAPTURED"
)

type PayoutStatus string

const (
	PayoutPending   PayoutStatus = "PENDING"
	PayoutSucceeded PayoutStatus = "SUCCEEDED"
	PayoutFailed    PayoutStatus = "FAILED"
)

type CreateFundingOrder struct {
	IdempotencyKey string
	Amount         money.Money
}

type FundingOrder struct {
	ID          string
	Amount      money.Money
	Status      FundingStatus
	ApprovalURL string
}

type CreatePayout struct {
	IdempotencyKey string
	Amount         money.Money
	Destination    string
}

type Payout struct {
	ID          string
	Amount      money.Money
	Destination string
	Status      PayoutStatus
}

type Provider interface {
	CreateFundingOrder(context.Context, CreateFundingOrder) (FundingOrder, error)
	CaptureFundingOrder(context.Context, string) (FundingOrder, error)
	CreatePayout(context.Context, CreatePayout) (Payout, error)
	GetPayout(context.Context, string) (Payout, error)
}

type PayoutSimulator interface {
	SettlePayout(context.Context, string, bool) (Payout, error)
}
