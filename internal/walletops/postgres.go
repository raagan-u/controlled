package walletops

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/pay-pact/internal/custody"
	"github.com/raagan-u/pay-pact/internal/identity"
	"github.com/raagan-u/pay-pact/internal/ledger"
	"github.com/raagan-u/pay-pact/internal/money"
)

type PostgresRepository struct {
	pool   *pgxpool.Pool
	ledger *ledger.PostgresBook
}

func NewPostgresRepository(pool *pgxpool.Pool, book *ledger.PostgresBook) *PostgresRepository {
	return &PostgresRepository{pool: pool, ledger: book}
}

func (r *PostgresRepository) CreateFunding(
	ctx context.Context,
	key string,
	candidate FundingIntent,
) (FundingIntent, error) {
	if r == nil || r.pool == nil {
		return FundingIntent{}, ErrUnavailable
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return FundingIntent{}, err
	}
	defer rollback(tx)
	if err := lockKey(ctx, tx, "funding:"+string(candidate.OwnerID)+":"+key); err != nil {
		return FundingIntent{}, err
	}

	existing, err := loadFundingByKey(ctx, tx, candidate.OwnerID, key, true)
	if err == nil {
		if !existing.Amount.Equal(candidate.Amount) {
			return FundingIntent{}, ErrIdempotencyConflict
		}
		existing.Duplicate = true
		return existing, nil
	}
	if !errors.Is(err, ErrOperationNotFound) {
		return FundingIntent{}, err
	}

	walletID, currency, err := walletForOwner(ctx, tx, candidate.OwnerID)
	if err != nil {
		return FundingIntent{}, err
	}
	if candidate.Amount.Currency() != currency {
		return FundingIntent{}, ErrCurrencyMismatch
	}
	parsedID, err := operationUUID(string(candidate.ID))
	if err != nil {
		return FundingIntent{}, err
	}
	ownerID, err := operationUUID(string(candidate.OwnerID))
	if err != nil {
		return FundingIntent{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO funding_intents (
			id, owner_id, wallet_id, idempotency_key, amount_minor, currency,
			status, created_at, updated_at
		) VALUES ($1, $2, $3::text::uuid, $4, $5, $6, 'CREATED', $7, $7)
	`, parsedID, ownerID, walletID, key, candidate.Amount.AmountMinor(), currency, candidate.CreatedAt)
	if err != nil {
		return FundingIntent{}, mapOperationError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return FundingIntent{}, mapOperationError(err)
	}
	candidate.WalletID = walletID
	candidate.Status = FundingCreated
	candidate.UpdatedAt = candidate.CreatedAt
	return candidate, nil
}

func (r *PostgresRepository) AttachFundingOrder(
	ctx context.Context,
	ownerID identity.UserID,
	id OperationID,
	order custody.FundingOrder,
	now time.Time,
) (FundingIntent, error) {
	if r == nil || r.pool == nil {
		return FundingIntent{}, ErrUnavailable
	}
	parsedID, err := operationUUID(string(id))
	if err != nil {
		return FundingIntent{}, err
	}
	result, err := r.pool.Exec(ctx, `
		UPDATE funding_intents
		   SET provider_order_id = $3,
		       approval_url = $4,
		       status = 'AWAITING_CONFIRMATION',
		       updated_at = $5
		 WHERE id = $1
		   AND owner_id = $2::text::uuid
		   AND status = 'CREATED'
	`, parsedID, ownerID, order.ID, order.ApprovalURL, now)
	if err != nil {
		return FundingIntent{}, mapOperationError(err)
	}
	if result.RowsAffected() == 0 {
		existing, loadErr := loadFunding(ctx, r.pool, ownerID, id, false)
		if loadErr != nil {
			return FundingIntent{}, loadErr
		}
		if existing.ProviderOrderID != order.ID || !existing.Amount.Equal(order.Amount) {
			return FundingIntent{}, ErrInvalidState
		}
		return existing, nil
	}
	return loadFunding(ctx, r.pool, ownerID, id, false)
}

func (r *PostgresRepository) Funding(
	ctx context.Context,
	ownerID identity.UserID,
	id OperationID,
) (FundingIntent, error) {
	if r == nil || r.pool == nil {
		return FundingIntent{}, ErrUnavailable
	}
	return loadFunding(ctx, r.pool, ownerID, id, false)
}

func (r *PostgresRepository) CompleteFunding(
	ctx context.Context,
	ownerID identity.UserID,
	id OperationID,
	providerOrderID string,
	journalID ledger.JournalID,
	custodyAccountCandidate ledger.AccountID,
	now time.Time,
) (FundingIntent, error) {
	if r == nil || r.pool == nil || r.ledger == nil {
		return FundingIntent{}, ErrUnavailable
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return FundingIntent{}, err
	}
	defer rollback(tx)
	intent, err := loadFunding(ctx, tx, ownerID, id, true)
	if err != nil {
		return FundingIntent{}, err
	}
	if intent.Status == FundingCompleted {
		intent.Duplicate = true
		return intent, nil
	}
	if intent.Status != FundingAwaitingConfirmation || intent.ProviderOrderID != providerOrderID {
		return FundingIntent{}, ErrInvalidState
	}

	custodyAccount, err := ensureCustodyAccount(ctx, tx, custodyAccountCandidate, intent.Amount.Currency(), now)
	if err != nil {
		return FundingIntent{}, err
	}
	availableAccount, _, err := accountForWallet(ctx, tx, intent.WalletID, ledger.AccountKindAvailable)
	if err != nil {
		return FundingIntent{}, err
	}
	debit, err := ledger.NewEntry(custodyAccount, ledger.SideDebit, intent.Amount)
	if err != nil {
		return FundingIntent{}, err
	}
	credit, err := ledger.NewEntry(availableAccount, ledger.SideCredit, intent.Amount)
	if err != nil {
		return FundingIntent{}, err
	}
	journal, err := ledger.NewJournal(
		journalID,
		"funding:"+string(id),
		ledger.Reference{Type: "FUNDING", ID: string(id)},
		"Wallet funding",
		now,
		[]ledger.Entry{debit, credit},
	)
	if err != nil {
		return FundingIntent{}, err
	}
	posted, err := r.ledger.PostInTx(ctx, tx, journal)
	if err != nil {
		return FundingIntent{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE funding_intents
		   SET status = 'COMPLETED',
		       ledger_transaction_id = $2::text::uuid,
		       updated_at = $3,
		       completed_at = $3
		 WHERE id = $1::text::uuid
	`, string(id), string(posted.JournalID), now)
	if err != nil {
		return FundingIntent{}, mapOperationError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return FundingIntent{}, mapOperationError(err)
	}
	intent.Status = FundingCompleted
	intent.LedgerTransactionID = posted.JournalID
	intent.UpdatedAt = now
	intent.CompletedAt = now
	return intent, nil
}

func (r *PostgresRepository) CreateTransfer(
	ctx context.Context,
	key string,
	candidate Transfer,
	journalID ledger.JournalID,
) (Transfer, error) {
	if r == nil || r.pool == nil || r.ledger == nil {
		return Transfer{}, ErrUnavailable
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return Transfer{}, err
	}
	defer rollback(tx)
	if err := lockKey(ctx, tx, "transfer:"+string(candidate.SenderID)+":"+key); err != nil {
		return Transfer{}, err
	}

	existing, err := loadTransferByKey(ctx, tx, candidate.SenderID, key)
	if err == nil {
		if existing.RecipientEmail != candidate.RecipientEmail || !existing.Amount.Equal(candidate.Amount) {
			return Transfer{}, ErrIdempotencyConflict
		}
		existing.Duplicate = true
		return existing, nil
	}
	if !errors.Is(err, ErrOperationNotFound) {
		return Transfer{}, err
	}

	senderWallet, currency, err := walletForOwner(ctx, tx, candidate.SenderID)
	if err != nil {
		return Transfer{}, err
	}
	if currency != candidate.Amount.Currency() {
		return Transfer{}, ErrCurrencyMismatch
	}
	var recipientID string
	var recipientWallet string
	var recipientCurrency string
	err = tx.QueryRow(ctx, `
		SELECT users.id::text, wallets.id::text, wallets.currency
		  FROM users
		  JOIN wallets ON wallets.owner_id = users.id
		 WHERE users.email = $1
	`, candidate.RecipientEmail).Scan(&recipientID, &recipientWallet, &recipientCurrency)
	if errors.Is(err, pgx.ErrNoRows) {
		return Transfer{}, ErrRecipientNotFound
	}
	if err != nil {
		return Transfer{}, fmt.Errorf("read transfer recipient: %w", err)
	}
	if recipientID == string(candidate.SenderID) {
		return Transfer{}, ErrSelfTransfer
	}
	if recipientCurrency != currency {
		return Transfer{}, ErrCurrencyMismatch
	}

	senderAvailable, _, err := accountForWallet(ctx, tx, senderWallet, ledger.AccountKindAvailable)
	if err != nil {
		return Transfer{}, err
	}
	recipientAvailable, _, err := accountForWallet(ctx, tx, recipientWallet, ledger.AccountKindAvailable)
	if err != nil {
		return Transfer{}, err
	}
	debit, err := ledger.NewEntry(senderAvailable, ledger.SideDebit, candidate.Amount)
	if err != nil {
		return Transfer{}, err
	}
	credit, err := ledger.NewEntry(recipientAvailable, ledger.SideCredit, candidate.Amount)
	if err != nil {
		return Transfer{}, err
	}
	journal, err := ledger.NewJournal(
		journalID,
		"internal-transfer:"+string(candidate.ID),
		ledger.Reference{Type: "INTERNAL_TRANSFER", ID: string(candidate.ID)},
		"PayPact internal transfer",
		candidate.CreatedAt,
		[]ledger.Entry{debit, credit},
	)
	if err != nil {
		return Transfer{}, err
	}
	posted, err := r.ledger.PostInTx(ctx, tx, journal)
	if err != nil {
		return Transfer{}, err
	}
	parsedID, err := operationUUID(string(candidate.ID))
	if err != nil {
		return Transfer{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO internal_transfers (
			id, sender_wallet_id, recipient_wallet_id, idempotency_key,
			amount_minor, currency, ledger_transaction_id, created_at
		) VALUES ($1, $2::text::uuid, $3::text::uuid, $4, $5, $6, $7::text::uuid, $8)
	`, parsedID, senderWallet, recipientWallet, key, candidate.Amount.AmountMinor(), currency, string(posted.JournalID), candidate.CreatedAt)
	if err != nil {
		return Transfer{}, mapOperationError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Transfer{}, mapOperationError(err)
	}
	candidate.RecipientID = identity.UserID(recipientID)
	candidate.LedgerTransactionID = posted.JournalID
	return candidate, nil
}

func (r *PostgresRepository) ReserveWithdrawal(
	ctx context.Context,
	key string,
	candidate Withdrawal,
	journalID ledger.JournalID,
) (Withdrawal, error) {
	if r == nil || r.pool == nil || r.ledger == nil {
		return Withdrawal{}, ErrUnavailable
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return Withdrawal{}, err
	}
	defer rollback(tx)
	if err := lockKey(ctx, tx, "withdrawal:"+string(candidate.OwnerID)+":"+key); err != nil {
		return Withdrawal{}, err
	}

	existing, err := loadWithdrawalByKey(ctx, tx, candidate.OwnerID, key, true)
	if err == nil {
		if existing.Destination != candidate.Destination || !existing.Amount.Equal(candidate.Amount) {
			return Withdrawal{}, ErrIdempotencyConflict
		}
		existing.Duplicate = true
		return existing, nil
	}
	if !errors.Is(err, ErrOperationNotFound) {
		return Withdrawal{}, err
	}

	walletID, currency, err := walletForOwner(ctx, tx, candidate.OwnerID)
	if err != nil {
		return Withdrawal{}, err
	}
	if candidate.Amount.Currency() != currency {
		return Withdrawal{}, ErrCurrencyMismatch
	}
	availableAccount, _, err := accountForWallet(ctx, tx, walletID, ledger.AccountKindAvailable)
	if err != nil {
		return Withdrawal{}, err
	}
	pendingAccount, _, err := accountForWallet(ctx, tx, walletID, ledger.AccountKindPending)
	if err != nil {
		return Withdrawal{}, err
	}
	debit, err := ledger.NewEntry(availableAccount, ledger.SideDebit, candidate.Amount)
	if err != nil {
		return Withdrawal{}, err
	}
	credit, err := ledger.NewEntry(pendingAccount, ledger.SideCredit, candidate.Amount)
	if err != nil {
		return Withdrawal{}, err
	}
	journal, err := ledger.NewJournal(
		journalID,
		"withdrawal-reserve:"+string(candidate.ID),
		ledger.Reference{Type: "WITHDRAWAL_RESERVATION", ID: string(candidate.ID)},
		"Reserve withdrawal funds",
		candidate.CreatedAt,
		[]ledger.Entry{debit, credit},
	)
	if err != nil {
		return Withdrawal{}, err
	}
	posted, err := r.ledger.PostInTx(ctx, tx, journal)
	if err != nil {
		return Withdrawal{}, err
	}
	parsedID, err := operationUUID(string(candidate.ID))
	if err != nil {
		return Withdrawal{}, err
	}
	ownerID, err := operationUUID(string(candidate.OwnerID))
	if err != nil {
		return Withdrawal{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO withdrawals (
			id, owner_id, wallet_id, idempotency_key, destination,
			amount_minor, currency, status, reservation_transaction_id,
			created_at, updated_at
		) VALUES ($1, $2, $3::text::uuid, $4, $5, $6, $7, 'RESERVED', $8::text::uuid, $9, $9)
	`, parsedID, ownerID, walletID, key, candidate.Destination, candidate.Amount.AmountMinor(), currency, string(posted.JournalID), candidate.CreatedAt)
	if err != nil {
		return Withdrawal{}, mapOperationError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Withdrawal{}, mapOperationError(err)
	}
	candidate.WalletID = walletID
	candidate.Status = WithdrawalReserved
	candidate.ReservationTransactionID = posted.JournalID
	candidate.UpdatedAt = candidate.CreatedAt
	return candidate, nil
}

func (r *PostgresRepository) AttachPayout(
	ctx context.Context,
	ownerID identity.UserID,
	id OperationID,
	payout custody.Payout,
	now time.Time,
) (Withdrawal, error) {
	if r == nil || r.pool == nil {
		return Withdrawal{}, ErrUnavailable
	}
	parsedID, err := operationUUID(string(id))
	if err != nil {
		return Withdrawal{}, err
	}
	result, err := r.pool.Exec(ctx, `
		UPDATE withdrawals
		   SET provider_payout_id = $3,
		       status = 'PENDING',
		       updated_at = $4
		 WHERE id = $1
		   AND owner_id = $2::text::uuid
		   AND status = 'RESERVED'
	`, parsedID, ownerID, payout.ID, now)
	if err != nil {
		return Withdrawal{}, mapOperationError(err)
	}
	if result.RowsAffected() == 0 {
		existing, loadErr := loadWithdrawal(ctx, r.pool, ownerID, id, false)
		if loadErr != nil {
			return Withdrawal{}, loadErr
		}
		if existing.ProviderPayoutID != payout.ID || !existing.Amount.Equal(payout.Amount) {
			return Withdrawal{}, ErrInvalidState
		}
		return existing, nil
	}
	return loadWithdrawal(ctx, r.pool, ownerID, id, false)
}

func (r *PostgresRepository) Withdrawal(
	ctx context.Context,
	ownerID identity.UserID,
	id OperationID,
) (Withdrawal, error) {
	if r == nil || r.pool == nil {
		return Withdrawal{}, ErrUnavailable
	}
	return loadWithdrawal(ctx, r.pool, ownerID, id, false)
}

func (r *PostgresRepository) SettleWithdrawal(
	ctx context.Context,
	ownerID identity.UserID,
	id OperationID,
	succeeded bool,
	journalID ledger.JournalID,
	custodyAccountCandidate ledger.AccountID,
	now time.Time,
) (Withdrawal, error) {
	if r == nil || r.pool == nil || r.ledger == nil {
		return Withdrawal{}, ErrUnavailable
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return Withdrawal{}, err
	}
	defer rollback(tx)
	withdrawal, err := loadWithdrawal(ctx, tx, ownerID, id, true)
	if err != nil {
		return Withdrawal{}, err
	}
	if withdrawal.Status == WithdrawalCompleted || withdrawal.Status == WithdrawalFailed {
		withdrawal.Duplicate = true
		return withdrawal, nil
	}
	if withdrawal.Status != WithdrawalPending || withdrawal.ProviderPayoutID == "" {
		return Withdrawal{}, ErrInvalidState
	}
	pendingAccount, _, err := accountForWallet(ctx, tx, withdrawal.WalletID, ledger.AccountKindPending)
	if err != nil {
		return Withdrawal{}, err
	}
	debit, err := ledger.NewEntry(pendingAccount, ledger.SideDebit, withdrawal.Amount)
	if err != nil {
		return Withdrawal{}, err
	}
	var credit ledger.Entry
	var referenceType string
	var description string
	status := WithdrawalFailed
	if succeeded {
		custodyAccount, err := ensureCustodyAccount(ctx, tx, custodyAccountCandidate, withdrawal.Amount.Currency(), now)
		if err != nil {
			return Withdrawal{}, err
		}
		credit, err = ledger.NewEntry(custodyAccount, ledger.SideCredit, withdrawal.Amount)
		if err != nil {
			return Withdrawal{}, err
		}
		referenceType = "WITHDRAWAL_SETTLEMENT"
		description = "Settle withdrawal"
		status = WithdrawalCompleted
	} else {
		availableAccount, _, err := accountForWallet(ctx, tx, withdrawal.WalletID, ledger.AccountKindAvailable)
		if err != nil {
			return Withdrawal{}, err
		}
		credit, err = ledger.NewEntry(availableAccount, ledger.SideCredit, withdrawal.Amount)
		if err != nil {
			return Withdrawal{}, err
		}
		referenceType = "WITHDRAWAL_RELEASE"
		description = "Release failed withdrawal"
	}
	journal, err := ledger.NewJournal(
		journalID,
		"withdrawal-settle:"+string(id),
		ledger.Reference{Type: referenceType, ID: string(id)},
		description,
		now,
		[]ledger.Entry{debit, credit},
	)
	if err != nil {
		return Withdrawal{}, err
	}
	posted, err := r.ledger.PostInTx(ctx, tx, journal)
	if err != nil {
		return Withdrawal{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE withdrawals
		   SET status = $2,
		       settlement_transaction_id = $3::text::uuid,
		       updated_at = $4,
		       completed_at = $4
		 WHERE id = $1::text::uuid
	`, string(id), status, string(posted.JournalID), now)
	if err != nil {
		return Withdrawal{}, mapOperationError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Withdrawal{}, mapOperationError(err)
	}
	withdrawal.Status = status
	withdrawal.SettlementTransactionID = posted.JournalID
	withdrawal.UpdatedAt = now
	withdrawal.CompletedAt = now
	return withdrawal, nil
}

func (r *PostgresRepository) begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("begin wallet operation: %w", err)
	}
	return tx, nil
}

func rollback(tx pgx.Tx) {
	_ = tx.Rollback(context.Background())
}

func lockKey(ctx context.Context, tx pgx.Tx, key string) error {
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", key); err != nil {
		return fmt.Errorf("lock wallet operation: %w", err)
	}
	return nil
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func walletForOwner(ctx context.Context, query rowQuerier, ownerID identity.UserID) (string, string, error) {
	parsedOwnerID, err := operationUUID(string(ownerID))
	if err != nil {
		return "", "", err
	}
	var walletID string
	var currency string
	err = query.QueryRow(ctx, "SELECT id::text, currency FROM wallets WHERE owner_id = $1", parsedOwnerID).Scan(&walletID, &currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrOperationNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("read operation wallet: %w", err)
	}
	return walletID, currency, nil
}

func accountForWallet(
	ctx context.Context,
	query rowQuerier,
	walletID string,
	kind ledger.AccountKind,
) (ledger.AccountID, string, error) {
	var accountID string
	var currency string
	err := query.QueryRow(ctx, `
		SELECT id::text, currency
		  FROM ledger_accounts
		 WHERE wallet_id = $1::text::uuid AND kind = $2
	`, walletID, kind).Scan(&accountID, &currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ledger.ErrAccountNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("read wallet ledger account: %w", err)
	}
	return ledger.AccountID(accountID), currency, nil
}

func ensureCustodyAccount(
	ctx context.Context,
	tx pgx.Tx,
	candidate ledger.AccountID,
	currency string,
	now time.Time,
) (ledger.AccountID, error) {
	parsedCandidate, err := operationUUID(string(candidate))
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO ledger_accounts (id, kind, normal_side, currency, created_at)
		VALUES ($1, 'CUSTODY_ASSET', 'DEBIT', $2, $3)
		ON CONFLICT DO NOTHING
	`, parsedCandidate, currency, now)
	if err != nil {
		return "", fmt.Errorf("ensure custody account: %w", err)
	}
	var id string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		  FROM ledger_accounts
		 WHERE wallet_id IS NULL AND kind = 'CUSTODY_ASSET' AND currency = $1
	`, currency).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("read custody account: %w", err)
	}
	return ledger.AccountID(id), nil
}

func loadFundingByKey(
	ctx context.Context,
	query rowQuerier,
	ownerID identity.UserID,
	key string,
	forUpdate bool,
) (FundingIntent, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	return scanFunding(query.QueryRow(ctx, `
		SELECT id::text, owner_id::text, wallet_id::text, amount_minor, currency,
		       status, COALESCE(provider_order_id, ''), COALESCE(approval_url, ''),
		       COALESCE(ledger_transaction_id::text, ''), created_at, updated_at, completed_at
		  FROM funding_intents
		 WHERE owner_id = $1::text::uuid AND idempotency_key = $2`+suffix, string(ownerID), key))
}

func loadFunding(
	ctx context.Context,
	query rowQuerier,
	ownerID identity.UserID,
	id OperationID,
	forUpdate bool,
) (FundingIntent, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	return scanFunding(query.QueryRow(ctx, `
		SELECT id::text, owner_id::text, wallet_id::text, amount_minor, currency,
		       status, COALESCE(provider_order_id, ''), COALESCE(approval_url, ''),
		       COALESCE(ledger_transaction_id::text, ''), created_at, updated_at, completed_at
		  FROM funding_intents
		 WHERE owner_id = $1::text::uuid AND id = $2::text::uuid`+suffix, string(ownerID), string(id)))
}

func scanFunding(row pgx.Row) (FundingIntent, error) {
	var result FundingIntent
	var id string
	var ownerID string
	var amountMinor int64
	var currency string
	var createdAt pgtype.Timestamptz
	var updatedAt pgtype.Timestamptz
	var completedAt pgtype.Timestamptz
	err := row.Scan(
		&id, &ownerID, &result.WalletID, &amountMinor, &currency,
		&result.Status, &result.ProviderOrderID, &result.ApprovalURL,
		&result.LedgerTransactionID, &createdAt, &updatedAt, &completedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return FundingIntent{}, ErrOperationNotFound
	}
	if err != nil {
		return FundingIntent{}, fmt.Errorf("read funding intent: %w", err)
	}
	amount, err := money.New(amountMinor, currency)
	if err != nil {
		return FundingIntent{}, err
	}
	result.ID = OperationID(id)
	result.OwnerID = identity.UserID(ownerID)
	result.Amount = amount
	result.CreatedAt = createdAt.Time
	result.UpdatedAt = updatedAt.Time
	if completedAt.Valid {
		result.CompletedAt = completedAt.Time
	}
	return result, nil
}

func loadTransferByKey(
	ctx context.Context,
	query rowQuerier,
	senderID identity.UserID,
	key string,
) (Transfer, error) {
	var result Transfer
	var id string
	var storedSenderID string
	var recipientID string
	var amountMinor int64
	var currency string
	var createdAt pgtype.Timestamptz
	err := query.QueryRow(ctx, `
		SELECT transfer.id::text,
		       sender.owner_id::text,
		       recipient.owner_id::text,
		       recipient_user.email,
		       transfer.amount_minor,
		       transfer.currency,
		       transfer.ledger_transaction_id::text,
		       transfer.created_at
		  FROM internal_transfers AS transfer
		  JOIN wallets AS sender ON sender.id = transfer.sender_wallet_id
		  JOIN wallets AS recipient ON recipient.id = transfer.recipient_wallet_id
		  JOIN users AS recipient_user ON recipient_user.id = recipient.owner_id
		 WHERE sender.owner_id = $1::text::uuid AND transfer.idempotency_key = $2
		 FOR UPDATE OF transfer
	`, string(senderID), key).Scan(
		&id, &storedSenderID, &recipientID, &result.RecipientEmail,
		&amountMinor, &currency, &result.LedgerTransactionID, &createdAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Transfer{}, ErrOperationNotFound
	}
	if err != nil {
		return Transfer{}, fmt.Errorf("read internal transfer: %w", err)
	}
	amount, err := money.New(amountMinor, currency)
	if err != nil {
		return Transfer{}, err
	}
	result.ID = OperationID(id)
	result.SenderID = identity.UserID(storedSenderID)
	result.RecipientID = identity.UserID(recipientID)
	result.Amount = amount
	result.CreatedAt = createdAt.Time
	return result, nil
}

func loadWithdrawalByKey(
	ctx context.Context,
	query rowQuerier,
	ownerID identity.UserID,
	key string,
	forUpdate bool,
) (Withdrawal, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	return scanWithdrawal(query.QueryRow(ctx, withdrawalSelect+` WHERE owner_id = $1::text::uuid AND idempotency_key = $2`+suffix, string(ownerID), key))
}

func loadWithdrawal(
	ctx context.Context,
	query rowQuerier,
	ownerID identity.UserID,
	id OperationID,
	forUpdate bool,
) (Withdrawal, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	return scanWithdrawal(query.QueryRow(ctx, withdrawalSelect+` WHERE owner_id = $1::text::uuid AND id = $2::text::uuid`+suffix, string(ownerID), string(id)))
}

const withdrawalSelect = `
	SELECT id::text, owner_id::text, wallet_id::text, destination,
	       amount_minor, currency, status, COALESCE(provider_payout_id, ''),
	       reservation_transaction_id::text,
	       COALESCE(settlement_transaction_id::text, ''),
	       created_at, updated_at, completed_at
	  FROM withdrawals`

func scanWithdrawal(row pgx.Row) (Withdrawal, error) {
	var result Withdrawal
	var id string
	var ownerID string
	var amountMinor int64
	var currency string
	var createdAt pgtype.Timestamptz
	var updatedAt pgtype.Timestamptz
	var completedAt pgtype.Timestamptz
	err := row.Scan(
		&id, &ownerID, &result.WalletID, &result.Destination,
		&amountMinor, &currency, &result.Status, &result.ProviderPayoutID,
		&result.ReservationTransactionID, &result.SettlementTransactionID,
		&createdAt, &updatedAt, &completedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Withdrawal{}, ErrOperationNotFound
	}
	if err != nil {
		return Withdrawal{}, fmt.Errorf("read withdrawal: %w", err)
	}
	amount, err := money.New(amountMinor, currency)
	if err != nil {
		return Withdrawal{}, err
	}
	result.ID = OperationID(id)
	result.OwnerID = identity.UserID(ownerID)
	result.Amount = amount
	result.CreatedAt = createdAt.Time
	result.UpdatedAt = updatedAt.Time
	if completedAt.Valid {
		result.CompletedAt = completedAt.Time
	}
	return result, nil
}

func operationUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil || !parsed.Valid {
		return pgtype.UUID{}, fmt.Errorf("%w: %q", ErrPersistenceID, value)
	}
	return parsed, nil
}

func mapOperationError(err error) error {
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) {
		return err
	}
	switch databaseError.ConstraintName {
	case "funding_intents_owner_id_idempotency_key_key",
		"internal_transfers_sender_wallet_id_idempotency_key_key",
		"withdrawals_owner_id_idempotency_key_key":
		return ErrIdempotencyConflict
	case "funding_intents_pkey", "internal_transfers_pkey", "withdrawals_pkey":
		return ErrInvalidOperation
	default:
		return err
	}
}

func normalizedEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
