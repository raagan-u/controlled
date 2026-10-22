package wallet_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/raagan-u/pay-pact/internal/identity"
	"github.com/raagan-u/pay-pact/internal/ledger"
	"github.com/raagan-u/pay-pact/internal/wallet"
)

func TestNewWallet(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.October, 22, 10, 0, 0, 0, time.UTC)
	accountSet := wallet.AccountSet{
		Available:         "available-1",
		Pending:           "pending-1",
		CommitmentReserve: "reserve-1",
	}
	created, err := wallet.NewWallet("wallet-1", "user-1", "USD", accountSet, createdAt)
	if err != nil {
		t.Fatalf("NewWallet() error = %v", err)
	}
	if created.OwnerID() != identity.UserID("user-1") || created.Accounts() != accountSet {
		t.Fatalf("NewWallet() = %#v", created)
	}
}

func TestNewWalletRequiresDistinctAccounts(t *testing.T) {
	t.Parallel()

	_, err := wallet.NewWallet("wallet-1", "user-1", "USD", wallet.AccountSet{
		Available:         "same-account",
		Pending:           "same-account",
		CommitmentReserve: "reserve-1",
	}, time.Now())
	if !errors.Is(err, wallet.ErrInvalidAccountSet) {
		t.Fatalf("NewWallet() error = %v, want %v", err, wallet.ErrInvalidAccountSet)
	}
}

func TestOnboardingCreatesWalletAndLedgerAccounts(t *testing.T) {
	t.Parallel()

	service, store := newOnboardingService()
	result, err := service.Onboard(t.Context(), "onboard-1", wallet.OnboardingRequest{
		Email:       " Founder@Example.com ",
		DisplayName: "Raagan",
		Currency:    "usd",
	})
	if err != nil {
		t.Fatalf("Onboard() error = %v", err)
	}
	if result.Duplicate {
		t.Fatal("first Onboard() was marked duplicate")
	}

	provisioning := result.Provisioning
	if provisioning.User().Email() != "founder@example.com" {
		t.Fatalf("user email = %q", provisioning.User().Email())
	}
	if provisioning.Wallet().OwnerID() != provisioning.User().ID() {
		t.Fatal("wallet owner does not match user")
	}
	assertAccountKinds(t, provisioning.Accounts())

	stored, err := store.ByUser(t.Context(), provisioning.User().ID())
	if err != nil {
		t.Fatalf("ByUser() error = %v", err)
	}
	if stored.Wallet().ID() != provisioning.Wallet().ID() {
		t.Fatalf("stored wallet = %s, want %s", stored.Wallet().ID(), provisioning.Wallet().ID())
	}
}

func TestOnboardingIsIdempotent(t *testing.T) {
	t.Parallel()

	service, _ := newOnboardingService()
	request := wallet.OnboardingRequest{Email: "founder@example.com", DisplayName: "Raagan", Currency: "USD"}
	first, err := service.Onboard(t.Context(), "onboard-1", request)
	if err != nil {
		t.Fatalf("first Onboard() error = %v", err)
	}
	second, err := service.Onboard(t.Context(), "onboard-1", request)
	if err != nil {
		t.Fatalf("retry Onboard() error = %v", err)
	}
	if !second.Duplicate || second.Provisioning.Wallet().ID() != first.Provisioning.Wallet().ID() {
		t.Fatalf("retry result = %#v", second)
	}

	changed := request
	changed.DisplayName = "Someone Else"
	if _, err := service.Onboard(t.Context(), "onboard-1", changed); !errors.Is(err, wallet.ErrIdempotencyConflict) {
		t.Fatalf("changed retry error = %v, want %v", err, wallet.ErrIdempotencyConflict)
	}
}

func TestOnboardingRejectsDuplicateEmail(t *testing.T) {
	t.Parallel()

	service, _ := newOnboardingService()
	request := wallet.OnboardingRequest{Email: "founder@example.com", DisplayName: "Raagan", Currency: "USD"}
	if _, err := service.Onboard(t.Context(), "onboard-1", request); err != nil {
		t.Fatalf("first Onboard() error = %v", err)
	}
	if _, err := service.Onboard(t.Context(), "onboard-2", request); !errors.Is(err, wallet.ErrUserExists) {
		t.Fatalf("duplicate email error = %v, want %v", err, wallet.ErrUserExists)
	}
}

func TestOnboardingPropagatesIDFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("entropy unavailable")
	service := wallet.NewOnboardingService(
		failingIDs{err: wantErr},
		fixedClock{now: time.Now()},
		wallet.NewMemoryProvisioningStore(),
	)
	_, err := service.Onboard(t.Context(), "onboard-1", wallet.OnboardingRequest{
		Email:       "founder@example.com",
		DisplayName: "Raagan",
		Currency:    "USD",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Onboard() error = %v, want %v", err, wantErr)
	}
}

func assertAccountKinds(t *testing.T, accounts []ledger.Account) {
	t.Helper()

	want := map[ledger.AccountKind]bool{
		ledger.AccountKindAvailable:         true,
		ledger.AccountKindPending:           true,
		ledger.AccountKindCommitmentReserve: true,
	}
	if len(accounts) != len(want) {
		t.Fatalf("account count = %d, want %d", len(accounts), len(want))
	}
	for _, account := range accounts {
		if !want[account.Kind()] || account.NormalSide() != ledger.SideCredit || account.Currency() != "USD" {
			t.Fatalf("unexpected account = %#v", account)
		}
		delete(want, account.Kind())
	}
}

func newOnboardingService() (*wallet.OnboardingService, *wallet.MemoryProvisioningStore) {
	store := wallet.NewMemoryProvisioningStore()
	service := wallet.NewOnboardingService(
		&sequenceIDs{},
		fixedClock{now: time.Date(2026, time.October, 22, 10, 0, 0, 0, time.UTC)},
		store,
	)
	return service, store
}

type sequenceIDs struct {
	next int
}

func (s *sequenceIDs) NewID() (string, error) {
	s.next++
	return "id-" + strconv.Itoa(s.next), nil
}

type fixedClock struct {
	now time.Time
}

type failingIDs struct {
	err error
}

func (f failingIDs) NewID() (string, error) {
	return "", f.err
}

func (c fixedClock) Now() time.Time {
	return c.now
}
