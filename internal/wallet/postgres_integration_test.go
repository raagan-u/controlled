package wallet_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/pay-pact/internal/platform"
	platformpostgres "github.com/raagan-u/pay-pact/internal/platform/postgres"
	"github.com/raagan-u/pay-pact/internal/wallet"
)

func TestPostgresOnboardingLifecycle(t *testing.T) {
	databaseURL := os.Getenv("PAYPACT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PAYPACT_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	if err := platformpostgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	ids := platform.NewUUIDGenerator()
	emailID, err := ids.NewID()
	if err != nil {
		t.Fatalf("generate email ID: %v", err)
	}
	request := wallet.OnboardingRequest{
		Email:       fmt.Sprintf("wallet-%s@example.test", emailID),
		DisplayName: "Wallet Test",
		Currency:    "USD",
	}
	store := wallet.NewPostgresProvisioningStore(pool)
	service := wallet.NewOnboardingService(
		ids,
		platform.SystemClock{},
		store,
	)

	first, err := service.Onboard(ctx, "test:"+emailID, request)
	if err != nil {
		t.Fatalf("onboard user: %v", err)
	}
	if first.Duplicate || len(first.Provisioning.Accounts()) != 3 {
		t.Fatalf("unexpected first result: %+v", first)
	}

	retry, err := service.Onboard(ctx, "test:"+emailID, request)
	if err != nil {
		t.Fatalf("retry onboarding: %v", err)
	}
	if !retry.Duplicate || retry.Provisioning.Wallet().ID() != first.Provisioning.Wallet().ID() {
		t.Fatalf("unexpected retry result: %+v", retry)
	}

	changed := request
	changed.DisplayName = "Changed Name"
	if _, err := service.Onboard(ctx, "test:"+emailID, changed); !errors.Is(err, wallet.ErrIdempotencyConflict) {
		t.Fatalf("changed retry error = %v, want %v", err, wallet.ErrIdempotencyConflict)
	}
	if _, err := service.Onboard(ctx, "test:duplicate:"+emailID, request); !errors.Is(err, wallet.ErrUserExists) {
		t.Fatalf("duplicate email error = %v, want %v", err, wallet.ErrUserExists)
	}
	snapshot, err := store.WalletByUser(ctx, first.Provisioning.User().ID())
	if err != nil {
		t.Fatalf("read wallet: %v", err)
	}
	if snapshot.Wallet().ID() != first.Provisioning.Wallet().ID() ||
		snapshot.Balances().Available.AmountMinor() != 0 ||
		snapshot.Balances().Pending.AmountMinor() != 0 ||
		snapshot.Balances().CommitmentReserve.AmountMinor() != 0 {
		t.Fatalf("unexpected wallet snapshot: %+v", snapshot)
	}
	transactions, err := store.TransactionsByUser(ctx, first.Provisioning.User().ID(), 50)
	if err != nil {
		t.Fatalf("read transactions: %v", err)
	}
	if len(transactions) != 0 {
		t.Fatalf("new wallet transactions = %d, want 0", len(transactions))
	}
}
