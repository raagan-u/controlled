package walletops

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/money"
)

func ensureAvailableBalance(
	ctx context.Context,
	tx pgx.Tx,
	accountID ledger.AccountID,
	amount money.Money,
) error {
	var currency string
	if err := tx.QueryRow(ctx, `
		SELECT currency
		  FROM ledger_accounts
		 WHERE id = $1::text::uuid AND kind = 'AVAILABLE'
		 FOR UPDATE
	`, string(accountID)).Scan(&currency); err != nil {
		return fmt.Errorf("lock available account: %w", err)
	}
	if currency != amount.Currency() {
		return ErrCurrencyMismatch
	}
	var balance int64
	if err := tx.QueryRow(ctx, `
		SELECT amount_minor
		  FROM ledger_account_balances
		 WHERE account_id = $1::text::uuid
	`, string(accountID)).Scan(&balance); err != nil {
		return fmt.Errorf("read available balance: %w", err)
	}
	if balance < amount.AmountMinor() {
		return ledger.ErrInsufficientBalance
	}
	return nil
}

func evaluatePactPolicy(
	ctx context.Context,
	tx pgx.Tx,
	ownerID string,
	amount money.Money,
	at time.Time,
) ([]string, []PactBlock, error) {
	rows, err := tx.Query(ctx, `
		SELECT pact.id::text,
		       pact.spending_limit_minor,
		       COALESCE((
		           SELECT sum(usage.amount_minor)
		             FROM pact_transaction_usage AS usage
		            WHERE usage.pact_id = pact.id AND usage.status = 'COUNTED'
		       ), 0)::bigint
		  FROM pacts AS pact
		 WHERE pact.owner_id = $1::text::uuid
		   AND pact.status IN ('SCHEDULED', 'ACTIVE')
		   AND pact.starts_at <= $2
		   AND $2 < pact.ends_at
		 ORDER BY pact.id
		 FOR UPDATE OF pact
	`, ownerID, at)
	if err != nil {
		return nil, nil, fmt.Errorf("lock active pacts: %w", err)
	}
	defer rows.Close()

	var pactIDs []string
	var blocks []PactBlock
	for rows.Next() {
		var pactID string
		var limit, used int64
		if err := rows.Scan(&pactID, &limit, &used); err != nil {
			return nil, nil, fmt.Errorf("scan active pact: %w", err)
		}
		pactIDs = append(pactIDs, pactID)
		proposed := used + amount.AmountMinor()
		if proposed > limit {
			blocks = append(blocks, PactBlock{
				PactID: pactID, LimitMinor: limit, UsedMinor: used,
				ProposedMinor: proposed, OverageMinor: proposed - limit,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate active pacts: %w", err)
	}
	return pactIDs, blocks, nil
}

func recordPactUsage(
	ctx context.Context,
	tx pgx.Tx,
	pactIDs []string,
	ownerID string,
	kind DebitKind,
	operationID OperationID,
	amount money.Money,
	journalID ledger.JournalID,
	at time.Time,
) error {
	for _, pactID := range pactIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO pact_transaction_usage (
				pact_id, owner_id, operation_type, operation_id,
				amount_minor, currency, status, ledger_transaction_id, counted_at
			) VALUES ($1::text::uuid, $2::text::uuid, $3, $4::text::uuid, $5, $6, 'COUNTED', $7::text::uuid, $8)
			ON CONFLICT (pact_id, operation_type, operation_id) DO NOTHING
		`, pactID, ownerID, kind, string(operationID), amount.AmountMinor(), amount.Currency(), string(journalID), at)
		if err != nil {
			return fmt.Errorf("record pact usage: %w", err)
		}
	}
	return nil
}

func reverseWithdrawalPactUsage(
	ctx context.Context,
	tx pgx.Tx,
	operationID OperationID,
	journalID ledger.JournalID,
	at time.Time,
) error {
	result, err := tx.Exec(ctx, `
		UPDATE pact_transaction_usage
		   SET status = 'REVERSED', reversal_transaction_id = $2::text::uuid, reversed_at = $3
		 WHERE operation_type = 'WITHDRAWAL'
		   AND operation_id = $1::text::uuid
		   AND status = 'COUNTED'
	`, string(operationID), string(journalID), at)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("reverse withdrawal pact usage: %w", err)
	}
	_ = result
	return nil
}
