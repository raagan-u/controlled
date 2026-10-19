package pact

import "time"

// Pact is the consistency boundary for commitment terms and lifecycle rules.
// Payment execution, partner selection, and persistence live outside this package.
type Pact struct {
	id                 PactID
	ownerID            UserID
	terms              Terms
	status             Status
	outcome            Outcome
	fundingID          string
	emergencyCount     int
	processedOverrides map[OverrideID]struct{}
	version            uint64
	events             []Event
}

func CreateDraft(id PactID, ownerID UserID, terms Terms, now time.Time) (*Pact, error) {
	if id == "" {
		return nil, ErrInvalidPactID
	}
	if ownerID == "" {
		return nil, ErrInvalidOwnerID
	}
	if now.IsZero() {
		return nil, ErrInvalidTimestamp
	}
	if err := terms.Validate(); err != nil {
		return nil, err
	}

	p := &Pact{
		id:                 id,
		ownerID:            ownerID,
		terms:              terms,
		status:             StatusDraft,
		outcome:            OutcomeNone,
		processedOverrides: make(map[OverrideID]struct{}),
		version:            1,
	}
	p.record(EventPactCreated, now, StakeDispositionNone, "")

	return p, nil
}

func (p *Pact) UpdateDraft(terms Terms, now time.Time) error {
	if err := p.requireTime(now); err != nil {
		return err
	}
	if p.status != StatusDraft {
		return ErrInvalidTransition
	}
	if err := terms.Validate(); err != nil {
		return err
	}

	p.terms = terms
	p.changed()
	p.record(EventDraftUpdated, now, StakeDispositionNone, "")
	return nil
}

func (p *Pact) Confirm(facts ConfirmationFacts, now time.Time) error {
	if err := p.requireTime(now); err != nil {
		return err
	}
	if p.status != StatusDraft {
		return ErrInvalidTransition
	}
	if !facts.valid() {
		return ErrAccountabilityNotReady
	}
	if !now.Before(p.terms.EndsAt()) {
		return ErrPeriodEnded
	}

	p.status = StatusAwaitingStake
	p.changed()
	p.record(EventPactConfirmed, now, StakeDispositionNone, "")
	return nil
}

func (p *Pact) RecordStakeFunded(funding StakeFunding, now time.Time) error {
	if err := p.requireTime(now); err != nil {
		return err
	}
	if funding.ID != "" && funding.ID == p.fundingID {
		return nil
	}
	if p.status != StatusAwaitingStake {
		return ErrInvalidTransition
	}
	if !funding.valid() {
		return ErrInvalidFunding
	}
	if !funding.Amount.Equal(p.terms.Stake()) {
		return ErrFundingMismatch
	}
	if !now.Before(p.terms.EndsAt()) {
		return ErrPeriodEnded
	}

	p.fundingID = funding.ID
	if now.Before(p.terms.StartsAt()) {
		p.status = StatusScheduled
	} else {
		p.status = StatusActive
	}
	p.changed()
	p.record(EventStakeFunded, now, StakeDispositionHold, "")
	if p.status == StatusActive {
		p.record(EventPactStarted, now, StakeDispositionHold, "")
	}
	return nil
}

func (p *Pact) Start(now time.Time) error {
	if err := p.requireTime(now); err != nil {
		return err
	}
	if p.status != StatusScheduled {
		return ErrInvalidTransition
	}
	if now.Before(p.terms.StartsAt()) {
		return ErrStartTimeNotReached
	}
	if !now.Before(p.terms.EndsAt()) {
		return ErrPeriodEnded
	}

	p.status = StatusActive
	p.changed()
	p.record(EventPactStarted, now, StakeDispositionHold, "")
	return nil
}

func (p *Pact) ApplyOverride(decision OverrideDecision, now time.Time) error {
	if err := p.requireTime(now); err != nil {
		return err
	}
	if _, exists := p.processedOverrides[decision.ID]; decision.ID != "" && exists {
		return nil
	}
	if p.status != StatusActive {
		return ErrInvalidTransition
	}
	if !decision.valid() {
		return ErrInvalidOverride
	}
	if !now.Before(p.terms.EndsAt()) {
		return ErrPeriodEnded
	}

	p.processedOverrides[decision.ID] = struct{}{}
	p.changed()
	switch decision.Type {
	case OverrideEmergency:
		p.emergencyCount++
		p.record(EventEmergencyExceptionApplied, now, StakeDispositionHold, decision.ID)
	case OverrideConsciousBreak:
		p.status = StatusResolved
		p.outcome = OutcomeBroken
		p.record(EventPactBroken, now, StakeDispositionForfeit, decision.ID)
	}

	return nil
}

func (p *Pact) Complete(now time.Time) error {
	if err := p.requireTime(now); err != nil {
		return err
	}
	if p.status != StatusActive {
		return ErrInvalidTransition
	}
	if now.Before(p.terms.EndsAt()) {
		return ErrPeriodNotEnded
	}

	p.status = StatusResolved
	if p.emergencyCount == 0 {
		p.outcome = OutcomeKept
	} else {
		p.outcome = OutcomeKeptWithExceptions
	}
	p.changed()
	p.record(EventPactCompleted, now, StakeDispositionRefund, "")
	return nil
}

func (p *Pact) Cancel(now time.Time) error {
	if err := p.requireTime(now); err != nil {
		return err
	}

	disposition := StakeDispositionNone
	switch p.status {
	case StatusDraft, StatusAwaitingStake:
	case StatusScheduled:
		disposition = StakeDispositionRefund
	case StatusActive:
		return ErrActivePactCannotBeCancelled
	default:
		return ErrInvalidTransition
	}

	p.status = StatusCancelled
	p.outcome = OutcomeCancelled
	p.changed()
	p.record(EventPactCancelled, now, disposition, "")
	return nil
}

func (p *Pact) ID() PactID                      { return p.id }
func (p *Pact) OwnerID() UserID                 { return p.ownerID }
func (p *Pact) Terms() Terms                    { return p.terms }
func (p *Pact) Status() Status                  { return p.status }
func (p *Pact) Outcome() Outcome                { return p.outcome }
func (p *Pact) FundingID() string               { return p.fundingID }
func (p *Pact) EmergencyCount() int             { return p.emergencyCount }
func (p *Pact) Version() uint64                 { return p.version }
func (p *Pact) Events() []Event                 { return append([]Event(nil), p.events...) }
func (p *Pact) ClearEvents()                    { p.events = nil }
func (p *Pact) requireTime(now time.Time) error { return timeRequired(now) }
func (p *Pact) changed()                        { p.version++ }

func (p *Pact) record(kind EventType, at time.Time, disposition StakeDisposition, overrideID OverrideID) {
	p.events = append(p.events, Event{
		Type:             kind,
		OccurredAt:       at.UTC(),
		StakeDisposition: disposition,
		OverrideID:       overrideID,
	})
}

func timeRequired(now time.Time) error {
	if now.IsZero() {
		return ErrInvalidTimestamp
	}
	return nil
}
