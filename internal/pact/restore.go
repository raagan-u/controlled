package pact

// Restore rebuilds a persisted pact without replaying lifecycle events.
func Restore(
	id PactID,
	ownerID UserID,
	terms Terms,
	status Status,
	outcome Outcome,
	stakeReservationID string,
	emergencyCount int,
	version uint64,
) (*Pact, error) {
	if id == "" {
		return nil, ErrInvalidPactID
	}
	if ownerID == "" {
		return nil, ErrInvalidOwnerID
	}
	if err := terms.Validate(); err != nil {
		return nil, err
	}
	if version == 0 || emergencyCount < 0 || !validPersistedState(status, outcome, stakeReservationID) {
		return nil, ErrInvalidTransition
	}
	return &Pact{
		id: id, ownerID: ownerID, terms: terms, status: status, outcome: outcome,
		stakeReservationID: stakeReservationID, emergencyCount: emergencyCount,
		processedOverrides: make(map[OverrideID]struct{}), version: version,
	}, nil
}

func validPersistedState(status Status, outcome Outcome, reservationID string) bool {
	switch status {
	case StatusDraft, StatusAwaitingStake:
		return outcome == OutcomeNone && reservationID == ""
	case StatusScheduled, StatusActive:
		return outcome == OutcomeNone && reservationID != ""
	case StatusResolved:
		return (outcome == OutcomeKept || outcome == OutcomeKeptWithExceptions || outcome == OutcomeBroken) && reservationID != ""
	case StatusCancelled:
		return outcome == OutcomeCancelled
	default:
		return false
	}
}
