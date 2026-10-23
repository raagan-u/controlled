package wallet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/pay-pact/internal/identity"
	"github.com/raagan-u/pay-pact/internal/ledger"
	"github.com/raagan-u/pay-pact/internal/money"
)

const onboardingIdempotencyScope = "wallet_onboarding"

type PostgresProvisioningStore struct {
	pool *pgxpool.Pool
}

func (s *PostgresProvisioningStore) WalletByUser(
	ctx context.Context,
	userID identity.UserID,
) (Snapshot, error) {
	if s == nil || s.pool == nil {
		return Snapshot{}, ErrStoreUnavailable
	}
	parsedUserID, err := walletUUID(string(userID))
	if err != nil {
		return Snapshot{}, err
	}

	var walletID string
	var currency string
	var createdAt pgtype.Timestamptz
	var availableID string
	var pendingID string
	var reserveID string
	var availableMinor int64
	var pendingMinor int64
	var reserveMinor int64
	err = s.pool.QueryRow(ctx, `
		SELECT wallet.id::text,
		       wallet.currency,
		       wallet.created_at,
		       max(account.id::text) FILTER (WHERE account.kind = 'AVAILABLE'),
		       max(account.id::text) FILTER (WHERE account.kind = 'PENDING'),
		       max(account.id::text) FILTER (WHERE account.kind = 'COMMITMENT_RESERVE'),
		       max(balance.amount_minor) FILTER (WHERE account.kind = 'AVAILABLE'),
		       max(balance.amount_minor) FILTER (WHERE account.kind = 'PENDING'),
		       max(balance.amount_minor) FILTER (WHERE account.kind = 'COMMITMENT_RESERVE')
		  FROM wallets AS wallet
		  JOIN ledger_accounts AS account ON account.wallet_id = wallet.id
		  JOIN ledger_account_balances AS balance ON balance.account_id = account.id
		 WHERE wallet.owner_id = $1
		 GROUP BY wallet.id
	`, parsedUserID).Scan(
		&walletID,
		&currency,
		&createdAt,
		&availableID,
		&pendingID,
		&reserveID,
		&availableMinor,
		&pendingMinor,
		&reserveMinor,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Snapshot{}, ErrProvisioningNotFound
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("read wallet: %w", err)
	}

	wallet, err := NewWallet(
		WalletID(walletID),
		userID,
		currency,
		AccountSet{
			Available:         ledger.AccountID(availableID),
			Pending:           ledger.AccountID(pendingID),
			CommitmentReserve: ledger.AccountID(reserveID),
		},
		createdAt.Time,
	)
	if err != nil {
		return Snapshot{}, err
	}
	available, err := money.New(availableMinor, currency)
	if err != nil {
		return Snapshot{}, err
	}
	pending, err := money.New(pendingMinor, currency)
	if err != nil {
		return Snapshot{}, err
	}
	reserve, err := money.New(reserveMinor, currency)
	if err != nil {
		return Snapshot{}, err
	}
	return NewSnapshot(wallet, BalanceSet{
		Available:         available,
		Pending:           pending,
		CommitmentReserve: reserve,
	})
}

func (s *PostgresProvisioningStore) TransactionsByUser(
	ctx context.Context,
	userID identity.UserID,
	limit int,
) ([]Transaction, error) {
	if s == nil || s.pool == nil {
		return nil, ErrStoreUnavailable
	}
	parsedUserID, err := walletUUID(string(userID))
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := s.pool.Query(ctx, `
		WITH wallet_accounts AS (
			SELECT account.id, account.kind
			  FROM wallets AS wallet
			  JOIN ledger_accounts AS account ON account.wallet_id = wallet.id
			 WHERE wallet.owner_id = $1
		), recent_transactions AS (
			SELECT transaction.id, transaction.posted_at
			  FROM ledger_transactions AS transaction
			 WHERE EXISTS (
				SELECT 1
				  FROM ledger_entries AS entry
				  JOIN wallet_accounts ON wallet_accounts.id = entry.account_id
				 WHERE entry.transaction_id = transaction.id
			 )
			 ORDER BY transaction.posted_at DESC, transaction.id DESC
			 LIMIT $2
		)
		SELECT transaction.id::text,
		       transaction.reference_type,
		       transaction.reference_id,
		       transaction.description,
		       transaction.occurred_at,
		       transaction.posted_at,
		       EXISTS (
			       SELECT 1
			         FROM ledger_transactions AS reversal
			        WHERE reversal.reverses_transaction_id = transaction.id
		       ) AS reversed,
		       entry.account_id::text,
		       wallet_accounts.kind,
		       entry.side,
		       entry.amount_minor,
		       entry.currency
		  FROM recent_transactions
		  JOIN ledger_transactions AS transaction ON transaction.id = recent_transactions.id
		  JOIN ledger_entries AS entry ON entry.transaction_id = transaction.id
		  JOIN wallet_accounts ON wallet_accounts.id = entry.account_id
		 ORDER BY transaction.posted_at DESC, transaction.id DESC, entry.line_number
	`, parsedUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("read wallet transactions: %w", err)
	}
	defer rows.Close()

	transactions := make([]Transaction, 0)
	byID := make(map[ledger.JournalID]int)
	for rows.Next() {
		var id string
		var reference ledger.Reference
		var description string
		var occurredAt pgtype.Timestamptz
		var postedAt pgtype.Timestamptz
		var reversed bool
		var accountID string
		var accountKind ledger.AccountKind
		var side ledger.Side
		var amountMinor int64
		var currency string
		if err := rows.Scan(
			&id,
			&reference.Type,
			&reference.ID,
			&description,
			&occurredAt,
			&postedAt,
			&reversed,
			&accountID,
			&accountKind,
			&side,
			&amountMinor,
			&currency,
		); err != nil {
			return nil, fmt.Errorf("scan wallet transaction: %w", err)
		}
		amount, err := money.New(amountMinor, currency)
		if err != nil {
			return nil, err
		}
		journalID := ledger.JournalID(id)
		index, exists := byID[journalID]
		if !exists {
			index = len(transactions)
			byID[journalID] = index
			transactions = append(transactions, Transaction{
				ID:          journalID,
				Reference:   reference,
				Description: description,
				OccurredAt:  occurredAt.Time.UTC(),
				PostedAt:    postedAt.Time.UTC(),
				Reversed:    reversed,
			})
		}
		transactions[index].Entries = append(transactions[index].Entries, TransactionEntry{
			AccountID:   ledger.AccountID(accountID),
			AccountKind: accountKind,
			Side:        side,
			Amount:      amount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wallet transactions: %w", err)
	}
	return transactions, nil
}

func NewPostgresProvisioningStore(pool *pgxpool.Pool) *PostgresProvisioningStore {
	return &PostgresProvisioningStore{pool: pool}
}

func (s *PostgresProvisioningStore) Create(
	ctx context.Context,
	idempotencyKey string,
	provisioning Provisioning,
) (Provisioning, bool, error) {
	if s == nil || s.pool == nil {
		return Provisioning{}, false, ErrStoreUnavailable
	}
	if idempotencyKey == "" {
		return Provisioning{}, false, ErrInvalidIdempotencyKey
	}

	transaction, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Provisioning{}, false, fmt.Errorf("begin wallet provisioning: %w", err)
	}
	defer func() {
		_ = transaction.Rollback(context.Background())
	}()

	if _, err := transaction.Exec(
		ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended($1, 0))",
		onboardingIdempotencyScope+":"+idempotencyKey,
	); err != nil {
		return Provisioning{}, false, fmt.Errorf("lock onboarding key: %w", err)
	}

	requestHash := onboardingHash(provisioning)
	stored, found, err := loadIdempotentProvisioning(ctx, transaction, idempotencyKey, requestHash)
	if err != nil {
		return Provisioning{}, false, err
	}
	if found {
		return stored, true, nil
	}

	if err := insertProvisioning(ctx, transaction, provisioning); err != nil {
		return Provisioning{}, false, err
	}
	responseBody, err := json.Marshal(struct {
		UserID string `json:"userId"`
	}{UserID: string(provisioning.user.ID())})
	if err != nil {
		return Provisioning{}, false, fmt.Errorf("encode onboarding response: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO idempotency_keys (scope, key, request_hash, response_status, response_body)
		VALUES ($1, $2, $3, 201, $4)
	`, onboardingIdempotencyScope, idempotencyKey, requestHash, responseBody); err != nil {
		return Provisioning{}, false, mapProvisioningError(err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return Provisioning{}, false, mapProvisioningError(err)
	}
	return provisioning, false, nil
}

func (s *PostgresProvisioningStore) ByUser(
	ctx context.Context,
	userID identity.UserID,
) (Provisioning, error) {
	if s == nil || s.pool == nil {
		return Provisioning{}, ErrStoreUnavailable
	}
	return loadProvisioning(ctx, s.pool, userID)
}

type provisioningQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadIdempotentProvisioning(
	ctx context.Context,
	transaction pgx.Tx,
	idempotencyKey string,
	requestHash string,
) (Provisioning, bool, error) {
	var storedHash string
	var responseBody []byte
	err := transaction.QueryRow(ctx, `
		SELECT request_hash, response_body
		  FROM idempotency_keys
		 WHERE scope = $1 AND key = $2
	`, onboardingIdempotencyScope, idempotencyKey).Scan(&storedHash, &responseBody)
	if errors.Is(err, pgx.ErrNoRows) {
		return Provisioning{}, false, nil
	}
	if err != nil {
		return Provisioning{}, false, fmt.Errorf("read onboarding key: %w", err)
	}
	if storedHash != requestHash {
		return Provisioning{}, false, ErrIdempotencyConflict
	}

	var response struct {
		UserID string `json:"userId"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil || response.UserID == "" {
		return Provisioning{}, false, fmt.Errorf("decode onboarding response: %w", err)
	}
	stored, err := loadProvisioning(ctx, transaction, identity.UserID(response.UserID))
	return stored, true, err
}

func insertProvisioning(ctx context.Context, transaction pgx.Tx, provisioning Provisioning) error {
	userID, err := walletUUID(string(provisioning.user.ID()))
	if err != nil {
		return err
	}
	walletID, err := walletUUID(string(provisioning.wallet.ID()))
	if err != nil {
		return err
	}

	if _, err := transaction.Exec(ctx, `
		INSERT INTO users (id, email, display_name, created_at)
		VALUES ($1, $2, $3, $4)
	`, userID, provisioning.user.Email(), provisioning.user.DisplayName(), provisioning.user.CreatedAt()); err != nil {
		return mapProvisioningError(err)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO wallets (id, owner_id, currency, created_at)
		VALUES ($1, $2, $3, $4)
	`, walletID, userID, provisioning.wallet.Currency(), provisioning.wallet.CreatedAt()); err != nil {
		return mapProvisioningError(err)
	}
	for _, account := range provisioning.accounts {
		accountID, err := walletUUID(string(account.ID()))
		if err != nil {
			return err
		}
		if _, err := transaction.Exec(ctx, `
			INSERT INTO ledger_accounts (id, wallet_id, kind, normal_side, currency, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, accountID, walletID, account.Kind(), account.NormalSide(), account.Currency(), provisioning.wallet.CreatedAt()); err != nil {
			return mapProvisioningError(err)
		}
	}
	return nil
}

func loadProvisioning(
	ctx context.Context,
	query provisioningQuerier,
	userID identity.UserID,
) (Provisioning, error) {
	parsedUserID, err := walletUUID(string(userID))
	if err != nil {
		return Provisioning{}, err
	}

	var storedUserID string
	var email string
	var displayName string
	var userCreatedAt pgtype.Timestamptz
	var walletID string
	var currency string
	var walletCreatedAt pgtype.Timestamptz
	err = query.QueryRow(ctx, `
		SELECT users.id::text,
		       users.email,
		       users.display_name,
		       users.created_at,
		       wallets.id::text,
		       wallets.currency,
		       wallets.created_at
		  FROM users
		  JOIN wallets ON wallets.owner_id = users.id
		 WHERE users.id = $1
	`, parsedUserID).Scan(
		&storedUserID,
		&email,
		&displayName,
		&userCreatedAt,
		&walletID,
		&currency,
		&walletCreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Provisioning{}, ErrProvisioningNotFound
	}
	if err != nil {
		return Provisioning{}, fmt.Errorf("read wallet provisioning: %w", err)
	}

	user, err := identity.NewUser(identity.UserID(storedUserID), email, displayName, userCreatedAt.Time)
	if err != nil {
		return Provisioning{}, err
	}

	rows, err := query.Query(ctx, `
		SELECT id::text, kind, normal_side, currency
		  FROM ledger_accounts
		 WHERE wallet_id = $1::text::uuid
		   AND kind IN ('AVAILABLE', 'PENDING', 'COMMITMENT_RESERVE')
		 ORDER BY kind
	`, walletID)
	if err != nil {
		return Provisioning{}, fmt.Errorf("read wallet accounts: %w", err)
	}
	defer rows.Close()

	var accounts []ledger.Account
	accountSet := AccountSet{}
	for rows.Next() {
		var accountID string
		var kind ledger.AccountKind
		var normalSide ledger.Side
		var accountCurrency string
		if err := rows.Scan(&accountID, &kind, &normalSide, &accountCurrency); err != nil {
			return Provisioning{}, fmt.Errorf("scan wallet account: %w", err)
		}
		account, err := ledger.NewAccount(
			ledger.AccountID(accountID),
			ledger.OwnerID(storedUserID),
			kind,
			accountCurrency,
		)
		if err != nil || account.NormalSide() != normalSide {
			return Provisioning{}, ErrInvalidAccountSet
		}
		accounts = append(accounts, account)
		switch kind {
		case ledger.AccountKindAvailable:
			accountSet.Available = account.ID()
		case ledger.AccountKindPending:
			accountSet.Pending = account.ID()
		case ledger.AccountKindCommitmentReserve:
			accountSet.CommitmentReserve = account.ID()
		}
	}
	if err := rows.Err(); err != nil {
		return Provisioning{}, fmt.Errorf("iterate wallet accounts: %w", err)
	}

	wallet, err := NewWallet(
		WalletID(walletID),
		user.ID(),
		currency,
		accountSet,
		walletCreatedAt.Time,
	)
	if err != nil {
		return Provisioning{}, err
	}
	return newProvisioning(user, wallet, accounts)
}

func onboardingHash(provisioning Provisioning) string {
	value := provisioning.user.Email() + "\x00" +
		provisioning.user.DisplayName() + "\x00" +
		provisioning.wallet.Currency()
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func walletUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil || !parsed.Valid {
		return pgtype.UUID{}, fmt.Errorf("%w: %q", ErrPersistenceID, value)
	}
	return parsed, nil
}

func mapProvisioningError(err error) error {
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) {
		return err
	}
	switch databaseError.ConstraintName {
	case "users_email_unique":
		return ErrUserExists
	case "wallets_owner_id_key":
		return ErrWalletExists
	case "users_pkey", "wallets_pkey", "ledger_accounts_pkey", "ledger_wallet_account_kind_unique":
		return ErrProvisioningExists
	case "idempotency_keys_pkey":
		return ErrIdempotencyConflict
	default:
		return err
	}
}
