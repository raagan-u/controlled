package custody

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type IDGenerator interface {
	NewID() (string, error)
}

type Fake struct {
	mu           sync.RWMutex
	ids          IDGenerator
	fundingByID  map[string]FundingOrder
	fundingByKey map[string]string
	payoutsByID  map[string]Payout
	payoutsByKey map[string]string
}

func NewFake(ids IDGenerator) *Fake {
	return &Fake{
		ids:          ids,
		fundingByID:  make(map[string]FundingOrder),
		fundingByKey: make(map[string]string),
		payoutsByID:  make(map[string]Payout),
		payoutsByKey: make(map[string]string),
	}
}

func (f *Fake) CreateFundingOrder(
	ctx context.Context,
	request CreateFundingOrder,
) (FundingOrder, error) {
	if f == nil || f.ids == nil {
		return FundingOrder{}, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return FundingOrder{}, err
	}
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.IdempotencyKey == "" || !request.Amount.Positive() {
		return FundingOrder{}, ErrInvalidRequest
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if id, exists := f.fundingByKey[request.IdempotencyKey]; exists {
		order := f.fundingByID[id]
		if !order.Amount.Equal(request.Amount) {
			return FundingOrder{}, ErrIdempotencyConflict
		}
		return order, nil
	}
	id, err := f.ids.NewID()
	if err != nil {
		return FundingOrder{}, err
	}
	order := FundingOrder{
		ID:          id,
		Amount:      request.Amount,
		Status:      FundingAwaitingApproval,
		ApprovalURL: fmt.Sprintf("https://fake.controlled.local/approve/%s", id),
	}
	f.fundingByID[id] = order
	f.fundingByKey[request.IdempotencyKey] = id
	return order, nil
}

func (f *Fake) CaptureFundingOrder(ctx context.Context, id string) (FundingOrder, error) {
	if f == nil {
		return FundingOrder{}, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return FundingOrder{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	order, exists := f.fundingByID[strings.TrimSpace(id)]
	if !exists {
		return FundingOrder{}, ErrOrderNotFound
	}
	order.Status = FundingCaptured
	f.fundingByID[order.ID] = order
	return order, nil
}

func (f *Fake) CreatePayout(ctx context.Context, request CreatePayout) (Payout, error) {
	if f == nil || f.ids == nil {
		return Payout{}, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return Payout{}, err
	}
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.Destination = strings.TrimSpace(request.Destination)
	if request.IdempotencyKey == "" || request.Destination == "" || !request.Amount.Positive() {
		return Payout{}, ErrInvalidRequest
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if id, exists := f.payoutsByKey[request.IdempotencyKey]; exists {
		payout := f.payoutsByID[id]
		if !payout.Amount.Equal(request.Amount) || payout.Destination != request.Destination {
			return Payout{}, ErrIdempotencyConflict
		}
		return payout, nil
	}
	id, err := f.ids.NewID()
	if err != nil {
		return Payout{}, err
	}
	payout := Payout{
		ID:          id,
		Amount:      request.Amount,
		Destination: request.Destination,
		Status:      PayoutPending,
	}
	f.payoutsByID[id] = payout
	f.payoutsByKey[request.IdempotencyKey] = id
	return payout, nil
}

func (f *Fake) GetPayout(ctx context.Context, id string) (Payout, error) {
	if f == nil {
		return Payout{}, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return Payout{}, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	payout, exists := f.payoutsByID[strings.TrimSpace(id)]
	if !exists {
		return Payout{}, ErrPayoutNotFound
	}
	return payout, nil
}

func (f *Fake) SettlePayout(ctx context.Context, id string, succeed bool) (Payout, error) {
	if f == nil {
		return Payout{}, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return Payout{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	payout, exists := f.payoutsByID[strings.TrimSpace(id)]
	if !exists {
		return Payout{}, ErrPayoutNotFound
	}
	if payout.Status != PayoutPending {
		return Payout{}, ErrPayoutSettled
	}
	if succeed {
		payout.Status = PayoutSucceeded
	} else {
		payout.Status = PayoutFailed
	}
	f.payoutsByID[payout.ID] = payout
	return payout, nil
}
