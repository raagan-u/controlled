// Package ai provides grounded, read-only guidance for Controlled.
//
// Models can propose language and pact terms, but this package has no access to
// wallet commands, the ledger book, or pact lifecycle mutations.
package ai

import (
	"context"
	"errors"
)

var (
	ErrInvalidInput    = errors.New("AI guidance input is invalid")
	ErrInvalidResponse = errors.New("AI model returned an invalid response")
	ErrRateLimited     = errors.New("AI model is rate limited")
	ErrUnavailable     = errors.New("AI model is unavailable")
	ErrNotApplicable   = errors.New("AI guidance is not applicable")
)

type GuidanceKind string

const (
	GuidancePactCreation        GuidanceKind = "PACT_CREATION"
	GuidancePactProgress        GuidanceKind = "PACT_PROGRESS"
	GuidanceAccountabilityBrief GuidanceKind = "ACCOUNTABILITY_BRIEF"
	GuidanceTransactionPreview  GuidanceKind = "TRANSACTION_PREVIEW"
)

type Relationship string

const (
	RelationshipAligned     Relationship = "ALIGNED"
	RelationshipConflicting Relationship = "CONFLICTING"
	RelationshipExceptional Relationship = "EXCEPTIONAL"
	RelationshipUnclear     Relationship = "UNCLEAR"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "HIGH"
	ConfidenceMedium Confidence = "MEDIUM"
	ConfidenceLow    Confidence = "LOW"
)

type ReasonPattern struct {
	Reason      string `json:"reason"`
	Count       int    `json:"count"`
	AmountMinor int64  `json:"amountMinor"`
}

type SpendingPattern struct {
	RecentSpentMinor     int64           `json:"recentSpentMinor"`
	RecentCount          int             `json:"recentCount"`
	PreviousSpentMinor   int64           `json:"previousSpentMinor"`
	PreviousCount        int             `json:"previousCount"`
	Change               string          `json:"change"`
	PeakDay              string          `json:"peakDay,omitempty"`
	PeakTime             string          `json:"peakTime,omitempty"`
	FrequentReasons      []ReasonPattern `json:"frequentReasons,omitempty"`
	HasSufficientHistory bool            `json:"hasSufficientHistory"`
	AlignedCount         int             `json:"alignedCount"`
	ConflictingCount     int             `json:"conflictingCount"`
	ExceptionalCount     int             `json:"exceptionalCount"`
	UnclearCount         int             `json:"unclearCount"`
}

type Tone string

const (
	ToneNeutral  Tone = "NEUTRAL"
	TonePositive Tone = "POSITIVE"
	ToneCaution  Tone = "CAUTION"
	ToneCritical Tone = "CRITICAL"
)

type ModelInput struct {
	Kind                    GuidanceKind     `json:"kind"`
	Goal                    string           `json:"goal,omitempty"`
	Intent                  string           `json:"intent,omitempty"`
	Reason                  string           `json:"reason,omitempty"`
	RequesterName           string           `json:"requesterName,omitempty"`
	OperationType           string           `json:"operationType,omitempty"`
	PactStatus              string           `json:"pactStatus,omitempty"`
	AvailableMinor          int64            `json:"availableMinor"`
	SpentThirtyDaysMinor    int64            `json:"spentThirtyDaysMinor"`
	OutgoingThirtyDaysCount int              `json:"outgoingThirtyDaysCount"`
	LimitMinor              int64            `json:"limitMinor"`
	UsedMinor               int64            `json:"usedMinor"`
	RemainingMinor          int64            `json:"remainingMinor"`
	OverageMinor            int64            `json:"overageMinor"`
	ProposedMinor           int64            `json:"proposedMinor"`
	DaysRemaining           int              `json:"daysRemaining"`
	ElapsedDays             int              `json:"elapsedDays"`
	ProposedReason          string           `json:"proposedReason,omitempty"`
	IsPurchase              bool             `json:"isPurchase"`
	Relationship            Relationship     `json:"relationship,omitempty"`
	Confidence              Confidence       `json:"confidence,omitempty"`
	ProjectedMinor          int64            `json:"projectedMinor"`
	ForecastWarning         bool             `json:"forecastWarning"`
	SpendingPattern         *SpendingPattern `json:"spendingPattern,omitempty"`
	RejectedResponse        string           `json:"rejectedResponse,omitempty"`
}

type ModelDraft struct {
	Headline              string `json:"headline"`
	Message               string `json:"message"`
	Tone                  Tone   `json:"tone"`
	SuggestedLimitMinor   int64  `json:"suggestedLimitMinor"`
	SuggestedStakeMinor   int64  `json:"suggestedStakeMinor"`
	SuggestedDurationDays int    `json:"suggestedDurationDays"`
}

type Model interface {
	Draft(context.Context, ModelInput) (ModelDraft, error)
}

type ClassificationInput struct {
	Goal       string `json:"goal"`
	Intent     string `json:"intent"`
	Reason     string `json:"reason"`
	IsPurchase bool   `json:"isPurchase"`
}

type Classification struct {
	Relationship Relationship `json:"relationship"`
	Confidence   Confidence   `json:"confidence"`
	Reason       string       `json:"reason"`
}

// TransactionClassifier is optional so deterministic guidance and existing
// Model implementations continue to work when semantic classification is not
// available.
type TransactionClassifier interface {
	ClassifyTransaction(context.Context, ClassificationInput) (Classification, error)
}
