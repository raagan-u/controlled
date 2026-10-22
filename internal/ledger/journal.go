package ledger

import (
	"math"
	"strings"
	"time"

	"github.com/raagan-u/pay-pact/internal/money"
)

type JournalID string

type Reference struct {
	Type string
	ID   string
}

func (r Reference) valid() bool {
	return strings.TrimSpace(r.Type) != "" && strings.TrimSpace(r.ID) != ""
}

type Entry struct {
	accountID AccountID
	side      Side
	amount    money.Money
}

func NewEntry(accountID AccountID, side Side, amount money.Money) (Entry, error) {
	if accountID == "" || (side != SideDebit && side != SideCredit) || !amount.Positive() {
		return Entry{}, ErrInvalidEntry
	}

	return Entry{accountID: accountID, side: side, amount: amount}, nil
}

func (e Entry) AccountID() AccountID { return e.accountID }
func (e Entry) Side() Side           { return e.side }
func (e Entry) Amount() money.Money  { return e.amount }

type Journal struct {
	id             JournalID
	idempotencyKey string
	reference      Reference
	description    string
	occurredAt     time.Time
	entries        []Entry
}

func NewJournal(
	id JournalID,
	idempotencyKey string,
	reference Reference,
	description string,
	occurredAt time.Time,
	entries []Entry,
) (Journal, error) {
	journal := Journal{
		id:             id,
		idempotencyKey: strings.TrimSpace(idempotencyKey),
		reference: Reference{
			Type: strings.TrimSpace(reference.Type),
			ID:   strings.TrimSpace(reference.ID),
		},
		description: strings.TrimSpace(description),
		occurredAt:  occurredAt.UTC(),
		entries:     append([]Entry(nil), entries...),
	}

	if err := journal.validate(); err != nil {
		return Journal{}, err
	}

	return journal, nil
}

func (j Journal) validate() error {
	if j.id == "" || j.idempotencyKey == "" || !j.reference.valid() || j.occurredAt.IsZero() {
		return ErrInvalidJournal
	}
	if len(j.entries) < 2 {
		return ErrJournalNeedsTwoEntries
	}

	currency := ""
	accounts := make(map[AccountID]struct{}, len(j.entries))
	var debits int64
	var credits int64
	for _, entry := range j.entries {
		if entry.accountID == "" || (entry.side != SideDebit && entry.side != SideCredit) || !entry.amount.Positive() {
			return ErrInvalidEntry
		}
		if currency == "" {
			currency = entry.amount.Currency()
		} else if entry.amount.Currency() != currency {
			return ErrJournalCurrencyMismatch
		}
		accounts[entry.accountID] = struct{}{}

		var ok bool
		if entry.side == SideDebit {
			debits, ok = safeAdd(debits, entry.amount.AmountMinor())
		} else {
			credits, ok = safeAdd(credits, entry.amount.AmountMinor())
		}
		if !ok {
			return ErrAmountOverflow
		}
	}

	if len(accounts) < 2 {
		return ErrJournalNeedsTwoAccounts
	}
	if debits != credits {
		return ErrJournalUnbalanced
	}

	return nil
}

func (j Journal) Reverse(id JournalID, idempotencyKey string, occurredAt time.Time) (Journal, error) {
	entries := make([]Entry, 0, len(j.entries))
	for _, original := range j.entries {
		side := SideDebit
		if original.side == SideDebit {
			side = SideCredit
		}
		entries = append(entries, Entry{accountID: original.accountID, side: side, amount: original.amount})
	}

	return NewJournal(
		id,
		idempotencyKey,
		Reference{Type: "REVERSAL", ID: string(j.id)},
		"Reversal: "+j.description,
		occurredAt,
		entries,
	)
}

func (j Journal) ID() JournalID          { return j.id }
func (j Journal) IdempotencyKey() string { return j.idempotencyKey }
func (j Journal) Reference() Reference   { return j.reference }
func (j Journal) Description() string    { return j.description }
func (j Journal) OccurredAt() time.Time  { return j.occurredAt }
func (j Journal) Entries() []Entry       { return append([]Entry(nil), j.entries...) }

func safeAdd(left, right int64) (int64, bool) {
	if right > 0 && left > math.MaxInt64-right {
		return 0, false
	}
	return left + right, true
}
