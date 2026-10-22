package ledger

import "errors"

var (
	ErrInvalidAccount          = errors.New("ledger account is invalid")
	ErrInvalidAccountKind      = errors.New("ledger account kind is invalid")
	ErrInvalidJournal          = errors.New("ledger journal is invalid")
	ErrInvalidEntry            = errors.New("ledger entry is invalid")
	ErrJournalNeedsTwoEntries  = errors.New("ledger journal requires at least two entries")
	ErrJournalNeedsTwoAccounts = errors.New("ledger journal requires at least two distinct accounts")
	ErrJournalCurrencyMismatch = errors.New("ledger journal must contain one currency")
	ErrJournalUnbalanced       = errors.New("ledger journal debits and credits must balance")
	ErrAmountOverflow          = errors.New("ledger amount exceeds supported range")
)
