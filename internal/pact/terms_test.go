package pact_test

import (
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/pay-pact/internal/pact"
)

func TestNewTerms(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour)
	limit := mustMoney(t, 15_000, "USD")
	stake := mustMoney(t, 2_500, "USD")

	tests := []struct {
		name             string
		goal             string
		intent           string
		limit            pact.Money
		stake            pact.Money
		start            time.Time
		end              time.Time
		circle           pact.CircleID
		breakConsequence pact.BreakConsequence
		wantErr          error
	}{
		{
			name:             "valid",
			goal:             "Keep enough money for rent",
			intent:           "Reduce discretionary spending until payday",
			limit:            limit,
			stake:            stake,
			start:            start,
			end:              end,
			circle:           "circle-1",
			breakConsequence: pact.BreakConsequenceCommitmentReserve,
		},
		{
			name:             "currency mismatch",
			goal:             "Keep enough money for rent",
			intent:           "Reduce discretionary spending until payday",
			limit:            limit,
			stake:            mustMoney(t, 2_500, "EUR"),
			start:            start,
			end:              end,
			circle:           "circle-1",
			breakConsequence: pact.BreakConsequenceCommitmentReserve,
			wantErr:          pact.ErrCurrencyMismatch,
		},
		{
			name:             "missing goal",
			intent:           "Reduce discretionary spending until payday",
			limit:            limit,
			stake:            stake,
			start:            start,
			end:              end,
			circle:           "circle-1",
			breakConsequence: pact.BreakConsequenceCommitmentReserve,
			wantErr:          pact.ErrMissingGoal,
		},
		{
			name:             "missing intent",
			goal:             "Keep enough money for rent",
			limit:            limit,
			stake:            stake,
			start:            start,
			end:              end,
			circle:           "circle-1",
			breakConsequence: pact.BreakConsequenceCommitmentReserve,
			wantErr:          pact.ErrMissingIntent,
		},
		{
			name:             "invalid period",
			goal:             "Keep enough money for rent",
			intent:           "Reduce discretionary spending until payday",
			limit:            limit,
			stake:            stake,
			start:            start,
			end:              start,
			circle:           "circle-1",
			breakConsequence: pact.BreakConsequenceCommitmentReserve,
			wantErr:          pact.ErrInvalidPeriod,
		},
		{
			name:             "missing circle",
			goal:             "Keep enough money for rent",
			intent:           "Reduce discretionary spending until payday",
			limit:            limit,
			stake:            stake,
			start:            start,
			end:              end,
			breakConsequence: pact.BreakConsequenceCommitmentReserve,
			wantErr:          pact.ErrMissingAccountabilityCircle,
		},
		{
			name:    "unsupported break consequence",
			goal:    "Keep enough money for rent",
			intent:  "Reduce discretionary spending until payday",
			limit:   limit,
			stake:   stake,
			start:   start,
			end:     end,
			circle:  "circle-1",
			wantErr: pact.ErrInvalidBreakConsequence,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			terms, err := pact.NewTerms(
				test.goal,
				test.intent,
				test.limit,
				test.stake,
				test.start,
				test.end,
				test.circle,
				test.breakConsequence,
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("NewTerms() error = %v, want %v", err, test.wantErr)
			}

			if test.wantErr != nil {
				return
			}

			if !terms.StartsAt().Equal(start) || !terms.EndsAt().Equal(end) {
				t.Fatal("NewTerms() did not preserve the commitment period")
			}
			if terms.Goal() != test.goal || terms.Intent() != test.intent {
				t.Fatal("NewTerms() did not preserve the goal and intent")
			}
		})
	}
}

func mustMoney(t *testing.T, amountMinor int64, currency string) pact.Money {
	t.Helper()

	money, err := pact.NewMoney(amountMinor, currency)
	if err != nil {
		t.Fatalf("NewMoney() error = %v", err)
	}

	return money
}
