package pact

// Money stores an amount in the currency's minor unit. For example, USD 25.00
// is represented as 2500. The zero value is invalid.
type Money struct {
	amountMinor int64
	currency    string
}

func NewMoney(amountMinor int64, currency string) (Money, error) {
	money := Money{amountMinor: amountMinor, currency: currency}
	if !money.Valid() {
		return Money{}, ErrInvalidMoney
	}

	return money, nil
}

func (m Money) AmountMinor() int64 {
	return m.amountMinor
}

func (m Money) Currency() string {
	return m.currency
}

func (m Money) Valid() bool {
	return m.amountMinor > 0 && validCurrency(m.currency)
}

func (m Money) Equal(other Money) bool {
	return m.amountMinor == other.amountMinor && m.currency == other.currency
}

func validCurrency(currency string) bool {
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
