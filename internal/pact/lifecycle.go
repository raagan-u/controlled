package pact

type Status string

const (
	StatusDraft         Status = "DRAFT"
	StatusAwaitingStake Status = "AWAITING_STAKE"
	StatusScheduled     Status = "SCHEDULED"
	StatusActive        Status = "ACTIVE"
	StatusResolved      Status = "RESOLVED"
	StatusCancelled     Status = "CANCELLED"
)

type Outcome string

const (
	OutcomeNone               Outcome = "NONE"
	OutcomeKept               Outcome = "KEPT"
	OutcomeKeptWithExceptions Outcome = "KEPT_WITH_EXCEPTIONS"
	OutcomeBroken             Outcome = "BROKEN"
	OutcomeCancelled          Outcome = "CANCELLED"
)

type StakeDisposition string

const (
	StakeDispositionNone    StakeDisposition = "NONE"
	StakeDispositionHold    StakeDisposition = "HOLD"
	StakeDispositionRefund  StakeDisposition = "REFUND"
	StakeDispositionForfeit StakeDisposition = "FORFEIT"
)

type OverrideType string

const (
	OverrideEmergency      OverrideType = "EMERGENCY"
	OverrideConsciousBreak OverrideType = "CONSCIOUS_BREAK"
)

// ConfirmationFacts are supplied by the accountability module after it checks
// circle membership. The pact domain only owns the readiness invariant.
type ConfirmationFacts struct {
	PartnerCount         int
	DistinctPartnerCount int
	OwnerIsPartner       bool
}

func (f ConfirmationFacts) valid() bool {
	return f.PartnerCount == 4 && f.DistinctPartnerCount == 4 && !f.OwnerIsPartner
}

type StakeFunding struct {
	ID     string
	Amount Money
}

func (f StakeFunding) valid() bool {
	return f.ID != "" && f.Amount.Valid()
}

type OverrideDecision struct {
	ID               OverrideID
	Type             OverrideType
	PurchaseIntentID string
	Approved         bool
}

func (d OverrideDecision) valid() bool {
	if d.ID == "" || d.PurchaseIntentID == "" || !d.Approved {
		return false
	}

	return d.Type == OverrideEmergency || d.Type == OverrideConsciousBreak
}
