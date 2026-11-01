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
	progress pact.Progress
	err      error
}

func (s pactReaderStub) Pact(context.Context, pact.UserID, pact.PactID) (*pact.Pact, error) {
	return s.value, s.err
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
	drafts []ModelDraft
	errors []error
	inputs []ModelInput
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
