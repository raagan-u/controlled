package ledger_test

import (
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/money"
)

func TestNewJournal(t *testing.T) {
	t.Parallel()

	debit := mustEntry(t, "custody-usd", ledger.SideDebit, 2500, "USD")
	credit := mustEntry(t, "available-1", ledger.SideCredit, 2500, "USD")
	journal, err := ledger.NewJournal(
		"journal-1",
		"funding-1",
		ledger.Reference{Type: "FUNDING", ID: "capture-1"},
		"Fund wallet",
		time.Date(2026, time.October, 22, 10, 0, 0, 0, time.UTC),
		[]ledger.Entry{debit, credit},
	)
	if err != nil {
		t.Fatalf("NewJournal() error = %v", err)
	}

	entries := journal.Entries()
	entries[0] = credit
	if journal.Entries()[0].Side() != ledger.SideDebit {
		t.Fatal("Entries() exposed mutable journal state")
	}
}

func TestJournalRejectsBrokenAccounting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []ledger.Entry
		wantErr error
	}{
		{
			name:    "one entry",
			entries: []ledger.Entry{mustEntry(t, "custody-usd", ledger.SideDebit, 2500, "USD")},
			wantErr: ledger.ErrJournalNeedsTwoEntries,
		},
		{
			name: "one account",
			entries: []ledger.Entry{
				mustEntry(t, "custody-usd", ledger.SideDebit, 2500, "USD"),
				mustEntry(t, "custody-usd", ledger.SideCredit, 2500, "USD"),
			},
			wantErr: ledger.ErrJournalNeedsTwoAccounts,
		},
		{
			name: "unbalanced",
			entries: []ledger.Entry{
				mustEntry(t, "custody-usd", ledger.SideDebit, 2500, "USD"),
				mustEntry(t, "available-1", ledger.SideCredit, 2400, "USD"),
			},
			wantErr: ledger.ErrJournalUnbalanced,
		},
		{
			name: "mixed currency",
			entries: []ledger.Entry{
				mustEntry(t, "custody-usd", ledger.SideDebit, 2500, "USD"),
				mustEntry(t, "available-eur", ledger.SideCredit, 2500, "EUR"),
			},
			wantErr: ledger.ErrJournalCurrencyMismatch,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := ledger.NewJournal(
				"journal-1",
				"key-1",
				ledger.Reference{Type: "TEST", ID: "reference-1"},
				"Test journal",
				time.Now(),
				test.entries,
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("NewJournal() error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestReverseSwapsEntrySides(t *testing.T) {
	t.Parallel()

	journal := mustJournal(t)
	reversal, err := journal.Reverse("journal-2", "reversal-1", time.Now())
	if err != nil {
		t.Fatalf("Reverse() error = %v", err)
	}
	if reversal.Reference().Type != "REVERSAL" || reversal.Reference().ID != string(journal.ID()) {
		t.Fatalf("Reverse() reference = %#v", reversal.Reference())
	}
	if reversal.Entries()[0].Side() != ledger.SideCredit || reversal.Entries()[1].Side() != ledger.SideDebit {
		t.Fatal("Reverse() did not swap entry sides")
	}
}

func mustJournal(t *testing.T) ledger.Journal {
	t.Helper()

	journal, err := ledger.NewJournal(
		"journal-1",
		"funding-1",
		ledger.Reference{Type: "FUNDING", ID: "capture-1"},
		"Fund wallet",
		time.Now(),
		[]ledger.Entry{
			mustEntry(t, "custody-usd", ledger.SideDebit, 2500, "USD"),
			mustEntry(t, "available-1", ledger.SideCredit, 2500, "USD"),
		},
	)
	if err != nil {
		t.Fatalf("NewJournal() error = %v", err)
	}
	return journal
}

func mustEntry(t *testing.T, accountID ledger.AccountID, side ledger.Side, amountMinor int64, currency string) ledger.Entry {
	t.Helper()

	amount, err := money.New(amountMinor, currency)
	if err != nil {
		t.Fatalf("money.New() error = %v", err)
	}
	entry, err := ledger.NewEntry(accountID, side, amount)
	if err != nil {
		t.Fatalf("NewEntry() error = %v", err)
	}
	return entry
}
