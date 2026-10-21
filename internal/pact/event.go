package pact

import "time"

type EventType string

const (
	EventPactCreated               EventType = "PACT_CREATED"
	EventDraftUpdated              EventType = "DRAFT_UPDATED"
	EventPactConfirmed             EventType = "PACT_CONFIRMED"
	EventStakeReserved             EventType = "STAKE_RESERVED"
	EventPactStarted               EventType = "PACT_STARTED"
	EventEmergencyExceptionApplied EventType = "EMERGENCY_EXCEPTION_APPLIED"
	EventPactBroken                EventType = "PACT_BROKEN"
	EventPactCompleted             EventType = "PACT_COMPLETED"
	EventPactCancelled             EventType = "PACT_CANCELLED"
)

// Event records a domain fact. Adapters can translate it into outbox messages,
// payment commands, notifications, or an audit log.
type Event struct {
	Type             EventType
	OccurredAt       time.Time
	StakeDisposition StakeDisposition
	OverrideID       OverrideID
}
