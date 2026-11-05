package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/accountability"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/money"
	"github.com/raagan-u/controlled/internal/pact"
	"github.com/raagan-u/controlled/internal/wallet"
)

var testNow = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

type walletReaderStub struct {
	snapshot     wallet.Snapshot
	transactions []wallet.Transaction
	err          error
}

func (s walletReaderStub) WalletByUser(context.Context, identity.UserID) (wallet.Snapshot, error) {
	return s.snapshot, s.err
}
func (s walletReaderStub) TransactionsByUser(context.Context, identity.UserID, int) ([]wallet.Transaction, error) {
	return s.transactions, s.err
}

type pactReaderStub struct {
	value    *pact.Pact
	values   []*pact.Pact
	progress pact.Progress
	err      error
}

func (s pactReaderStub) Pact(context.Context, pact.UserID, pact.PactID) (*pact.Pact, error) {
	return s.value, s.err
}
func (s pactReaderStub) Pacts(context.Context, pact.UserID) ([]*pact.Pact, error) {
	return s.values, s.err
}
func (s pactReaderStub) PactProgress(context.Context, pact.UserID, pact.PactID) (pact.Progress, error) {
	return s.progress, s.err
}

type accountabilityReaderStub struct {
	value accountability.Request
	err   error
}

type identityReaderStub struct {
	value identity.User
	err   error
}

func (s identityReaderStub) UserByID(context.Context, identity.UserID) (identity.User, error) {
	return s.value, s.err
}

func (s accountabilityReaderStub) Request(context.Context, identity.UserID, string) (accountability.Request, error) {
	return s.value, s.err
}

type modelStub struct {
	drafts          []ModelDraft
	errors          []error
	inputs          []ModelInput
	classifications []Classification
	classifyErr     error
}

type idStub struct{ value string }

func (s idStub) NewID() (string, error) { return s.value, nil }

type assessmentStoreStub struct{ saved StoredAssessment }

func (s *assessmentStoreStub) SaveAssessment(_ context.Context, value StoredAssessment) error {
	s.saved = value
	return nil
}

func (s *modelStub) Draft(_ context.Context, input ModelInput) (ModelDraft, error) {
	s.inputs = append(s.inputs, input)
	index := len(s.inputs) - 1
	var err error
	if index < len(s.errors) {
		err = s.errors[index]
	}
	if index < len(s.drafts) {
		return s.drafts[index], err
	}
	return ModelDraft{}, err
}

func (s *modelStub) ClassifyTransaction(_ context.Context, _ ClassificationInput) (Classification, error) {
	if len(s.classifications) == 0 {
		return Classification{}, s.classifyErr
	}
	return s.classifications[0], s.classifyErr
}

func TestSuggestPactUsesDeterministicFallbackWithoutProvider(t *testing.T) {
	t.Parallel()
	snapshot := testWalletSnapshot(t, 20_000)
	service := NewService(fixedClock{testNow}, walletReaderStub{snapshot: snapshot, transactions: []wallet.Transaction{
		testSpendingTransaction(t, 8_000, testNow.Add(-24*time.Hour)),
		testSpendingTransaction(t, 2_000, testNow.Add(-31*24*time.Hour)),
	}}, nil, nil, nil, nil)

	guidance, err := service.SuggestPact(context.Background(), "user-1", "Spend with intention", "Save for a trip")
	if err != nil {
		t.Fatalf("SuggestPact() error = %v", err)
	}
	if guidance.Source != SourceRules || guidance.SuggestedTerms == nil {
		t.Fatalf("guidance = %#v", guidance)
	}
	if guidance.SuggestedTerms.LimitMinor != 6_500 || guidance.SuggestedTerms.StakeMinor != 1_500 {
		t.Fatalf("suggested terms = %#v", guidance.SuggestedTerms)
	}
	if guidance.Facts[1].Value != "$80.00" || guidance.Facts[2].Value != "1" {
		t.Fatalf("facts = %#v", guidance.Facts)
	}
}

func TestSuggestPactRepairsInvalidModelOutputOnce(t *testing.T) {
	t.Parallel()
	model := &modelStub{drafts: []ModelDraft{
		{Headline: "Save $20", Message: "Try it", Tone: ToneNeutral, SuggestedLimitMinor: 5_000, SuggestedStakeMinor: 1_000, SuggestedDurationDays: 7},
		{Headline: "Build a steady boundary", Message: "This starting point protects the goal without making ordinary spending impossible.", Tone: TonePositive, SuggestedLimitMinor: 5_000, SuggestedStakeMinor: 1_000, SuggestedDurationDays: 7},
	}}
	service := NewService(fixedClock{testNow}, walletReaderStub{snapshot: testWalletSnapshot(t, 10_000)}, nil, nil, nil, model)

	guidance, err := service.SuggestPact(context.Background(), "user-1", "Weekend boundary", "Spend more deliberately")
	if err != nil {
		t.Fatalf("SuggestPact() error = %v", err)
	}
	if guidance.Source != SourceAI || guidance.Headline != "Build a steady boundary" {
		t.Fatalf("guidance = %#v", guidance)
	}
	if len(model.inputs) != 2 || model.inputs[1].RejectedResponse == "" {
		t.Fatalf("model inputs = %#v", model.inputs)
	}
}

func TestPactInsightKeepsVerifiedNumbersOutOfModelNarrative(t *testing.T) {
	t.Parallel()
	limit, _ := pact.NewMoney(10_000, "USD")
	stake, _ := pact.NewMoney(2_000, "USD")
	terms, _ := pact.NewTerms("Keep dining intentional", "Cook more often", limit, stake, testNow.Add(-time.Hour), testNow.Add(48*time.Hour), "circle-1", pact.BreakConsequenceCommitmentReserve)
	value, err := pact.Restore("pact-1", "user-1", terms, pact.StatusActive, pact.OutcomeNone, "stake-1", 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	model := &modelStub{drafts: []ModelDraft{{Headline: "Protect the remaining room", Message: "The next payment deserves an intentional pause.", Tone: ToneCaution}}}
	service := NewService(fixedClock{testNow}, nil, pactReaderStub{value: value, progress: pact.Progress{UsedMinor: 8_500, RemainingMinor: 1_500}}, nil, nil, model)

	guidance, err := service.PactInsight(context.Background(), "user-1", "pact-1")
	if err != nil {
		t.Fatalf("PactInsight() error = %v", err)
	}
	if guidance.Source != SourceAI || guidance.Facts[0].Value != "$85.00" || guidance.Facts[2].Value != "2 days" {
		t.Fatalf("guidance = %#v", guidance)
	}
	if guidance.SuggestedTerms != nil {
		t.Fatalf("unexpected terms = %#v", guidance.SuggestedTerms)
	}
}

func TestAccountabilityBriefFallsBackWhenProviderUnavailable(t *testing.T) {
	t.Parallel()
	model := &modelStub{errors: []error{ErrRateLimited}}
	request := accountability.Request{ID: "request-1", OwnerID: "owner-1", AmountMinor: 2_500, Reason: "Unexpected travel", OperationType: "INTERNAL_TRANSFER", Pacts: []accountability.RequestPact{{Goal: "Travel fund", Intent: "Keep room for planned travel", LimitMinor: 10_000, UsedMinor: 9_000, OverageMinor: 1_500}}}
	requester, _ := identity.NewUser("owner-1", "tarnished@example.test", "Tarnished", testNow.Add(-time.Hour))
	service := NewService(fixedClock{testNow}, nil, nil, accountabilityReaderStub{value: request}, identityReaderStub{value: requester}, model)

	guidance, err := service.AccountabilityBrief(context.Background(), "partner-1", "request-1")
	if err != nil {
		t.Fatalf("AccountabilityBrief() error = %v", err)
	}
	if guidance.Source != SourceRules || guidance.Facts[1].Value != "Tarnished" || guidance.Facts[3].Value != "$15.00" || !strings.Contains(guidance.Message, "Tarnished") {
		t.Fatalf("guidance = %#v", guidance)
	}
	if len(model.inputs) != 1 || model.inputs[0].RequesterName != "Tarnished" {
		t.Fatalf("model inputs = %#v", model.inputs)
	}
}

func TestAccountabilityBriefRepairsOutputThatDoesNotNameRequester(t *testing.T) {
	t.Parallel()
	request := accountability.Request{ID: "request-1", OwnerID: "owner-1", AmountMinor: 2_500, Reason: "Unexpected travel", OperationType: "INTERNAL_TRANSFER"}
	requester, _ := identity.NewUser("owner-1", "tarnished@example.test", "Tarnished", testNow.Add(-time.Hour))
	model := &modelStub{drafts: []ModelDraft{
		{Headline: "Review the request", Message: "Consider whether this exception protects the pact.", Tone: ToneCaution},
		{Headline: "Review Tarnished's request", Message: "Tarnished raised this request. Compare their reason with the pact they chose.", Tone: ToneCaution},
	}}
	service := NewService(fixedClock{testNow}, nil, nil, accountabilityReaderStub{value: request}, identityReaderStub{value: requester}, model)

	guidance, err := service.AccountabilityBrief(context.Background(), "partner-1", "request-1")
	if err != nil {
		t.Fatalf("AccountabilityBrief() error = %v", err)
	}
	if guidance.Source != SourceAI || !strings.Contains(guidance.Message, "Tarnished") || len(model.inputs) != 2 || model.inputs[1].RejectedResponse == "" {
		t.Fatalf("guidance = %#v, inputs = %#v", guidance, model.inputs)
	}
}

func TestSuggestPactRejectsIncompleteInput(t *testing.T) {
	t.Parallel()
	service := NewService(fixedClock{testNow}, nil, nil, nil, nil, nil)
	_, err := service.SuggestPact(context.Background(), "user-1", "", "reason")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v", err)
	}
}

func TestAssessTransactionUsesHighestStakePactAndWarnsOnForecast(t *testing.T) {
	t.Parallel()
	primary := testActivePact(t, "primary", 20_000, 8_000, testNow.Add(-4*24*time.Hour), testNow.Add(6*24*time.Hour))
	secondary := testActivePact(t, "secondary", 30_000, 5_000, testNow.Add(-10*24*time.Hour), testNow.Add(20*24*time.Hour))
	model := &modelStub{
		classifications: []Classification{{Relationship: RelationshipConflicting, Confidence: ConfidenceHigh, Reason: "The reason conflicts with the pact."}},
		drafts:          []ModelDraft{{Headline: "Protect the room ahead", Message: "This payment may make the rest of the pact harder to sustain.", Tone: ToneCaution}},
	}
	service := NewService(fixedClock{testNow}, walletReaderStub{transactions: []wallet.Transaction{
		testSpendingTransactionWithReason(t, 2_000, testNow.Add(-2*time.Hour), "food"),
		testSpendingTransactionWithReason(t, 1_000, testNow.Add(-16*24*time.Hour), "coffee"),
		testSpendingTransactionWithReason(t, 500, testNow.Add(-20*24*time.Hour), "coffee"),
	}}, pactReaderStub{values: []*pact.Pact{secondary, primary}, progress: pact.Progress{UsedMinor: 9_000, RemainingMinor: 11_000}}, nil, nil, model)

	assessment, err := service.AssessTransaction(context.Background(), "user-1", 2_000, "expensive dinner", true)
	if err != nil {
		t.Fatalf("AssessTransaction() error = %v", err)
	}
	if assessment.PrimaryPactID != "primary" || !assessment.ForecastWarning || assessment.Relationship != RelationshipConflicting {
		t.Fatalf("assessment = %#v", assessment)
	}
	if assessment.Pattern.RecentCount != 1 || assessment.Pattern.PreviousCount != 2 || assessment.Pattern.FrequentReasons[0].Reason != "food" {
		t.Fatalf("pattern = %#v", assessment.Pattern)
	}
}

func TestAssessTransactionKeepsLowConfidenceClassificationUnclear(t *testing.T) {
	t.Parallel()
	value := testActivePact(t, "pact", 20_000, 4_000, testNow.Add(-24*time.Hour), testNow.Add(10*24*time.Hour))
	model := &modelStub{
		classifications: []Classification{{Relationship: RelationshipAligned, Confidence: ConfidenceMedium, Reason: "The connection is possible."}},
		drafts:          []ModelDraft{{Headline: "Not enough history yet", Message: "There is not enough recent activity to identify a reliable pattern.", Tone: ToneNeutral}},
	}
	service := NewService(fixedClock{testNow}, walletReaderStub{}, pactReaderStub{values: []*pact.Pact{value}}, nil, nil, model)
	assessment, err := service.AssessTransaction(context.Background(), "user-1", 1_000, "food", false)
	if err != nil || assessment.Relationship != RelationshipUnclear || assessment.ForecastWarning {
		t.Fatalf("assessment = %#v, error = %v", assessment, err)
	}
}

func TestAssessTransactionPersistsReusableAssessment(t *testing.T) {
	t.Parallel()
	value := testActivePact(t, "pact", 20_000, 4_000, testNow.Add(-4*24*time.Hour), testNow.Add(10*24*time.Hour))
	store := &assessmentStoreStub{}
	service := NewService(fixedClock{testNow}, walletReaderStub{}, pactReaderStub{values: []*pact.Pact{value}}, nil, nil, nil).
		EnableAssessmentPersistence(idStub{value: "assessment-1"}, store)
	assessment, err := service.AssessTransaction(context.Background(), "user-1", 1_000, "food", true)
	if err != nil || assessment.ID != "assessment-1" {
		t.Fatalf("assessment = %#v, error = %v", assessment, err)
	}
	if store.saved.ID != "assessment-1" || store.saved.OwnerID != "user-1" || !store.saved.IsPurchase || store.saved.ExpiresAt.Sub(store.saved.CreatedAt) != 15*time.Minute {
		t.Fatalf("stored = %#v", store.saved)
	}
}

func TestMeaningfulRemarkRejectsGibberish(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "a", "asdfgh", "heyyyy", "1234"} {
		if MeaningfulRemark(value) {
			t.Errorf("MeaningfulRemark(%q) = true", value)
		}
	}
	for _, value := range []string{"food", "gym", "dinner with friends"} {
		if !MeaningfulRemark(value) {
			t.Errorf("MeaningfulRemark(%q) = false", value)
		}
	}
}

func testWalletSnapshot(t *testing.T, availableMinor int64) wallet.Snapshot {
	t.Helper()
	value, err := wallet.NewWallet("wallet-1", "user-1", "USD", wallet.AccountSet{Available: "available-1", Pending: "pending-1", CommitmentReserve: "reserve-1"}, testNow.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	available, _ := money.New(availableMinor, "USD")
	zero, _ := money.New(0, "USD")
	snapshot, err := wallet.NewSnapshot(value, wallet.BalanceSet{Available: available, Pending: zero, Staked: zero, CommitmentReserve: zero})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func testSpendingTransaction(t *testing.T, amountMinor int64, at time.Time) wallet.Transaction {
	t.Helper()
	amount, err := money.New(amountMinor, "USD")
	if err != nil {
		t.Fatal(err)
	}
	return wallet.Transaction{ID: "transaction", Reference: ledger.Reference{Type: "INTERNAL_TRANSFER", ID: "transfer"}, OccurredAt: at, Entries: []wallet.TransactionEntry{{AccountID: "available-1", AccountKind: ledger.AccountKindAvailable, Side: ledger.SideDebit, Amount: amount}}}
}

func testSpendingTransactionWithReason(t *testing.T, amountMinor int64, at time.Time, reason string) wallet.Transaction {
	transaction := testSpendingTransaction(t, amountMinor, at)
	transaction.Transfer = &wallet.TransferDetails{Remark: reason}
	return transaction
}

func testActivePact(t *testing.T, id string, limitMinor, stakeMinor int64, startsAt, endsAt time.Time) *pact.Pact {
	t.Helper()
	limit, _ := pact.NewMoney(limitMinor, "USD")
	stake, _ := pact.NewMoney(stakeMinor, "USD")
	terms, err := pact.NewTerms("Avoid expensive dinners", "Protect travel savings", limit, stake, startsAt, endsAt, "circle-1", pact.BreakConsequenceCommitmentReserve)
	if err != nil {
		t.Fatal(err)
	}
	value, err := pact.Restore(pact.PactID(id), "user-1", terms, pact.StatusActive, pact.OutcomeNone, "stake-1", 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
