package pact

import "errors"

var (
	ErrInvalidMoney                 = errors.New("money must have a positive amount and a three-letter uppercase currency")
	ErrCurrencyMismatch             = errors.New("spending limit and stake must use the same currency")
	ErrInvalidPeriod                = errors.New("pact period must have a start before its end")
	ErrMissingAccountabilityCircle  = errors.New("accountability circle is required")
	ErrMissingForfeitureDestination = errors.New("forfeiture destination is required")
)
