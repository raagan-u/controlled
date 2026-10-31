package pact_test

import (
	"context"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/pact"
)

type pactIDs struct{ value string }

func (g pactIDs) NewID() (string, error) { return g.value, nil }

type pactClock struct{ now time.Time }

func (c pactClock) Now() time.Time { return c.now }

type pactRepository struct{ stored *pact.Pact }

func (r *pactRepository) Create(_ context.Context, _ string, value *pact.Pact) (*pact.Pact, bool, error) {
	r.stored = value
	return value, false, nil
}
func (r *pactRepository) Save(_ context.Context, value *pact.Pact, _ uint64) error {
	r.stored = value
	return nil
}
func (r *pactRepository) ByID(_ context.Context, _ pact.UserID, _ pact.PactID) (*pact.Pact, error) {
	return r.stored, nil
}
func (r *pactRepository) ForOwner(_ context.Context, _ pact.UserID) ([]*pact.Pact, error) {
	return []*pact.Pact{r.stored}, nil
}
func (r *pactRepository) CircleFacts(_ context.Context, _ pact.UserID, _ pact.CircleID) (pact.ConfirmationFacts, error) {
	return pact.ConfirmationFacts{PartnerCount: 2, DistinctPartnerCount: 2}, nil
}
func (r *pactRepository) ReserveStake(_ context.Context, _ pact.UserID, _ pact.PactID, journalID, _ string, _ string, now time.Time) (*pact.Pact, bool, error) {
	err := r.stored.RecordStakeReserved(pact.StakeReservation{ID: journalID, Amount: r.stored.Terms().Stake()}, now)
	return r.stored, false, err
}
func (r *pactRepository) Cancel(_ context.Context, _ pact.UserID, _ pact.PactID, _ string, now time.Time) (*pact.Pact, bool, error) {
	if r.stored.Status() == pact.StatusCancelled {
		return r.stored, true, nil
	}
	err := r.stored.Cancel(now)
	return r.stored, false, err
}

func TestPactServiceDraftUpdateAndConfirmation(t *testing.T) {
	now := time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)
	repository := &pactRepository{}
	service := pact.NewService(pactIDs{"pact-1"}, pactClock{now}, repository)
	terms := serviceTerms(t, now.Add(time.Hour), now.Add(8*24*time.Hour))

	draft, duplicate, err := service.CreateDraft(t.Context(), "create-1", "owner", terms)
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if duplicate || draft.Status() != pact.StatusDraft {
		t.Fatalf("draft status = %s, duplicate = %v", draft.Status(), duplicate)
	}

	updatedTerms := serviceTerms(t, now.Add(2*time.Hour), now.Add(9*24*time.Hour))
	updated, err := service.UpdateDraft(t.Context(), "owner", draft.ID(), updatedTerms)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	if !updated.Terms().Equal(updatedTerms) {
		t.Fatal("updated terms were not saved")
	}

	confirmed, err := service.Confirm(t.Context(), "owner", draft.ID())
	if err != nil {
		t.Fatalf("confirm pact: %v", err)
	}
	if confirmed.Status() != pact.StatusAwaitingStake {
		t.Fatalf("status = %s, want awaiting stake", confirmed.Status())
	}
	if _, err := service.Confirm(t.Context(), "owner", draft.ID()); err != nil {
		t.Fatalf("repeat confirmation should be idempotent: %v", err)
	}
}

func TestPactServiceReservesStakeAndCancelsScheduledPact(t *testing.T) {
	now := time.Date(2026, time.October, 30, 12, 0, 0, 0, time.UTC)
	repository := &pactRepository{}
	service := pact.NewService(&sequencePactIDs{values: []string{"pact-1", "journal", "vault", "release"}}, pactClock{now}, repository)
	draft, _, err := service.CreateDraft(t.Context(), "create", "owner", serviceTerms(t, now.Add(time.Hour), now.Add(24*time.Hour)))
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if _, err := service.Confirm(t.Context(), "owner", draft.ID()); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	reserved, duplicate, err := service.ReserveStake(t.Context(), "stake-key", "owner", draft.ID())
	if err != nil {
		t.Fatalf("reserve stake: %v", err)
	}
	if duplicate || reserved.Status() != pact.StatusScheduled {
		t.Fatalf("status = %s, duplicate = %v", reserved.Status(), duplicate)
	}
	cancelled, duplicate, err := service.Cancel(t.Context(), "owner", draft.ID())
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if duplicate || cancelled.Status() != pact.StatusCancelled {
		t.Fatalf("status = %s, duplicate = %v", cancelled.Status(), duplicate)
	}
}

type sequencePactIDs struct{ values []string }

func (g *sequencePactIDs) NewID() (string, error) {
	value := g.values[0]
	g.values = g.values[1:]
	return value, nil
}

func serviceTerms(t *testing.T, start, end time.Time) pact.Terms {
	t.Helper()
	limit, _ := pact.NewMoney(20_000, "USD")
	stake, _ := pact.NewMoney(5_000, "USD")
	terms, err := pact.NewTerms(
		"Save for rent", "Limit discretionary spending", limit, stake,
		start, end, "circle-1", pact.BreakConsequenceCommitmentReserve,
	)
	if err != nil {
		t.Fatalf("new terms: %v", err)
	}
	return terms
}
