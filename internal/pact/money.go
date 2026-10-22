package pact

import sharedmoney "github.com/raagan-u/pay-pact/internal/money"

type Money = sharedmoney.Money

func NewMoney(amountMinor int64, currency string) (Money, error) {
	money, err := sharedmoney.New(amountMinor, currency)
	if err != nil || !money.Positive() {
		return Money{}, ErrInvalidMoney
	}

	return money, nil
}
