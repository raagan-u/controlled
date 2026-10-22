package ledger

import (
	"math"
	"sync"

	"github.com/raagan-u/pay-pact/internal/money"
)

// Book is the in-memory reference implementation of PayPact's ledger rules.
// The PostgreSQL adapter must preserve the same atomicity and invariants.
type Book struct {
	mu          sync.RWMutex
	accounts    map[AccountID]Account
	balances    map[AccountID]int64
	journals    map[JournalID]Journal
	idempotency map[string]JournalID
	reversedBy  map[JournalID]JournalID
}

func NewBook() *Book {
	return &Book{
		accounts:    make(map[AccountID]Account),
		balances:    make(map[AccountID]int64),
		journals:    make(map[JournalID]Journal),
		idempotency: make(map[string]JournalID),
		reversedBy:  make(map[JournalID]JournalID),
	}
}

func (b *Book) RegisterAccounts(accounts ...Account) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	batch := make(map[AccountID]struct{}, len(accounts))
	for _, account := range accounts {
		if !account.Valid() {
			return ErrInvalidAccount
		}
		if _, exists := b.accounts[account.id]; exists {
			return ErrAccountExists
		}
		if _, exists := batch[account.id]; exists {
			return ErrAccountExists
		}
		batch[account.id] = struct{}{}
	}

	for _, account := range accounts {
		b.accounts[account.id] = account
		b.balances[account.id] = 0
	}
	return nil
}

type PostResult struct {
	JournalID JournalID
	Duplicate bool
}

func (b *Book) Post(journal Journal) (PostResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := journal.validate(); err != nil {
		return PostResult{}, err
	}
	if existingID, exists := b.idempotency[journal.idempotencyKey]; exists {
		existing := b.journals[existingID]
		if !samePosting(existing, journal) {
			return PostResult{}, ErrIdempotencyConflict
		}
		return PostResult{JournalID: existingID, Duplicate: true}, nil
	}
	if _, exists := b.journals[journal.id]; exists {
		return PostResult{}, ErrJournalExists
	}
	var reversedJournalID JournalID
	if journal.reference.Type == "REVERSAL" {
		reversedJournalID = JournalID(journal.reference.ID)
		original, exists := b.journals[reversedJournalID]
		if !exists {
			return PostResult{}, ErrJournalNotFound
		}
		if _, exists := b.reversedBy[reversedJournalID]; exists {
			return PostResult{}, ErrJournalAlreadyReversed
		}
		if !oppositeEntries(original.entries, journal.entries) {
			return PostResult{}, ErrInvalidJournal
		}
	}

	deltas := make(map[AccountID]int64, len(journal.entries))
	for _, entry := range journal.entries {
		account, exists := b.accounts[entry.accountID]
		if !exists {
			return PostResult{}, ErrAccountNotFound
		}
		if account.currency != entry.amount.Currency() {
			return PostResult{}, ErrAccountCurrencyMismatch
		}

		delta := entry.amount.AmountMinor()
		if entry.side != account.normalSide {
			delta = -delta
		}
		updated, ok := safeSignedAdd(deltas[entry.accountID], delta)
		if !ok {
			return PostResult{}, ErrAmountOverflow
		}
		deltas[entry.accountID] = updated
	}

	nextBalances := make(map[AccountID]int64, len(deltas))
	for accountID, delta := range deltas {
		next, ok := safeSignedAdd(b.balances[accountID], delta)
		if !ok {
			return PostResult{}, ErrAmountOverflow
		}
		if next < 0 {
			return PostResult{}, ErrInsufficientBalance
		}
		nextBalances[accountID] = next
	}

	for accountID, balance := range nextBalances {
		b.balances[accountID] = balance
	}
	b.journals[journal.id] = journal
	b.idempotency[journal.idempotencyKey] = journal.id
	if reversedJournalID != "" {
		b.reversedBy[reversedJournalID] = journal.id
	}

	return PostResult{JournalID: journal.id}, nil
}

func (b *Book) Reverse(originalID JournalID, reversal Journal) (PostResult, error) {
	b.mu.RLock()
	original, exists := b.journals[originalID]
	_, alreadyReversed := b.reversedBy[originalID]
	b.mu.RUnlock()

	if !exists {
		return PostResult{}, ErrJournalNotFound
	}
	if alreadyReversed {
		return PostResult{}, ErrJournalAlreadyReversed
	}
	if reversal.reference.Type != "REVERSAL" || reversal.reference.ID != string(original.id) {
		return PostResult{}, ErrInvalidJournal
	}
	if !oppositeEntries(original.entries, reversal.entries) {
		return PostResult{}, ErrInvalidJournal
	}

	return b.Post(reversal)
}

func (b *Book) Balance(accountID AccountID) (money.Money, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	account, exists := b.accounts[accountID]
	if !exists {
		return money.Money{}, ErrAccountNotFound
	}
	return money.New(b.balances[accountID], account.currency)
}

func (b *Book) Journal(id JournalID) (Journal, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	journal, exists := b.journals[id]
	if !exists {
		return Journal{}, ErrJournalNotFound
	}
	return journal, nil
}

func samePosting(left, right Journal) bool {
	if left.reference != right.reference || left.description != right.description || len(left.entries) != len(right.entries) {
		return false
	}
	for index := range left.entries {
		leftEntry := left.entries[index]
		rightEntry := right.entries[index]
		if leftEntry.accountID != rightEntry.accountID ||
			leftEntry.side != rightEntry.side ||
			!leftEntry.amount.Equal(rightEntry.amount) {
			return false
		}
	}
	return true
}

func oppositeEntries(original, reversal []Entry) bool {
	if len(original) != len(reversal) {
		return false
	}
	for index := range original {
		left := original[index]
		right := reversal[index]
		if left.accountID != right.accountID || !left.amount.Equal(right.amount) || left.side == right.side {
			return false
		}
	}
	return true
}

func safeSignedAdd(left, right int64) (int64, bool) {
	if right > 0 && left > math.MaxInt64-right {
		return 0, false
	}
	if right < 0 && left < math.MinInt64-right {
		return 0, false
	}
	return left + right, true
}
