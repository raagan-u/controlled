package ledger

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestDatabaseUUID(t *testing.T) {
	t.Parallel()

	parsed, err := databaseUUID("019a0d58-2b8a-7d71-8819-8e116a79b8c2")
	if err != nil {
		t.Fatalf("parse UUID: %v", err)
	}
	if !parsed.Valid {
		t.Fatal("expected parsed UUID to be valid")
	}
}

func TestDatabaseUUIDRejectsDomainIdentifiers(t *testing.T) {
	t.Parallel()

	_, err := databaseUUID("journal-1")
	if !errors.Is(err, ErrInvalidPersistenceID) {
		t.Fatalf("expected ErrInvalidPersistenceID, got %v", err)
	}
}

func TestMapPostgresError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		constraint string
		want       error
	}{
		{name: "journal ID", constraint: "ledger_transactions_pkey", want: ErrJournalExists},
		{name: "idempotency key", constraint: "ledger_transactions_idempotency_key_key", want: ErrIdempotencyConflict},
		{name: "business reference", constraint: "ledger_reference_unique", want: ErrReferenceExists},
		{name: "reversal", constraint: "ledger_transactions_reverses_transaction_id_key", want: ErrJournalAlreadyReversed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := mapPostgresError(&pgconn.PgError{ConstraintName: test.constraint})
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestPostgresBookRequiresPool(t *testing.T) {
	t.Parallel()

	book := NewPostgresBook(nil)
	if _, err := book.Post(t.Context(), Journal{}); !errors.Is(err, ErrLedgerUnavailable) {
		t.Fatalf("post: expected ErrLedgerUnavailable, got %v", err)
	}
	if _, err := book.Balance(t.Context(), "account"); !errors.Is(err, ErrLedgerUnavailable) {
		t.Fatalf("balance: expected ErrLedgerUnavailable, got %v", err)
	}
	if _, err := book.Journal(t.Context(), "journal"); !errors.Is(err, ErrLedgerUnavailable) {
		t.Fatalf("journal: expected ErrLedgerUnavailable, got %v", err)
	}
}
