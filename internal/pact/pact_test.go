package pact_test

import (
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/pay-pact/internal/pact"
)

func TestPactSuccessfulLifecycle(t *testing.T) {
	t.Parallel()

	p, start, end := mustDraft(t)
	mustConfirm(t, p, start.Add(-2*time.Hour))
	mustFund(t, p, start.Add(-time.Hour))

	assertStatus(t, p, pact.StatusScheduled, pact.OutcomeNone)
	if err := p.Start(start); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := p.Complete(end); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	assertStatus(t, p, pact.StatusResolved, pact.OutcomeKept)
	last := lastEvent(t, p)
	if last.Type != pact.EventPactCompleted || last.StakeDisposition != pact.StakeDispositionRefund {
		t.Fatalf("completion event = %#v", last)
	}
}

func TestEmergencyIsAnExceptionNotAFailure(t *testing.T) {
	t.Parallel()

	p, start, end := mustActivePact(t)
	decision := pact.OverrideDecision{
		ID:               "override-1",
		Type:             pact.OverrideEmergency,
		PurchaseIntentID: "purchase-1",
		Approved:         true,
	}

	if err := p.ApplyOverride(decision, start.Add(time.Hour)); err != nil {
		t.Fatalf("ApplyOverride() error = %v", err)
	}
	version := p.Version()
	eventCount := len(p.Events())
	if err := p.ApplyOverride(decision, start.Add(2*time.Hour)); err != nil {
		t.Fatalf("duplicate ApplyOverride() error = %v", err)
	}
	if p.Version() != version || len(p.Events()) != eventCount {
		t.Fatal("duplicate override was not idempotent")
	}
	if err := p.Complete(end); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	assertStatus(t, p, pact.StatusResolved, pact.OutcomeKeptWithExceptions)
	if p.EmergencyCount() != 1 {
		t.Fatalf("EmergencyCount() = %d, want 1", p.EmergencyCount())
	}
}

func TestConsciousBreakForfeitsStake(t *testing.T) {
	t.Parallel()

	p, start, _ := mustActivePact(t)
	err := p.ApplyOverride(pact.OverrideDecision{
		ID:               "override-2",
		Type:             pact.OverrideConsciousBreak,
		PurchaseIntentID: "purchase-2",
		Approved:         true,
	}, start.Add(time.Hour))
	if err != nil {
		t.Fatalf("ApplyOverride() error = %v", err)
	}

	assertStatus(t, p, pact.StatusResolved, pact.OutcomeBroken)
	last := lastEvent(t, p)
	if last.Type != pact.EventPactBroken || last.StakeDisposition != pact.StakeDispositionForfeit {
		t.Fatalf("break event = %#v", last)
	}
}

func TestCancellationStakeDisposition(t *testing.T) {
	t.Parallel()

	t.Run("unfunded pact has no stake movement", func(t *testing.T) {
		p, start, _ := mustDraft(t)
		if err := p.Cancel(start.Add(-time.Hour)); err != nil {
			t.Fatalf("Cancel() error = %v", err)
		}
		if got := lastEvent(t, p).StakeDisposition; got != pact.StakeDispositionNone {
			t.Fatalf("StakeDisposition = %s, want %s", got, pact.StakeDispositionNone)
		}
	})

	t.Run("funded scheduled pact refunds stake", func(t *testing.T) {
		p, start, _ := mustDraft(t)
		mustConfirm(t, p, start.Add(-2*time.Hour))
		mustFund(t, p, start.Add(-time.Hour))
		if err := p.Cancel(start.Add(-30 * time.Minute)); err != nil {
			t.Fatalf("Cancel() error = %v", err)
		}
		if got := lastEvent(t, p).StakeDisposition; got != pact.StakeDispositionRefund {
			t.Fatalf("StakeDisposition = %s, want %s", got, pact.StakeDispositionRefund)
		}
	})
}

func TestActivePactCannotBeCancelled(t *testing.T) {
	t.Parallel()

	p, start, _ := mustActivePact(t)
	if err := p.Cancel(start.Add(time.Hour)); !errors.Is(err, pact.ErrActivePactCannotBeCancelled) {
		t.Fatalf("Cancel() error = %v, want %v", err, pact.ErrActivePactCannotBeCancelled)
	}
}

func TestConfirmationRequiresFourDistinctPartners(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		facts pact.ConfirmationFacts
	}{
		{name: "too few", facts: pact.ConfirmationFacts{PartnerCount: 3, DistinctPartnerCount: 3}},
		{name: "duplicate", facts: pact.ConfirmationFacts{PartnerCount: 4, DistinctPartnerCount: 3}},
		{name: "owner included", facts: pact.ConfirmationFacts{PartnerCount: 4, DistinctPartnerCount: 4, OwnerIsPartner: true}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p, start, _ := mustDraft(t)
			err := p.Confirm(test.facts, start.Add(-time.Hour))
			if !errors.Is(err, pact.ErrAccountabilityNotReady) {
				t.Fatalf("Confirm() error = %v, want %v", err, pact.ErrAccountabilityNotReady)
			}
		})
	}
}

func TestStakeFundingMustMatchTerms(t *testing.T) {
	t.Parallel()

	p, start, _ := mustDraft(t)
	mustConfirm(t, p, start.Add(-time.Hour))
	err := p.RecordStakeFunded(pact.StakeFunding{
		ID:     "capture-1",
		Amount: mustMoney(t, 1_00, "USD"),
	}, start.Add(-30*time.Minute))
	if !errors.Is(err, pact.ErrFundingMismatch) {
		t.Fatalf("RecordStakeFunded() error = %v, want %v", err, pact.ErrFundingMismatch)
	}
}

func TestTermsCanOnlyChangeInDraft(t *testing.T) {
	t.Parallel()

	p, start, end := mustDraft(t)
	mustConfirm(t, p, start.Add(-time.Hour))
	updated := mustTerms(t, start, end.Add(24*time.Hour))
	if err := p.UpdateDraft(updated, start.Add(-30*time.Minute)); !errors.Is(err, pact.ErrInvalidTransition) {
		t.Fatalf("UpdateDraft() error = %v, want %v", err, pact.ErrInvalidTransition)
	}
}

func mustDraft(t *testing.T) (*pact.Pact, time.Time, time.Time) {
	t.Helper()

	start := time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour)
	p, err := pact.CreateDraft("pact-1", "user-1", mustTerms(t, start, end), start.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	return p, start, end
}

func mustActivePact(t *testing.T) (*pact.Pact, time.Time, time.Time) {
	t.Helper()

	p, start, end := mustDraft(t)
	mustConfirm(t, p, start.Add(-time.Hour))
	mustFund(t, p, start)
	assertStatus(t, p, pact.StatusActive, pact.OutcomeNone)
	return p, start, end
}

func mustTerms(t *testing.T, start, end time.Time) pact.Terms {
	t.Helper()

	terms, err := pact.NewTerms(
		mustMoney(t, 15_000, "USD"),
		mustMoney(t, 2_500, "USD"),
		start,
		end,
		"circle-1",
		"fund-1",
	)
	if err != nil {
		t.Fatalf("NewTerms() error = %v", err)
	}
	return terms
}

func mustConfirm(t *testing.T, p *pact.Pact, now time.Time) {
	t.Helper()

	err := p.Confirm(pact.ConfirmationFacts{PartnerCount: 4, DistinctPartnerCount: 4}, now)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
}

func mustFund(t *testing.T, p *pact.Pact, now time.Time) {
	t.Helper()

	err := p.RecordStakeFunded(pact.StakeFunding{ID: "capture-1", Amount: p.Terms().Stake()}, now)
	if err != nil {
		t.Fatalf("RecordStakeFunded() error = %v", err)
	}
}

func assertStatus(t *testing.T, p *pact.Pact, status pact.Status, outcome pact.Outcome) {
	t.Helper()

	if p.Status() != status || p.Outcome() != outcome {
		t.Fatalf("state = (%s, %s), want (%s, %s)", p.Status(), p.Outcome(), status, outcome)
	}
}

func lastEvent(t *testing.T, p *pact.Pact) pact.Event {
	t.Helper()

	events := p.Events()
	if len(events) == 0 {
		t.Fatal("expected a domain event")
	}
	return events[len(events)-1]
}
