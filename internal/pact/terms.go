package pact

import (
	"strings"
	"time"
)

type (
	PactID     string
	UserID     string
	CircleID   string
	OverrideID string
)

// Terms are immutable after a pact is confirmed.
type Terms struct {
	goal                 string
	intent               string
	spendingLimit        Money
	stake                Money
	startsAt             time.Time
	endsAt               time.Time
	accountabilityCircle CircleID
	breakConsequence     BreakConsequence
}

func NewTerms(
	goal string,
	intent string,
	spendingLimit Money,
	stake Money,
	startsAt time.Time,
	endsAt time.Time,
	accountabilityCircle CircleID,
	breakConsequence BreakConsequence,
) (Terms, error) {
	terms := Terms{
		goal:                 strings.TrimSpace(goal),
		intent:               strings.TrimSpace(intent),
		spendingLimit:        spendingLimit,
		stake:                stake,
		startsAt:             startsAt.UTC(),
		endsAt:               endsAt.UTC(),
		accountabilityCircle: accountabilityCircle,
		breakConsequence:     breakConsequence,
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
	if t.goal == "" {
		return ErrMissingGoal
	}
	if t.intent == "" {
		return ErrMissingIntent
	}
	if t.startsAt.IsZero() || t.endsAt.IsZero() || !t.startsAt.Before(t.endsAt) {
		return ErrInvalidPeriod
	}
	if t.accountabilityCircle == "" {
		return ErrMissingAccountabilityCircle
	}
	if !t.breakConsequence.valid() {
		return ErrInvalidBreakConsequence
	}

	return nil
}

func (t Terms) Goal() string {
	return t.goal
}

func (t Terms) Intent() string {
	return t.intent
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

func (t Terms) BreakConsequence() BreakConsequence {
	return t.breakConsequence
}
