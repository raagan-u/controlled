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
	ErrAccountExists           = errors.New("ledger account already exists")
	ErrAccountNotFound         = errors.New("ledger account was not found")
	ErrAccountCurrencyMismatch = errors.New("ledger entry currency does not match its account")
	ErrJournalExists           = errors.New("ledger journal already exists")
	ErrJournalNotFound         = errors.New("ledger journal was not found")
	ErrIdempotencyConflict     = errors.New("idempotency key was reused for a different journal")
	ErrInsufficientBalance     = errors.New("ledger posting would make an account balance negative")
	ErrJournalAlreadyReversed  = errors.New("ledger journal has already been reversed")
	ErrReferenceExists         = errors.New("ledger reference already exists")
	ErrInvalidPersistenceID    = errors.New("ledger persistence requires UUID identifiers")
	ErrLedgerUnavailable       = errors.New("ledger persistence is unavailable")
)
