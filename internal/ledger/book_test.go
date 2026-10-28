package ledger_test

import (
	"errors"
	"testing"
	"time"

	"github.com/raagan-u/controlled/internal/ledger"
)

func TestBookPostsFunding(t *testing.T) {
	t.Parallel()

	book := fundedBook(t, 2500)
	assertBalance(t, book, "custody-usd", 2500)
	assertBalance(t, book, "available-1", 2500)
}

func TestBookPostsInternalSend(t *testing.T) {
	t.Parallel()

	book := fundedBook(t, 2500)
	receiver := mustAccount(t, "available-2", "user-2", ledger.AccountKindAvailable)
	if err := book.RegisterAccounts(receiver); err != nil {
		t.Fatalf("RegisterAccounts() error = %v", err)
	}

	send := newJournal(t, "send-1", "send-key-1", ledger.Reference{Type: "INTERNAL_SEND", ID: "transfer-1"}, []ledger.Entry{
		mustEntry(t, "available-1", ledger.SideDebit, 700, "USD"),
		mustEntry(t, "available-2", ledger.SideCredit, 700, "USD"),
	})
	if _, err := book.Post(send); err != nil {
		t.Fatalf("Post() error = %v", err)
	}

	assertBalance(t, book, "available-1", 1800)
	assertBalance(t, book, "available-2", 700)
	assertBalance(t, book, "custody-usd", 2500)
}

func TestBookRejectsOverdraftAtomically(t *testing.T) {
	t.Parallel()

	book := fundedBook(t, 2500)
	receiver := mustAccount(t, "available-2", "user-2", ledger.AccountKindAvailable)
	if err := book.RegisterAccounts(receiver); err != nil {
		t.Fatalf("RegisterAccounts() error = %v", err)
	}

	send := newJournal(t, "send-1", "send-key-1", ledger.Reference{Type: "INTERNAL_SEND", ID: "transfer-1"}, []ledger.Entry{
		mustEntry(t, "available-1", ledger.SideDebit, 2600, "USD"),
		mustEntry(t, "available-2", ledger.SideCredit, 2600, "USD"),
	})
	if _, err := book.Post(send); !errors.Is(err, ledger.ErrInsufficientBalance) {
		t.Fatalf("Post() error = %v, want %v", err, ledger.ErrInsufficientBalance)
	}

	assertBalance(t, book, "available-1", 2500)
	assertBalance(t, book, "available-2", 0)
}

func TestBookMakesPostingIdempotent(t *testing.T) {
	t.Parallel()

	book := newBook(t)
	journal := fundingJournal(t, "funding-1", "funding-key-1", 2500)
	first, err := book.Post(journal)
	if err != nil {
		t.Fatalf("first Post() error = %v", err)
	}

	retry := fundingJournal(t, "new-generated-id", "funding-key-1", 2500)
	second, err := book.Post(retry)
	if err != nil {
		t.Fatalf("retry Post() error = %v", err)
	}
	if !second.Duplicate || second.JournalID != first.JournalID {
		t.Fatalf("retry result = %#v, want duplicate of %s", second, first.JournalID)
	}
	assertBalance(t, book, "available-1", 2500)

	conflict := fundingJournal(t, "funding-2", "funding-key-1", 3000)
	if _, err := book.Post(conflict); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("conflicting Post() error = %v, want %v", err, ledger.ErrIdempotencyConflict)
	}
}

func TestBookReversesWithANewJournal(t *testing.T) {
	t.Parallel()

	book := newBook(t)
	original := fundingJournal(t, "funding-1", "funding-key-1", 2500)
	if _, err := book.Post(original); err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	reversal, err := original.Reverse("reversal-1", "reversal-key-1", time.Now())
	if err != nil {
		t.Fatalf("Reverse() error = %v", err)
	}
	if _, err := book.Reverse(original.ID(), reversal); err != nil {
		t.Fatalf("Book.Reverse() error = %v", err)
	}

	assertBalance(t, book, "custody-usd", 0)
	assertBalance(t, book, "available-1", 0)

	second, err := original.Reverse("reversal-2", "reversal-key-2", time.Now())
	if err != nil {
		t.Fatalf("Reverse() error = %v", err)
	}
	if _, err := book.Reverse(original.ID(), second); !errors.Is(err, ledger.ErrJournalAlreadyReversed) {
		t.Fatalf("second Book.Reverse() error = %v, want %v", err, ledger.ErrJournalAlreadyReversed)
	}
}

func TestBookRejectsUnknownAndMismatchedAccounts(t *testing.T) {
	t.Parallel()

	t.Run("unknown account", func(t *testing.T) {
		book := newBook(t)
		journal := newJournal(t, "journal-1", "key-1", ledger.Reference{Type: "TEST", ID: "test-1"}, []ledger.Entry{
			mustEntry(t, "custody-usd", ledger.SideDebit, 100, "USD"),
			mustEntry(t, "missing", ledger.SideCredit, 100, "USD"),
		})
		if _, err := book.Post(journal); !errors.Is(err, ledger.ErrAccountNotFound) {
			t.Fatalf("Post() error = %v, want %v", err, ledger.ErrAccountNotFound)
		}
	})

	t.Run("account currency", func(t *testing.T) {
		book := ledger.NewBook()
		if err := book.RegisterAccounts(
			mustAccount(t, "custody-eur", "", ledger.AccountKindCustodyAsset),
			mustAccountWithCurrency(t, "available-eur", "user-1", ledger.AccountKindAvailable, "EUR"),
		); err != nil {
			t.Fatalf("RegisterAccounts() error = %v", err)
		}
		journal := newJournal(t, "journal-1", "key-1", ledger.Reference{Type: "TEST", ID: "test-1"}, []ledger.Entry{
			mustEntry(t, "custody-eur", ledger.SideDebit, 100, "USD"),
			mustEntry(t, "available-eur", ledger.SideCredit, 100, "USD"),
		})
		if _, err := book.Post(journal); !errors.Is(err, ledger.ErrAccountCurrencyMismatch) {
			t.Fatalf("Post() error = %v, want %v", err, ledger.ErrAccountCurrencyMismatch)
		}
	})
}

func newBook(t *testing.T) *ledger.Book {
	t.Helper()

	book := ledger.NewBook()
	if err := book.RegisterAccounts(
		mustAccount(t, "custody-usd", "", ledger.AccountKindCustodyAsset),
		mustAccount(t, "available-1", "user-1", ledger.AccountKindAvailable),
	); err != nil {
		t.Fatalf("RegisterAccounts() error = %v", err)
	}
	return book
}

func fundedBook(t *testing.T, amountMinor int64) *ledger.Book {
	t.Helper()

	book := newBook(t)
	if _, err := book.Post(fundingJournal(t, "funding-1", "funding-key-1", amountMinor)); err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	return book
}

func fundingJournal(t *testing.T, id ledger.JournalID, key string, amountMinor int64) ledger.Journal {
	t.Helper()
	return newJournal(t, id, key, ledger.Reference{Type: "FUNDING", ID: "capture-1"}, []ledger.Entry{
		mustEntry(t, "custody-usd", ledger.SideDebit, amountMinor, "USD"),
		mustEntry(t, "available-1", ledger.SideCredit, amountMinor, "USD"),
	})
}

func newJournal(t *testing.T, id ledger.JournalID, key string, reference ledger.Reference, entries []ledger.Entry) ledger.Journal {
	t.Helper()

	journal, err := ledger.NewJournal(id, key, reference, "Test posting", time.Now(), entries)
	if err != nil {
		t.Fatalf("NewJournal() error = %v", err)
	}
	return journal
}

func mustAccount(t *testing.T, id ledger.AccountID, ownerID ledger.OwnerID, kind ledger.AccountKind) ledger.Account {
	t.Helper()
	return mustAccountWithCurrency(t, id, ownerID, kind, "USD")
}

func mustAccountWithCurrency(t *testing.T, id ledger.AccountID, ownerID ledger.OwnerID, kind ledger.AccountKind, currency string) ledger.Account {
	t.Helper()

	account, err := ledger.NewAccount(id, ownerID, kind, currency)
	if err != nil {
		t.Fatalf("NewAccount() error = %v", err)
	}
	return account
}

func assertBalance(t *testing.T, book *ledger.Book, accountID ledger.AccountID, want int64) {
	t.Helper()

	balance, err := book.Balance(accountID)
	if err != nil {
		t.Fatalf("Balance() error = %v", err)
	}
	if balance.AmountMinor() != want {
		t.Fatalf("Balance(%s) = %d, want %d", accountID, balance.AmountMinor(), want)
	}
}
