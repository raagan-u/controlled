package ledger_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/pay-pact/internal/ledger"
	"github.com/raagan-u/pay-pact/internal/money"
	"github.com/raagan-u/pay-pact/internal/platform"
	platformpostgres "github.com/raagan-u/pay-pact/internal/platform/postgres"
)

func TestPostgresBookLifecycle(t *testing.T) {
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
	userID := nextID(t, ids)
	walletID := nextID(t, ids)
	availableID := nextID(t, ids)
	custodyID := ensureCustodyAccount(t, ctx, pool, ids)
	createdAt := time.Now().UTC()

	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, display_name, created_at)
		VALUES ($1::text::uuid, $2, $3, $4)
	`, userID, fmt.Sprintf("ledger-%s@example.test", userID), "Ledger Test", createdAt); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO wallets (id, owner_id, currency, created_at)
		VALUES ($1::text::uuid, $2::text::uuid, 'USD', $3)
	`, walletID, userID, createdAt); err != nil {
		t.Fatalf("create test wallet: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO ledger_accounts (id, wallet_id, kind, normal_side, currency, created_at)
		VALUES ($1::text::uuid, $2::text::uuid, 'AVAILABLE', 'CREDIT', 'USD', $3)
	`, availableID, walletID, createdAt); err != nil {
		t.Fatalf("create available account: %v", err)
	}

	amount, err := money.New(12_500, "USD")
	if err != nil {
		t.Fatalf("create amount: %v", err)
	}
	debit, err := ledger.NewEntry(ledger.AccountID(custodyID), ledger.SideDebit, amount)
	if err != nil {
		t.Fatalf("create custody entry: %v", err)
	}
	credit, err := ledger.NewEntry(ledger.AccountID(availableID), ledger.SideCredit, amount)
	if err != nil {
		t.Fatalf("create available entry: %v", err)
	}

	journalID := nextID(t, ids)
	reference := ledger.Reference{Type: "TEST_FUNDING", ID: nextID(t, ids)}
	journal, err := ledger.NewJournal(
		ledger.JournalID(journalID),
		"test:"+nextID(t, ids),
		reference,
		"Integration-test funding",
		createdAt,
		[]ledger.Entry{debit, credit},
	)
	if err != nil {
		t.Fatalf("create journal: %v", err)
	}

	book := ledger.NewPostgresBook(pool)
	posted, err := book.Post(ctx, journal)
	if err != nil {
		t.Fatalf("post journal: %v", err)
	}
	if posted.Duplicate || posted.JournalID != journal.ID() {
		t.Fatalf("unexpected post result: %+v", posted)
	}

	retry, err := ledger.NewJournal(
		ledger.JournalID(nextID(t, ids)),
		journal.IdempotencyKey(),
		reference,
		journal.Description(),
		createdAt,
		journal.Entries(),
	)
	if err != nil {
		t.Fatalf("create retry: %v", err)
	}
	retried, err := book.Post(ctx, retry)
	if err != nil {
		t.Fatalf("retry journal: %v", err)
	}
	if !retried.Duplicate || retried.JournalID != journal.ID() {
		t.Fatalf("unexpected retry result: %+v", retried)
	}

	balance, err := book.Balance(ctx, ledger.AccountID(availableID))
	if err != nil {
		t.Fatalf("read funded balance: %v", err)
	}
	if !balance.Equal(amount) {
		t.Fatalf("expected balance %v, got %v", amount, balance)
	}
	stored, err := book.Journal(ctx, journal.ID())
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	if stored.ID() != journal.ID() || stored.Reference() != journal.Reference() {
		t.Fatalf("unexpected stored journal: %+v", stored)
	}

	reversal, err := journal.Reverse(
		ledger.JournalID(nextID(t, ids)),
		"test:"+nextID(t, ids),
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("create reversal: %v", err)
	}
	if _, err := book.Reverse(ctx, journal.ID(), reversal); err != nil {
		t.Fatalf("reverse journal: %v", err)
	}
	balance, err = book.Balance(ctx, ledger.AccountID(availableID))
	if err != nil {
		t.Fatalf("read reversed balance: %v", err)
	}
	zero, err := money.New(0, "USD")
	if err != nil {
		t.Fatalf("create zero amount: %v", err)
	}
	if !balance.Equal(zero) {
		t.Fatalf("expected zero balance, got %v", balance)
	}
	repeated, err := book.Reverse(ctx, journal.ID(), reversal)
	if err != nil {
		t.Fatalf("retry reversal: %v", err)
	}
	if !repeated.Duplicate || repeated.JournalID != reversal.ID() {
		t.Fatalf("unexpected reversal retry result: %+v", repeated)
	}
	secondReversal, err := journal.Reverse(
		ledger.JournalID(nextID(t, ids)),
		"test:"+nextID(t, ids),
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("create second reversal: %v", err)
	}
	if _, err := book.Reverse(ctx, journal.ID(), secondReversal); !errors.Is(err, ledger.ErrJournalAlreadyReversed) {
		t.Fatalf("expected ErrJournalAlreadyReversed, got %v", err)
	}
}

func ensureCustodyAccount(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	ids platform.UUIDGenerator,
) string {
	t.Helper()

	accountID := nextID(t, ids)
	if _, err := pool.Exec(ctx, `
		INSERT INTO ledger_accounts (id, kind, normal_side, currency, created_at)
		VALUES ($1::text::uuid, 'CUSTODY_ASSET', 'DEBIT', 'USD', $2)
		ON CONFLICT DO NOTHING
	`, accountID, time.Now().UTC()); err != nil {
		t.Fatalf("ensure custody account: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT id::text
		  FROM ledger_accounts
		 WHERE wallet_id IS NULL
		   AND kind = 'CUSTODY_ASSET'
		   AND currency = 'USD'
	`).Scan(&accountID); err != nil {
		t.Fatalf("read custody account: %v", err)
	}
	return accountID
}

func nextID(t *testing.T, generator platform.UUIDGenerator) string {
	t.Helper()

	id, err := generator.NewID()
	if err != nil {
		t.Fatalf("generate test ID: %v", err)
	}
	return id
}
