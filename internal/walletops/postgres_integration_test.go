package walletops_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/controlled/internal/custody"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/platform"
	platformpostgres "github.com/raagan-u/controlled/internal/platform/postgres"
	"github.com/raagan-u/controlled/internal/wallet"
	"github.com/raagan-u/controlled/internal/walletops"
)

func TestPostgresWalletOperationsLifecycle(t *testing.T) {
	databaseURL := os.Getenv("CONTROLLED_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("CONTROLLED_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
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
	store := wallet.NewPostgresProvisioningStore(pool)
	onboarding := wallet.NewOnboardingService(ids, platform.SystemClock{}, store)
	testID, err := ids.NewID()
	if err != nil {
		t.Fatalf("generate test ID: %v", err)
	}
	senderEmail := fmt.Sprintf("sender-%s@example.test", testID)
	recipientEmail := fmt.Sprintf("recipient-%s@example.test", testID)
	sender, err := onboarding.Onboard(ctx, "test:sender:"+testID, wallet.OnboardingRequest{
		Email: senderEmail, DisplayName: "Sender", Currency: "USD",
	})
	if err != nil {
		t.Fatalf("onboard sender: %v", err)
	}
	recipient, err := onboarding.Onboard(ctx, "test:recipient:"+testID, wallet.OnboardingRequest{
		Email: recipientEmail, DisplayName: "Recipient", Currency: "USD",
	})
	if err != nil {
		t.Fatalf("onboard recipient: %v", err)
	}

	provider := custody.NewFake(ids)
	operations := walletops.NewService(
		ids,
		platform.SystemClock{},
		walletops.NewPostgresRepository(pool, ledger.NewPostgresBook(pool)),
		provider,
		walletops.AllowAllPolicy{},
	)
	funding, err := operations.CreateFunding(ctx, "fund:"+testID, sender.Provisioning.User().ID(), 10_000, "USD")
	if err != nil {
		t.Fatalf("create funding: %v", err)
	}
	if funding.Status != walletops.FundingAwaitingConfirmation {
		t.Fatalf("funding status = %s", funding.Status)
	}
	assertWalletBalances(t, ctx, store, sender.Provisioning.User().ID(), 0, 0)
	funding, err = operations.ConfirmFunding(ctx, sender.Provisioning.User().ID(), funding.ID)
	if err != nil {
		t.Fatalf("confirm funding: %v", err)
	}
	if funding.Status != walletops.FundingCompleted {
		t.Fatalf("confirmed funding status = %s", funding.Status)
	}
	assertWalletBalances(t, ctx, store, sender.Provisioning.User().ID(), 10_000, 0)

	transfer, err := operations.Transfer(
		ctx, "send:"+testID, sender.Provisioning.User().ID(), recipient.Provisioning.User().CtrldID(), 3_000, "USD",
	)
	if err != nil {
		t.Fatalf("internal transfer: %v", err)
	}
	retry, err := operations.Transfer(
		ctx, "send:"+testID, sender.Provisioning.User().ID(), recipient.Provisioning.User().CtrldID(), 3_000, "USD",
	)
	if err != nil {
		t.Fatalf("retry internal transfer: %v", err)
	}
	if !retry.Duplicate || retry.ID != transfer.ID {
		t.Fatalf("unexpected transfer retry: %+v", retry)
	}
	assertWalletBalances(t, ctx, store, sender.Provisioning.User().ID(), 7_000, 0)
	assertWalletBalances(t, ctx, store, recipient.Provisioning.User().ID(), 3_000, 0)

	withdrawal, err := operations.CreateWithdrawal(
		ctx, "withdraw:"+testID, sender.Provisioning.User().ID(), "paypal@example.test", 2_000, "USD",
	)
	if err != nil {
		t.Fatalf("create withdrawal: %v", err)
	}
	assertWalletBalances(t, ctx, store, sender.Provisioning.User().ID(), 5_000, 2_000)
	withdrawal, err = operations.SimulateWithdrawalSettlement(
		ctx, sender.Provisioning.User().ID(), withdrawal.ID, true,
	)
	if err != nil {
		t.Fatalf("settle withdrawal: %v", err)
	}
	if withdrawal.Status != walletops.WithdrawalCompleted {
		t.Fatalf("withdrawal status = %s", withdrawal.Status)
	}
	assertWalletBalances(t, ctx, store, sender.Provisioning.User().ID(), 5_000, 0)

	failed, err := operations.CreateWithdrawal(
		ctx, "failed:"+testID, recipient.Provisioning.User().ID(), "paypal@example.test", 1_000, "USD",
	)
	if err != nil {
		t.Fatalf("create failed withdrawal: %v", err)
	}
	assertWalletBalances(t, ctx, store, recipient.Provisioning.User().ID(), 2_000, 1_000)
	failed, err = operations.SimulateWithdrawalSettlement(
		ctx, recipient.Provisioning.User().ID(), failed.ID, false,
	)
	if err != nil {
		t.Fatalf("fail withdrawal: %v", err)
	}
	if failed.Status != walletops.WithdrawalFailed {
		t.Fatalf("failed withdrawal status = %s", failed.Status)
	}
	assertWalletBalances(t, ctx, store, recipient.Provisioning.User().ID(), 3_000, 0)

	senderTransactions, err := store.TransactionsByUser(ctx, sender.Provisioning.User().ID(), 50)
	if err != nil {
		t.Fatalf("read sender transactions: %v", err)
	}
	if len(senderTransactions) != 4 {
		t.Fatalf("sender transaction count = %d, want 4", len(senderTransactions))
	}
	detail, err := store.TransactionByUser(ctx, sender.Provisioning.User().ID(), transfer.LedgerTransactionID)
	if err != nil || detail.ID != transfer.LedgerTransactionID || len(detail.Entries) != 1 {
		t.Fatalf("sender transaction detail = %+v err=%v", detail, err)
	}
	if _, err := store.TransactionByUser(ctx, recipient.Provisioning.User().ID(), funding.LedgerTransactionID); !errors.Is(err, wallet.ErrTransactionNotFound) {
		t.Fatalf("cross-wallet transaction detail error = %v", err)
	}
}

func assertWalletBalances(
	t *testing.T,
	ctx context.Context,
	store *wallet.PostgresProvisioningStore,
	userID identity.UserID,
	available int64,
	pending int64,
) {
	t.Helper()
	snapshot, err := store.WalletByUser(ctx, userID)
	if err != nil {
		t.Fatalf("read wallet: %v", err)
	}
	if snapshot.Balances().Available.AmountMinor() != available ||
		snapshot.Balances().Pending.AmountMinor() != pending {
		t.Fatalf(
			"balances = available %d, pending %d; want %d, %d",
			snapshot.Balances().Available.AmountMinor(),
			snapshot.Balances().Pending.AmountMinor(),
			available,
			pending,
		)
	}
}
