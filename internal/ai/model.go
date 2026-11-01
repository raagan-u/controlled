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
)

type GuidanceKind string

const (
	GuidancePactCreation        GuidanceKind = "PACT_CREATION"
	GuidancePactProgress        GuidanceKind = "PACT_PROGRESS"
	GuidanceAccountabilityBrief GuidanceKind = "ACCOUNTABILITY_BRIEF"
)

type Tone string

const (
	ToneNeutral  Tone = "NEUTRAL"
	TonePositive Tone = "POSITIVE"
	ToneCaution  Tone = "CAUTION"
	ToneCritical Tone = "CRITICAL"
)

type ModelInput struct {
	Kind                    GuidanceKind `json:"kind"`
	Goal                    string       `json:"goal,omitempty"`
	Intent                  string       `json:"intent,omitempty"`
	Reason                  string       `json:"reason,omitempty"`
	OperationType           string       `json:"operationType,omitempty"`
	PactStatus              string       `json:"pactStatus,omitempty"`
	AvailableMinor          int64        `json:"availableMinor"`
	SpentThirtyDaysMinor    int64        `json:"spentThirtyDaysMinor"`
	OutgoingThirtyDaysCount int          `json:"outgoingThirtyDaysCount"`
	LimitMinor              int64        `json:"limitMinor"`
	UsedMinor               int64        `json:"usedMinor"`
	RemainingMinor          int64        `json:"remainingMinor"`
	OverageMinor            int64        `json:"overageMinor"`
	ProposedMinor           int64        `json:"proposedMinor"`
	DaysRemaining           int          `json:"daysRemaining"`
	RejectedResponse        string       `json:"rejectedResponse,omitempty"`
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
