package money_test

import (
	"errors"
	"testing"

	"github.com/raagan-u/controlled/internal/money"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		amountMinor int64
		currency    string
		wantErr     error
	}{
		{name: "positive", amountMinor: 2500, currency: "USD"},
		{name: "zero", amountMinor: 0, currency: "USD"},
		{name: "negative", amountMinor: -1, currency: "USD", wantErr: money.ErrInvalid},
		{name: "lowercase currency", amountMinor: 2500, currency: "usd", wantErr: money.ErrInvalid},
		{name: "invalid currency length", amountMinor: 2500, currency: "US", wantErr: money.ErrInvalid},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			value, err := money.New(test.amountMinor, test.currency)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("New() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if value.AmountMinor() != test.amountMinor || value.Currency() != test.currency {
				t.Fatalf("New() = (%d, %q), want (%d, %q)", value.AmountMinor(), value.Currency(), test.amountMinor, test.currency)
			}
		})
	}
}

func TestPositive(t *testing.T) {
	t.Parallel()

	zero, err := money.New(0, "USD")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if zero.Positive() {
		t.Fatal("zero money must not be positive")
	}
}
