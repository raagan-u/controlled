package pact

import "errors"

var (
	ErrInvalidMoney                 = errors.New("money must have a positive amount and a three-letter uppercase currency")
	ErrCurrencyMismatch             = errors.New("spending limit and stake must use the same currency")
	ErrInvalidPeriod                = errors.New("pact period must have a start before its end")
	ErrMissingAccountabilityCircle  = errors.New("accountability circle is required")
	ErrMissingForfeitureDestination = errors.New("forfeiture destination is required")
	ErrInvalidPactID                = errors.New("pact ID is required")
	ErrInvalidOwnerID               = errors.New("owner ID is required")
	ErrInvalidTimestamp             = errors.New("timestamp is required")
	ErrAccountabilityNotReady       = errors.New("accountability circle must contain exactly four distinct partners and exclude the owner")
	ErrInvalidTransition            = errors.New("pact lifecycle transition is not allowed")
	ErrPeriodEnded                  = errors.New("pact period has already ended")
	ErrPeriodNotEnded               = errors.New("pact period has not ended")
	ErrStartTimeNotReached          = errors.New("pact start time has not been reached")
	ErrInvalidFunding               = errors.New("stake funding ID and amount are required")
	ErrFundingMismatch              = errors.New("funded amount must equal the pact stake")
	ErrInvalidOverride              = errors.New("override decision is invalid")
	ErrActivePactCannotBeCancelled  = errors.New("an active pact cannot be cancelled directly")
)
