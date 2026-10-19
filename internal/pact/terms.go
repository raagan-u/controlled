package pact

import "time"

type (
	PactID        string
	UserID        string
	CircleID      string
	DestinationID string
	OverrideID    string
)

// Terms are immutable after a pact is confirmed.
type Terms struct {
	spendingLimit         Money
	stake                 Money
	startsAt              time.Time
	endsAt                time.Time
	accountabilityCircle  CircleID
	forfeitureDestination DestinationID
}

func NewTerms(
	spendingLimit Money,
	stake Money,
	startsAt time.Time,
	endsAt time.Time,
	accountabilityCircle CircleID,
	forfeitureDestination DestinationID,
) (Terms, error) {
	terms := Terms{
		spendingLimit:         spendingLimit,
		stake:                 stake,
		startsAt:              startsAt.UTC(),
		endsAt:                endsAt.UTC(),
		accountabilityCircle:  accountabilityCircle,
		forfeitureDestination: forfeitureDestination,
	}

	if err := terms.Validate(); err != nil {
		return Terms{}, err
	}

	return terms, nil
}

func (t Terms) Validate() error {
	if !t.spendingLimit.Valid() || !t.stake.Valid() {
		return ErrInvalidMoney
	}
	if t.spendingLimit.Currency() != t.stake.Currency() {
		return ErrCurrencyMismatch
	}
	if t.startsAt.IsZero() || t.endsAt.IsZero() || !t.startsAt.Before(t.endsAt) {
		return ErrInvalidPeriod
	}
	if t.accountabilityCircle == "" {
		return ErrMissingAccountabilityCircle
	}
	if t.forfeitureDestination == "" {
		return ErrMissingForfeitureDestination
	}

	return nil
}

func (t Terms) SpendingLimit() Money {
	return t.spendingLimit
}

func (t Terms) Stake() Money {
	return t.stake
}

func (t Terms) StartsAt() time.Time {
	return t.startsAt
}

func (t Terms) EndsAt() time.Time {
	return t.endsAt
}

func (t Terms) AccountabilityCircleID() CircleID {
	return t.accountabilityCircle
}

func (t Terms) ForfeitureDestinationID() DestinationID {
	return t.forfeitureDestination
}
