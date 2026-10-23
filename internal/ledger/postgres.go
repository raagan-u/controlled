package ledger

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/pay-pact/internal/money"
)

type PostgresBook struct {
	pool *pgxpool.Pool
}

func NewPostgresBook(pool *pgxpool.Pool) *PostgresBook {
	return &PostgresBook{pool: pool}
}

func (b *PostgresBook) Post(ctx context.Context, journal Journal) (PostResult, error) {
	if b == nil || b.pool == nil {
		return PostResult{}, ErrLedgerUnavailable
	}

	transaction, err := b.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return PostResult{}, fmt.Errorf("begin ledger posting: %w", err)
	}
	defer func() {
		_ = transaction.Rollback(context.Background())
	}()
	result, err := b.PostInTx(ctx, transaction, journal)
	if err != nil {
		return PostResult{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return PostResult{}, mapPostgresError(err)
	}
	return result, nil
}

// PostInTx posts a journal inside a caller-owned transaction so an operation
// record and its money movement can commit atomically. The caller must use a
// serializable transaction and remains responsible for commit or rollback.
func (b *PostgresBook) PostInTx(ctx context.Context, transaction pgx.Tx, journal Journal) (PostResult, error) {
	if b == nil || transaction == nil {
		return PostResult{}, ErrLedgerUnavailable
	}
	if err := journal.validate(); err != nil {
		return PostResult{}, err
	}

	if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", journal.idempotencyKey); err != nil {
		return PostResult{}, fmt.Errorf("lock ledger idempotency key: %w", err)
	}
	existing, err := loadJournalByIdempotencyKey(ctx, transaction, journal.idempotencyKey)
	switch {
	case err == nil:
		if !samePosting(existing, journal) {
			return PostResult{}, ErrIdempotencyConflict
		}
		return PostResult{JournalID: existing.id, Duplicate: true}, nil
	case !errors.Is(err, ErrJournalNotFound):
		return PostResult{}, err
	}

	journalID, err := databaseUUID(string(journal.id))
	if err != nil {
		return PostResult{}, err
	}
	if _, err := loadJournal(ctx, transaction, journal.id); err == nil {
		return PostResult{}, ErrJournalExists
	} else if !errors.Is(err, ErrJournalNotFound) {
		return PostResult{}, err
	}

	if journal.reference.Type == "REVERSAL" {
		if err := validateDatabaseReversal(ctx, transaction, journal); err != nil {
			return PostResult{}, err
		}
	}

	states, err := lockAccountStates(ctx, transaction, journal.entries)
	if err != nil {
		return PostResult{}, err
	}
	if _, err := evaluatePosting(journal, states); err != nil {
		return PostResult{}, err
	}

	var reversesID any
	if journal.reference.Type == "REVERSAL" {
		parsed, err := databaseUUID(journal.reference.ID)
		if err != nil {
			return PostResult{}, err
		}
		reversesID = parsed
	}
	_, err = transaction.Exec(ctx, `
		INSERT INTO ledger_transactions (
			id,
			idempotency_key,
			reference_type,
			reference_id,
			description,
			occurred_at,
			reverses_transaction_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, journalID, journal.idempotencyKey, journal.reference.Type, journal.reference.ID, journal.description, journal.occurredAt, reversesID)
	if err != nil {
		return PostResult{}, mapPostgresError(err)
	}

	for index, entry := range journal.entries {
		accountID, err := databaseUUID(string(entry.accountID))
		if err != nil {
			return PostResult{}, err
		}
		_, err = transaction.Exec(ctx, `
			INSERT INTO ledger_entries (
				transaction_id,
				line_number,
				account_id,
				side,
				amount_minor,
				currency
			) VALUES ($1, $2, $3, $4, $5, $6)
		`, journalID, index+1, accountID, entry.side, entry.amount.AmountMinor(), entry.amount.Currency())
		if err != nil {
			return PostResult{}, mapPostgresError(err)
		}
	}

	return PostResult{JournalID: journal.id}, nil
}

func (b *PostgresBook) Reverse(ctx context.Context, originalID JournalID, reversal Journal) (PostResult, error) {
	if reversal.reference.Type != "REVERSAL" || reversal.reference.ID != string(originalID) {
		return PostResult{}, ErrInvalidJournal
	}
	return b.Post(ctx, reversal)
}

func (b *PostgresBook) Balance(ctx context.Context, accountID AccountID) (money.Money, error) {
	if b == nil || b.pool == nil {
		return money.Money{}, ErrLedgerUnavailable
	}
	parsedID, err := databaseUUID(string(accountID))
	if err != nil {
		return money.Money{}, err
	}

	var amountMinor int64
	var currency string
	err = b.pool.QueryRow(ctx, `
		SELECT amount_minor, currency
		  FROM ledger_account_balances
		 WHERE account_id = $1
	`, parsedID).Scan(&amountMinor, &currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return money.Money{}, ErrAccountNotFound
	}
	if err != nil {
		return money.Money{}, fmt.Errorf("read ledger balance: %w", err)
	}
	return money.New(amountMinor, currency)
}

func (b *PostgresBook) Journal(ctx context.Context, id JournalID) (Journal, error) {
	if b == nil || b.pool == nil {
		return Journal{}, ErrLedgerUnavailable
	}
	return loadJournal(ctx, b.pool, id)
}

type journalQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadJournalByIdempotencyKey(ctx context.Context, query journalQuerier, key string) (Journal, error) {
	var id string
	err := query.QueryRow(ctx, "SELECT id::text FROM ledger_transactions WHERE idempotency_key = $1", key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Journal{}, ErrJournalNotFound
	}
	if err != nil {
		return Journal{}, fmt.Errorf("find ledger idempotency key: %w", err)
	}
	return loadJournal(ctx, query, JournalID(id))
}

func loadJournal(ctx context.Context, query journalQuerier, id JournalID) (Journal, error) {
	parsedID, err := databaseUUID(string(id))
	if err != nil {
		return Journal{}, err
	}

	var storedID string
	var idempotencyKey string
	var reference Reference
	var description string
	var occurredAt pgtype.Timestamptz
	err = query.QueryRow(ctx, `
		SELECT id::text,
		       idempotency_key,
		       reference_type,
		       reference_id,
		       description,
		       occurred_at
		  FROM ledger_transactions
		 WHERE id = $1
	`, parsedID).Scan(
		&storedID,
		&idempotencyKey,
		&reference.Type,
		&reference.ID,
		&description,
		&occurredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Journal{}, ErrJournalNotFound
	}
	if err != nil {
		return Journal{}, fmt.Errorf("read ledger journal: %w", err)
	}

	rows, err := query.Query(ctx, `
		SELECT account_id::text, side, amount_minor, currency
		  FROM ledger_entries
		 WHERE transaction_id = $1
		 ORDER BY line_number
	`, parsedID)
	if err != nil {
		return Journal{}, fmt.Errorf("read ledger entries: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var accountID string
		var side Side
		var amountMinor int64
		var currency string
		if err := rows.Scan(&accountID, &side, &amountMinor, &currency); err != nil {
			return Journal{}, fmt.Errorf("scan ledger entry: %w", err)
		}
		amount, err := money.New(amountMinor, currency)
		if err != nil {
			return Journal{}, fmt.Errorf("load ledger money: %w", err)
		}
		entry, err := NewEntry(AccountID(accountID), side, amount)
		if err != nil {
			return Journal{}, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return Journal{}, fmt.Errorf("iterate ledger entries: %w", err)
	}
	return NewJournal(JournalID(storedID), idempotencyKey, reference, description, occurredAt.Time, entries)
}

func validateDatabaseReversal(ctx context.Context, transaction pgx.Tx, reversal Journal) error {
	original, err := loadJournal(ctx, transaction, JournalID(reversal.reference.ID))
	if err != nil {
		return err
	}
	if !oppositeEntries(original.entries, reversal.entries) {
		return ErrInvalidJournal
	}

	originalID, err := databaseUUID(reversal.reference.ID)
	if err != nil {
		return err
	}
	var exists bool
	if err := transaction.QueryRow(
		ctx,
		"SELECT EXISTS (SELECT 1 FROM ledger_transactions WHERE reverses_transaction_id = $1)",
		originalID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check ledger reversal: %w", err)
	}
	if exists {
		return ErrJournalAlreadyReversed
	}
	return nil
}

func lockAccountStates(ctx context.Context, transaction pgx.Tx, entries []Entry) (map[AccountID]accountState, error) {
	unique := make(map[AccountID]pgtype.UUID, len(entries))
	for _, entry := range entries {
		if _, exists := unique[entry.accountID]; exists {
			continue
		}
		parsed, err := databaseUUID(string(entry.accountID))
		if err != nil {
			return nil, err
		}
		unique[entry.accountID] = parsed
	}

	ordered := make([]AccountID, 0, len(unique))
	for accountID := range unique {
		ordered = append(ordered, accountID)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })
	ids := make([]pgtype.UUID, 0, len(ordered))
	for _, accountID := range ordered {
		ids = append(ids, unique[accountID])
	}

	rows, err := transaction.Query(ctx, `
		SELECT account.id::text,
		       COALESCE(wallet.owner_id::text, ''),
		       account.kind,
		       account.normal_side,
		       account.currency,
		       balance.amount_minor
		  FROM ledger_accounts AS account
		  LEFT JOIN wallets AS wallet ON wallet.id = account.wallet_id
		  JOIN ledger_account_balances AS balance ON balance.account_id = account.id
		 WHERE account.id = ANY($1::uuid[])
		 ORDER BY account.id
		 FOR UPDATE OF account
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("lock ledger accounts: %w", err)
	}
	defer rows.Close()

	states := make(map[AccountID]accountState, len(ids))
	for rows.Next() {
		var id string
		var ownerID string
		var kind AccountKind
		var storedNormalSide Side
		var currency string
		var balance int64
		if err := rows.Scan(&id, &ownerID, &kind, &storedNormalSide, &currency, &balance); err != nil {
			return nil, fmt.Errorf("scan ledger account: %w", err)
		}
		account, err := NewAccount(AccountID(id), OwnerID(ownerID), kind, currency)
		if err != nil || account.normalSide != storedNormalSide {
			return nil, ErrInvalidAccount
		}
		states[account.id] = accountState{account: account, balance: balance}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ledger accounts: %w", err)
	}
	if len(states) != len(ids) {
		return nil, ErrAccountNotFound
	}
	return states, nil
}

func databaseUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil || !parsed.Valid {
		return pgtype.UUID{}, fmt.Errorf("%w: %q", ErrInvalidPersistenceID, value)
	}
	return parsed, nil
}

func mapPostgresError(err error) error {
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) {
		return err
	}
	switch databaseError.ConstraintName {
	case "ledger_transactions_pkey":
		return ErrJournalExists
	case "ledger_transactions_idempotency_key_key":
		return ErrIdempotencyConflict
	case "ledger_reference_unique":
		return ErrReferenceExists
	case "ledger_transactions_reverses_transaction_id_key":
		return ErrJournalAlreadyReversed
	default:
		return err
	}
}
