package ai

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
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
	Pacts(context.Context, pact.UserID) ([]*pact.Pact, error)
	PactProgress(context.Context, pact.UserID, pact.PactID) (pact.Progress, error)
}

type AccountabilityReader interface {
	Request(context.Context, identity.UserID, string) (accountability.Request, error)
}

type IdentityReader interface {
	UserByID(context.Context, identity.UserID) (identity.User, error)
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

type TransactionAssessment struct {
	ID                string          `json:"id,omitempty"`
	Relationship      Relationship    `json:"relationship"`
	Confidence        Confidence      `json:"confidence"`
	ClassifierVersion string          `json:"classifierVersion"`
	PrimaryPactID     string          `json:"primaryPactId,omitempty"`
	Pattern           SpendingPattern `json:"pattern"`
	ForecastWarning   bool            `json:"forecastWarning"`
	ProjectedMinor    int64           `json:"projectedMinor,omitempty"`
	Guidance          Guidance        `json:"guidance"`
}

type Service struct {
	clock          Clock
	wallets        WalletReader
	pacts          PactReader
	accountability AccountabilityReader
	identities     IdentityReader
	model          Model
	assessmentIDs  interface{ NewID() (string, error) }
	assessments    AssessmentStore
}

func NewService(clock Clock, wallets WalletReader, pacts PactReader, accountability AccountabilityReader, identities IdentityReader, model Model) *Service {
	return &Service{clock: clock, wallets: wallets, pacts: pacts, accountability: accountability, identities: identities, model: model}
}

func (s *Service) EnableAssessmentPersistence(ids interface{ NewID() (string, error) }, store AssessmentStore) *Service {
	if s != nil {
		s.assessmentIDs, s.assessments = ids, store
	}
	return s
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
	if s.identities == nil {
		return Guidance{}, ErrUnavailable
	}
	requester, err := s.identities.UserByID(ctx, request.OwnerID)
	if err != nil {
		return Guidance{}, err
	}
	requesterName := requesterReference(requester.DisplayName())
	input := ModelInput{Kind: GuidanceAccountabilityBrief, RequesterName: requesterName, Reason: request.Reason, OperationType: string(request.OperationType), ProposedMinor: request.AmountMinor}
	facts := []Fact{{Label: "Requested", Value: usd(request.AmountMinor)}, {Label: "Requester", Value: requester.DisplayName()}}
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

func (s *Service) AssessTransaction(ctx context.Context, userID identity.UserID, amountMinor int64, remark string, isPurchase bool) (TransactionAssessment, error) {
	remark = strings.TrimSpace(remark)
	if userID == "" || amountMinor <= 0 || !MeaningfulRemark(remark) || s == nil || s.wallets == nil || s.pacts == nil {
		return TransactionAssessment{}, ErrInvalidInput
	}
	transactions, err := s.wallets.TransactionsByUser(ctx, userID, 100)
	if err != nil {
		return TransactionAssessment{}, err
	}
	pattern := summarizeSpending(transactions, s.now())
	primary, progress, err := s.primaryPact(ctx, userID)
	if err != nil {
		return TransactionAssessment{}, err
	}
	input := ModelInput{Kind: GuidanceTransactionPreview, ProposedMinor: amountMinor, ProposedReason: remark, IsPurchase: isPurchase, SpendingPattern: &pattern}
	facts := []Fact{{Label: "Last thirty days", Value: usd(pattern.RecentSpentMinor + pattern.PreviousSpentMinor)}, {Label: "Payments counted", Value: fmt.Sprintf("%d", pattern.RecentCount+pattern.PreviousCount)}}
	classification := Classification{Relationship: RelationshipUnclear, Confidence: ConfidenceLow, Reason: "There is not enough pact context to classify this payment."}
	primaryID := ""
	if primary != nil {
		primaryID = string(primary.ID())
		terms := primary.Terms()
		input.Goal, input.Intent = terms.Goal(), terms.Intent()
		input.LimitMinor, input.UsedMinor = terms.SpendingLimit().AmountMinor(), progress.UsedMinor
		input.RemainingMinor, input.OverageMinor = progress.RemainingMinor, progress.OverageMinor
		input.PactStatus = string(primary.Status())
		input.ElapsedDays = elapsedDays(terms.StartsAt(), s.now())
		input.DaysRemaining = remainingDays(terms.EndsAt(), s.now())
		classification = s.classify(ctx, ClassificationInput{Goal: input.Goal, Intent: input.Intent, Reason: remark, IsPurchase: isPurchase})
		input.Relationship, input.Confidence = classification.Relationship, classification.Confidence
		input.ProjectedMinor, input.ForecastWarning = forecast(input.UsedMinor, amountMinor, input.LimitMinor, input.ElapsedDays, input.DaysRemaining)
		facts = append(facts, Fact{Label: "Pact used after payment", Value: percent(input.UsedMinor+amountMinor, input.LimitMinor)})
		if input.ElapsedDays >= 3 {
			facts = append(facts, Fact{Label: "Projected at pact end", Value: forecastLabel(input.ProjectedMinor, input.LimitMinor)})
		}
	}
	guidance := s.generate(ctx, input, facts, transactionFallback(input, pattern.HasSufficientHistory))
	assessment := TransactionAssessment{
		Relationship: classification.Relationship, Confidence: classification.Confidence,
		ClassifierVersion: "transaction-relationship-v1", PrimaryPactID: primaryID,
		Pattern: pattern, ForecastWarning: input.ForecastWarning, ProjectedMinor: input.ProjectedMinor, Guidance: guidance,
	}
	if s.assessmentIDs != nil && s.assessments != nil {
		id, idErr := s.assessmentIDs.NewID()
		if idErr == nil {
			createdAt := s.now()
			stored := StoredAssessment{
				ID: id, OwnerID: userID, PrimaryPactID: primaryID, AmountMinor: amountMinor, Remark: remark, IsPurchase: isPurchase,
				Relationship: assessment.Relationship, Confidence: assessment.Confidence, ClassifierVersion: assessment.ClassifierVersion,
				CreatedAt: createdAt, ExpiresAt: createdAt.Add(15 * time.Minute),
			}
			if saveErr := s.assessments.SaveAssessment(ctx, stored); saveErr == nil {
				assessment.ID = id
			}
		}
	}
	return assessment, nil
}

func MeaningfulRemark(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 300 {
		return false
	}
	lower := strings.ToLower(value)
	for _, noise := range []string{"asdf", "qwer", "zxcv", "hjkl", "1234"} {
		if strings.Contains(lower, noise) {
			return false
		}
	}
	letters := 0
	var previous rune
	repeated := 0
	for _, character := range lower {
		if character >= 'a' && character <= 'z' {
			letters++
			if character == previous {
				repeated++
			} else {
				repeated = 1
			}
			if repeated >= 4 {
				return false
			}
		}
		previous = character
	}
	return letters >= 2
}

func (s *Service) classify(ctx context.Context, input ClassificationInput) Classification {
	classifier, ok := s.model.(TransactionClassifier)
	if !ok {
		return Classification{Relationship: RelationshipUnclear, Confidence: ConfidenceLow, Reason: "Semantic classification is unavailable."}
	}
	result, err := classifier.ClassifyTransaction(ctx, input)
	if err != nil || !validClassification(result) {
		return Classification{Relationship: RelationshipUnclear, Confidence: ConfidenceLow, Reason: "Semantic classification is unavailable."}
	}
	if result.Confidence != ConfidenceHigh {
		result.Relationship = RelationshipUnclear
	}
	return result
}

func validClassification(value Classification) bool {
	validRelationship := value.Relationship == RelationshipAligned || value.Relationship == RelationshipConflicting || value.Relationship == RelationshipExceptional || value.Relationship == RelationshipUnclear
	validConfidence := value.Confidence == ConfidenceHigh || value.Confidence == ConfidenceMedium || value.Confidence == ConfidenceLow
	return validRelationship && validConfidence && strings.TrimSpace(value.Reason) != "" && len(value.Reason) <= 180
}

func (s *Service) primaryPact(ctx context.Context, userID identity.UserID) (*pact.Pact, pact.Progress, error) {
	values, err := s.pacts.Pacts(ctx, pact.UserID(userID))
	if err != nil {
		return nil, pact.Progress{}, err
	}
	var primary *pact.Pact
	for _, candidate := range values {
		if candidate == nil || candidate.Status() != pact.StatusActive {
			continue
		}
		if primary == nil || moreImportantPact(candidate, primary) {
			primary = candidate
		}
	}
	if primary == nil {
		return nil, pact.Progress{}, nil
	}
	progress, err := s.pacts.PactProgress(ctx, pact.UserID(userID), primary.ID())
	return primary, progress, err
}

func moreImportantPact(left, right *pact.Pact) bool {
	l, r := left.Terms(), right.Terms()
	if l.Stake().AmountMinor() != r.Stake().AmountMinor() {
		return l.Stake().AmountMinor() > r.Stake().AmountMinor()
	}
	if l.SpendingLimit().AmountMinor() != r.SpendingLimit().AmountMinor() {
		return l.SpendingLimit().AmountMinor() > r.SpendingLimit().AmountMinor()
	}
	return l.EndsAt().Sub(l.StartsAt()) > r.EndsAt().Sub(r.StartsAt())
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
		if input.Kind == GuidanceAccountabilityBrief {
			requiredPrefix := strings.ToLower(input.RequesterName + " raised this request.")
			if !strings.HasPrefix(strings.ToLower(draft.Message), requiredPrefix) {
				return fmt.Errorf("%w: accountability brief must identify who raised the request", ErrInvalidResponse)
			}
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

func accountabilityFallback(input ModelInput) ModelDraft {
	return ModelDraft{Headline: "Review the trade-off", Message: fmt.Sprintf("%s raised this request. Review their reason against the pact they chose to protect, then decide whether the exception fits.", input.RequesterName), Tone: ToneCaution}
}

func summarizeSpending(transactions []wallet.Transaction, now time.Time) SpendingPattern {
	cutoff, midpoint := now.Add(-30*24*time.Hour), now.Add(-15*24*time.Hour)
	pattern := SpendingPattern{FrequentReasons: []ReasonPattern{}}
	type totals struct {
		count  int
		amount int64
	}
	reasons := map[string]totals{}
	days, times := map[string]int64{}, map[string]int64{}
	for _, transaction := range transactions {
		if transaction.Reversed || transaction.OccurredAt.Before(cutoff) || !isSpendingReference(transaction.Reference.Type) {
			continue
		}
		amount := outgoingAmount(transaction)
		if amount <= 0 {
			continue
		}
		if transaction.OccurredAt.Before(midpoint) {
			pattern.PreviousSpentMinor += amount
			pattern.PreviousCount++
		} else {
			pattern.RecentSpentMinor += amount
			pattern.RecentCount++
		}
		days[transaction.OccurredAt.Weekday().String()] += amount
		times[dayPart(transaction.OccurredAt)] += amount
		reason := transactionReason(transaction)
		if reason != "" && MeaningfulRemark(reason) {
			key := strings.ToLower(strings.TrimSpace(reason))
			value := reasons[key]
			value.count++
			value.amount += amount
			reasons[key] = value
		}
	}
	pattern.Change = spendingChange(pattern.RecentSpentMinor, pattern.PreviousSpentMinor)
	pattern.PeakDay, pattern.PeakTime = largestBucket(days), largestBucket(times)
	for reason, value := range reasons {
		pattern.FrequentReasons = append(pattern.FrequentReasons, ReasonPattern{Reason: reason, Count: value.count, AmountMinor: value.amount})
	}
	sort.Slice(pattern.FrequentReasons, func(i, j int) bool {
		return pattern.FrequentReasons[i].AmountMinor > pattern.FrequentReasons[j].AmountMinor
	})
	if len(pattern.FrequentReasons) > 3 {
		pattern.FrequentReasons = pattern.FrequentReasons[:3]
	}
	pattern.HasSufficientHistory = pattern.RecentCount+pattern.PreviousCount >= 3
	return pattern
}

func outgoingAmount(transaction wallet.Transaction) int64 {
	for _, entry := range transaction.Entries {
		if entry.AccountKind == ledger.AccountKindAvailable && entry.Side == ledger.SideDebit {
			return entry.Amount.AmountMinor()
		}
	}
	return 0
}

func transactionReason(transaction wallet.Transaction) string {
	if transaction.Transfer != nil {
		return transaction.Transfer.Remark
	}
	if transaction.Approval != nil {
		return transaction.Approval.Reason
	}
	return ""
}

func dayPart(value time.Time) string {
	switch hour := value.Hour(); {
	case hour < 6:
		return "overnight"
	case hour < 12:
		return "morning"
	case hour < 17:
		return "afternoon"
	case hour < 22:
		return "evening"
	default:
		return "late night"
	}
}

func largestBucket(values map[string]int64) string {
	name := ""
	var largest int64
	for candidate, value := range values {
		if value > largest || value == largest && candidate < name {
			name, largest = candidate, value
		}
	}
	return name
}

func spendingChange(current, previous int64) string {
	if previous == 0 {
		if current == 0 {
			return "STABLE"
		}
		return "NEW_ACTIVITY"
	}
	ratio := float64(current-previous) / float64(previous)
	if ratio >= .2 {
		return "INCREASING"
	}
	if ratio <= -.2 {
		return "DECREASING"
	}
	return "STABLE"
}

func elapsedDays(start, now time.Time) int {
	if !now.After(start) {
		return 0
	}
	return int(math.Floor(now.Sub(start).Hours()/24)) + 1
}

func remainingDays(end, now time.Time) int {
	days := int(math.Ceil(end.Sub(now).Hours() / 24))
	if days < 0 {
		return 0
	}
	return days
}

func forecast(used, proposed, limit int64, elapsed, remaining int) (int64, bool) {
	usedAfter := used + proposed
	if elapsed < 3 || limit <= 0 {
		return 0, false
	}
	projected := usedAfter + int64(math.Round(float64(used)/float64(elapsed)*float64(remaining)))
	return projected, usedAfter*2 >= limit && projected > limit
}

func percent(value, total int64) string {
	if total <= 0 {
		return "—"
	}
	return fmt.Sprintf("%d%%", int(math.Round(float64(value)*100/float64(total))))
}

func forecastLabel(projected, limit int64) string {
	if projected > limit {
		return "Above limit"
	}
	return "Within limit"
}

func transactionFallback(input ModelInput, sufficientHistory bool) ModelDraft {
	switch {
	case input.ForecastWarning:
		return ModelDraft{Headline: "Protect the room ahead", Message: "This payment may make the rest of your pact harder to sustain. Pause and compare it with the goal you chose.", Tone: ToneCaution}
	case input.Relationship == RelationshipConflicting:
		return ModelDraft{Headline: "Pause on the trade-off", Message: "This reason may work against the goal you chose. Consider whether the payment is worth reducing the room left in your pact.", Tone: ToneCaution}
	case input.Relationship == RelationshipAligned:
		return ModelDraft{Headline: "Connected to your goal", Message: "This reason appears consistent with what you chose to protect, while the payment still counts toward your limit.", Tone: TonePositive}
	case input.Relationship == RelationshipExceptional:
		return ModelDraft{Headline: "An exception worth noticing", Message: "This looks different from your usual pattern. Consider it deliberately while keeping the rest of your pact in view.", Tone: ToneNeutral}
	case !sufficientHistory:
		return ModelDraft{Headline: "Not enough history yet", Message: "There is not enough recent activity to identify a reliable pattern. Consider whether this payment supports the reason you created the pact.", Tone: ToneNeutral}
	default:
		return ModelDraft{Headline: "Check this against your goal", Message: "The reason is not clear enough to connect confidently with your pact. Take a moment to compare the payment with what you chose to protect.", Tone: ToneNeutral}
	}
}

func requesterReference(displayName string) string {
	value := strings.TrimSpace(displayName)
	if value == "" || containsFinancialLiteral.MatchString(value) {
		return "the requester"
	}
	return value
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
