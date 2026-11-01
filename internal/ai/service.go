package ai

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/raagan-u/controlled/internal/accountability"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/pact"
	"github.com/raagan-u/controlled/internal/wallet"
)

const maxModelAttempts = 2

var containsFinancialLiteral = regexp.MustCompile(`[0-9$]`)

type Clock interface{ Now() time.Time }

type WalletReader interface {
	WalletByUser(context.Context, identity.UserID) (wallet.Snapshot, error)
	TransactionsByUser(context.Context, identity.UserID, int) ([]wallet.Transaction, error)
}

type PactReader interface {
	Pact(context.Context, pact.UserID, pact.PactID) (*pact.Pact, error)
	PactProgress(context.Context, pact.UserID, pact.PactID) (pact.Progress, error)
}

type AccountabilityReader interface {
	Request(context.Context, identity.UserID, string) (accountability.Request, error)
}

type Source string

const (
	SourceAI    Source = "AI"
	SourceRules Source = "RULES"
)

type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type SuggestedTerms struct {
	LimitMinor   int64 `json:"limitMinor"`
	StakeMinor   int64 `json:"stakeMinor"`
	DurationDays int   `json:"durationDays"`
}

type Guidance struct {
	Kind           GuidanceKind    `json:"kind"`
	Headline       string          `json:"headline"`
	Message        string          `json:"message"`
	Tone           Tone            `json:"tone"`
	Facts          []Fact          `json:"facts"`
	SuggestedTerms *SuggestedTerms `json:"suggestedTerms,omitempty"`
	Source         Source          `json:"source"`
	GeneratedAt    time.Time       `json:"generatedAt"`
}

type Service struct {
	clock          Clock
	wallets        WalletReader
	pacts          PactReader
	accountability AccountabilityReader
	model          Model
}

func NewService(clock Clock, wallets WalletReader, pacts PactReader, accountability AccountabilityReader, model Model) *Service {
	return &Service{clock: clock, wallets: wallets, pacts: pacts, accountability: accountability, model: model}
}

func (s *Service) SuggestPact(ctx context.Context, userID identity.UserID, goal, intent string) (Guidance, error) {
	goal, intent = strings.TrimSpace(goal), strings.TrimSpace(intent)
	if userID == "" || goal == "" || intent == "" || len(goal) > 120 || len(intent) > 300 {
		return Guidance{}, ErrInvalidInput
	}
	input, facts, err := s.walletContext(ctx, userID)
	if err != nil {
		return Guidance{}, err
	}
	input.Kind, input.Goal, input.Intent = GuidancePactCreation, goal, intent
	fallback := creationFallback(input)
	return s.generate(ctx, input, facts, fallback), nil
}

func (s *Service) PactInsight(ctx context.Context, userID identity.UserID, pactID pact.PactID) (Guidance, error) {
	if userID == "" || pactID == "" || s == nil || s.pacts == nil {
		return Guidance{}, ErrInvalidInput
	}
	value, err := s.pacts.Pact(ctx, pact.UserID(userID), pactID)
	if err != nil {
		return Guidance{}, err
	}
	progress, err := s.pacts.PactProgress(ctx, pact.UserID(userID), pactID)
	if err != nil {
		return Guidance{}, err
	}
	terms := value.Terms()
	days := int(math.Ceil(terms.EndsAt().Sub(s.now()).Hours() / 24))
	if days < 0 {
		days = 0
	}
	input := ModelInput{
		Kind: GuidancePactProgress, Goal: terms.Goal(), Intent: terms.Intent(), PactStatus: string(value.Status()),
		LimitMinor: terms.SpendingLimit().AmountMinor(), UsedMinor: progress.UsedMinor,
		RemainingMinor: progress.RemainingMinor, OverageMinor: progress.OverageMinor, DaysRemaining: days,
	}
	facts := []Fact{{Label: "Spent", Value: usd(progress.UsedMinor)}, {Label: "Remaining", Value: usd(progress.RemainingMinor)}, {Label: "Time left", Value: dayLabel(days)}}
	return s.generate(ctx, input, facts, progressFallback(input)), nil
}

func (s *Service) AccountabilityBrief(ctx context.Context, userID identity.UserID, requestID string) (Guidance, error) {
	if userID == "" || strings.TrimSpace(requestID) == "" || s == nil || s.accountability == nil {
		return Guidance{}, ErrInvalidInput
	}
	request, err := s.accountability.Request(ctx, userID, requestID)
	if err != nil {
		return Guidance{}, err
	}
	input := ModelInput{Kind: GuidanceAccountabilityBrief, Reason: request.Reason, OperationType: string(request.OperationType), ProposedMinor: request.AmountMinor}
	facts := []Fact{{Label: "Requested", Value: usd(request.AmountMinor)}}
	if len(request.Pacts) > 0 {
		blocking := request.Pacts[0]
		for _, candidate := range request.Pacts[1:] {
			if candidate.OverageMinor > blocking.OverageMinor {
				blocking = candidate
			}
		}
		input.Goal, input.Intent = blocking.Goal, blocking.Intent
		input.LimitMinor, input.UsedMinor, input.OverageMinor = blocking.LimitMinor, blocking.UsedMinor, blocking.OverageMinor
		facts = append(facts, Fact{Label: "Pact used", Value: fmt.Sprintf("%s of %s", usd(blocking.UsedMinor), usd(blocking.LimitMinor))}, Fact{Label: "Would exceed by", Value: usd(blocking.OverageMinor)})
	}
	return s.generate(ctx, input, facts, accountabilityFallback(input)), nil
}

func (s *Service) walletContext(ctx context.Context, userID identity.UserID) (ModelInput, []Fact, error) {
	if s == nil || s.wallets == nil {
		return ModelInput{}, nil, ErrUnavailable
	}
	snapshot, err := s.wallets.WalletByUser(ctx, userID)
	if err != nil {
		return ModelInput{}, nil, err
	}
	transactions, err := s.wallets.TransactionsByUser(ctx, userID, 100)
	if err != nil {
		return ModelInput{}, nil, err
	}
	available := snapshot.Balances().Available.AmountMinor()
	cutoff, availableAccount := s.now().Add(-30*24*time.Hour), snapshot.Wallet().Accounts().Available
	var spent int64
	count := 0
	for _, transaction := range transactions {
		if transaction.Reversed || transaction.OccurredAt.Before(cutoff) || !isSpendingReference(transaction.Reference.Type) {
			continue
		}
		for _, entry := range transaction.Entries {
			if entry.AccountID == availableAccount && entry.Side == ledger.SideDebit {
				spent += entry.Amount.AmountMinor()
				count++
				break
			}
		}
	}
	input := ModelInput{AvailableMinor: available, SpentThirtyDaysMinor: spent, OutgoingThirtyDaysCount: count}
	facts := []Fact{{Label: "Available", Value: usd(available)}, {Label: "Recent outgoing", Value: usd(spent)}, {Label: "Payments counted", Value: fmt.Sprintf("%d", count)}}
	return input, facts, nil
}

func (s *Service) generate(ctx context.Context, input ModelInput, facts []Fact, fallback ModelDraft) Guidance {
	draft, source := fallback, SourceRules
	if s != nil && s.model != nil {
		for attempt := 0; attempt < maxModelAttempts; attempt++ {
			candidate, err := s.model.Draft(ctx, input)
			if err != nil {
				if errors.Is(err, ErrInvalidResponse) && attempt+1 < maxModelAttempts {
					input.RejectedResponse = "The prior response did not satisfy the output constraints. Return a corrected response."
					continue
				}
				break
			}
			if err := validateDraft(input, candidate); err != nil {
				if attempt+1 < maxModelAttempts {
					input.RejectedResponse = err.Error()
					continue
				}
				break
			}
			draft, source = candidate, SourceAI
			break
		}
	}
	guidance := Guidance{Kind: input.Kind, Headline: draft.Headline, Message: draft.Message, Tone: draft.Tone, Facts: facts, Source: source, GeneratedAt: s.now()}
	if input.Kind == GuidancePactCreation {
		guidance.SuggestedTerms = &SuggestedTerms{LimitMinor: draft.SuggestedLimitMinor, StakeMinor: draft.SuggestedStakeMinor, DurationDays: draft.SuggestedDurationDays}
	}
	return guidance
}

func validateDraft(input ModelInput, draft ModelDraft) error {
	draft.Headline, draft.Message = strings.TrimSpace(draft.Headline), strings.TrimSpace(draft.Message)
	if draft.Headline == "" || draft.Message == "" || len(draft.Headline) > 80 || len(draft.Message) > 280 {
		return fmt.Errorf("%w: headline or message length", ErrInvalidResponse)
	}
	if containsFinancialLiteral.MatchString(draft.Headline) || containsFinancialLiteral.MatchString(draft.Message) {
		return fmt.Errorf("%w: narrative must not contain numeric facts", ErrInvalidResponse)
	}
	if draft.Tone != ToneNeutral && draft.Tone != TonePositive && draft.Tone != ToneCaution && draft.Tone != ToneCritical {
		return fmt.Errorf("%w: tone", ErrInvalidResponse)
	}
	if input.Kind != GuidancePactCreation {
		if draft.SuggestedLimitMinor != 0 || draft.SuggestedStakeMinor != 0 || draft.SuggestedDurationDays != 0 {
			return fmt.Errorf("%w: unexpected suggested terms", ErrInvalidResponse)
		}
		return nil
	}
	if draft.SuggestedLimitMinor < 100 || draft.SuggestedLimitMinor > 10_000_000 ||
		draft.SuggestedStakeMinor < 100 || draft.SuggestedStakeMinor > draft.SuggestedLimitMinor ||
		draft.SuggestedDurationDays < 1 || draft.SuggestedDurationDays > 365 {
		return fmt.Errorf("%w: suggested terms outside safe bounds", ErrInvalidResponse)
	}
	return nil
}

func creationFallback(input ModelInput) ModelDraft {
	limit := roundToFive(input.SpentThirtyDaysMinor * 4 / 5)
	if limit < 1000 {
		limit = roundToFive(input.AvailableMinor)
	}
	if limit < 1000 {
		limit = 5000
	}
	stake := roundToFive(limit / 5)
	if input.AvailableMinor > 0 && stake > input.AvailableMinor {
		stake = roundDownToFive(input.AvailableMinor)
	}
	if stake < 100 {
		stake = 500
	}
	if stake > limit {
		stake = limit
	}
	return ModelDraft{Headline: "A sustainable starting point", Message: "Start with a limit that protects your goal while leaving room for ordinary spending.", Tone: ToneNeutral, SuggestedLimitMinor: limit, SuggestedStakeMinor: stake, SuggestedDurationDays: 30}
}

func progressFallback(input ModelInput) ModelDraft {
	ratio := float64(input.UsedMinor) / float64(max64(input.LimitMinor, 1))
	switch {
	case input.OverageMinor > 0:
		return ModelDraft{Headline: "This pact needs attention", Message: "The limit has been crossed, so pause before the next outgoing payment and revisit what this goal protects.", Tone: ToneCritical}
	case ratio >= .8:
		return ModelDraft{Headline: "You are close to the edge", Message: "There is little room left in this pact. Treat the next outgoing payment as a deliberate choice.", Tone: ToneCaution}
	case ratio < .5:
		return ModelDraft{Headline: "Your pact is on track", Message: "You still have healthy room inside the limit. Keep the same deliberate pace.", Tone: TonePositive}
	default:
		return ModelDraft{Headline: "Keep the pace intentional", Message: "The pact still has room, but each outgoing payment now matters more to the goal.", Tone: ToneNeutral}
	}
}

func accountabilityFallback(ModelInput) ModelDraft {
	return ModelDraft{Headline: "Review the trade-off", Message: "Compare the reason with the pact they chose to protect. Approve only if the exception matches the decision you are being asked to make.", Tone: ToneCaution}
}

func (s *Service) now() time.Time {
	if s != nil && s.clock != nil {
		return s.clock.Now().UTC()
	}
	return time.Now().UTC()
}

func isSpendingReference(value string) bool {
	return value == "INTERNAL_TRANSFER" || value == "WITHDRAWAL_RESERVATION"
}

func usd(minor int64) string { return fmt.Sprintf("$%.2f", float64(minor)/100) }
func dayLabel(days int) string {
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}
func roundToFive(value int64) int64     { return ((value + 250) / 500) * 500 }
func roundDownToFive(value int64) int64 { return (value / 500) * 500 }
func max64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
