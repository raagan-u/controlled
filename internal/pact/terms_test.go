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
		name        string
		limit       pact.Money
		stake       pact.Money
		start       time.Time
		end         time.Time
		circle      pact.CircleID
		destination pact.DestinationID
		wantErr     error
	}{
		{
			name:        "valid",
			limit:       limit,
			stake:       stake,
			start:       start,
			end:         end,
			circle:      "circle-1",
			destination: "fund-1",
		},
		{
			name:        "currency mismatch",
			limit:       limit,
			stake:       mustMoney(t, 2_500, "EUR"),
			start:       start,
			end:         end,
			circle:      "circle-1",
			destination: "fund-1",
			wantErr:     pact.ErrCurrencyMismatch,
		},
		{
			name:        "invalid period",
			limit:       limit,
			stake:       stake,
			start:       start,
			end:         start,
			circle:      "circle-1",
			destination: "fund-1",
			wantErr:     pact.ErrInvalidPeriod,
		},
		{
			name:        "missing circle",
			limit:       limit,
			stake:       stake,
			start:       start,
			end:         end,
			destination: "fund-1",
			wantErr:     pact.ErrMissingAccountabilityCircle,
		},
		{
			name:    "missing destination",
			limit:   limit,
			stake:   stake,
			start:   start,
			end:     end,
			circle:  "circle-1",
			wantErr: pact.ErrMissingForfeitureDestination,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			terms, err := pact.NewTerms(
				test.limit,
				test.stake,
				test.start,
				test.end,
				test.circle,
				test.destination,
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
