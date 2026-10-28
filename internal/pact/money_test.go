package pact_test

import (
	"errors"
	"testing"

	"github.com/raagan-u/controlled/internal/pact"
)

func TestNewMoney(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		amountMinor int64
		currency    string
		wantErr     error
	}{
		{name: "valid", amountMinor: 2500, currency: "USD"},
		{name: "zero amount", amountMinor: 0, currency: "USD", wantErr: pact.ErrInvalidMoney},
		{name: "negative amount", amountMinor: -1, currency: "USD", wantErr: pact.ErrInvalidMoney},
		{name: "lowercase currency", amountMinor: 2500, currency: "usd", wantErr: pact.ErrInvalidMoney},
		{name: "invalid currency length", amountMinor: 2500, currency: "US", wantErr: pact.ErrInvalidMoney},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			money, err := pact.NewMoney(test.amountMinor, test.currency)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("NewMoney() error = %v, want %v", err, test.wantErr)
			}

			if test.wantErr != nil {
				return
			}

			if money.AmountMinor() != test.amountMinor {
				t.Fatalf("AmountMinor() = %d, want %d", money.AmountMinor(), test.amountMinor)
			}
			if money.Currency() != test.currency {
				t.Fatalf("Currency() = %q, want %q", money.Currency(), test.currency)
			}
		})
	}
}
