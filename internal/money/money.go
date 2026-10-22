// Package money provides the shared monetary value used across PayPact modules.
package money

import "errors"

var ErrInvalid = errors.New("money must have a non-negative amount and a three-letter uppercase currency")

// Money stores an amount in a currency's minor unit. For example, USD 25.00
// is represented as 2500. Zero is valid because wallets begin empty.
type Money struct {
	amountMinor int64
	currency    string
}

func New(amountMinor int64, currency string) (Money, error) {
	value := Money{amountMinor: amountMinor, currency: currency}
	if !value.Valid() {
		return Money{}, ErrInvalid
	}

	return value, nil
}

func (m Money) AmountMinor() int64 {
	return m.amountMinor
}

func (m Money) Currency() string {
	return m.currency
}

func (m Money) Valid() bool {
	return m.amountMinor >= 0 && ValidCurrency(m.currency)
}

func (m Money) Positive() bool {
	return m.amountMinor > 0 && ValidCurrency(m.currency)
}

func (m Money) Equal(other Money) bool {
	return m.amountMinor == other.amountMinor && m.currency == other.currency
}

func ValidCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}

	for _, character := range currency {
		if character < 'A' || character > 'Z' {
			return false
		}
	}

	return true
}
