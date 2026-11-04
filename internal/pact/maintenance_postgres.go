package pact

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/raagan-u/controlled/internal/ledger"
)

func (r *PostgresRepository) StartDue(ctx context.Context, now time.Time) (int, error) {
	if r == nil || r.pool == nil {
		return 0, ErrUnavailable
	}
	result, err := r.pool.Exec(ctx, `
		UPDATE pacts
		   SET status = 'ACTIVE', version = version + 1, updated_at = $1
		 WHERE status = 'SCHEDULED' AND starts_at <= $1 AND $1 < ends_at
	`, now)
	if err != nil {
		return 0, fmt.Errorf("start due pacts: %w", err)
	}
	return int(result.RowsAffected()), nil
}

func (r *PostgresRepository) DueForCompletion(ctx context.Context, now time.Time) ([]PactID, error) {
	if r == nil || r.pool == nil {
		return nil, ErrUnavailable
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text FROM pacts
		 WHERE status IN ('SCHEDULED', 'ACTIVE') AND ends_at <= $1
		 ORDER BY ends_at, id
	`, now)
	if err != nil {
		return nil, fmt.Errorf("list due pacts: %w", err)
	}
	defer rows.Close()
	var ids []PactID
	for rows.Next() {
		var id PactID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *PostgresRepository) CompleteDue(
	ctx context.Context,
	pactID PactID,
	completionJournalID string,
	recoveryJournalID string,
	now time.Time,
) error {
	if r == nil || r.pool == nil || r.ledger == nil {
		return ErrUnavailable
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin pact completion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	parsedPactID, err := pactUUID(string(pactID))
	if err != nil {
		return err
	}
	current, err := scanPact(tx.QueryRow(ctx, pactSelect+` WHERE id = $1 FOR UPDATE`, parsedPactID))
	if err != nil {
		return err
	}
	if current.Status() == StatusResolved {
		return nil
	}
	if current.Status() != StatusActive && current.Status() != StatusScheduled {
		return ErrInvalidTransition
	}
	if now.Before(current.Terms().EndsAt()) {
		return ErrPeriodNotEnded
	}
	var amountMinor int64
	var currency string
	var stakeCreatedAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT amount_minor, currency, created_at FROM stake_allocations
		 WHERE pact_id = $1 AND status = 'ACTIVE' FOR UPDATE
	`, parsedPactID).Scan(&amountMinor, &currency, &stakeCreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidStakeReservation
	}
	if err != nil {
		return fmt.Errorf("lock due pact stake: %w", err)
	}
	amount, err := NewMoney(amountMinor, currency)
	if err != nil {
		return err
	}
	ownerID, err := pactUUID(string(current.OwnerID()))
	if err != nil {
		return err
	}
	available, vault, err := existingStakeAccounts(ctx, tx, ownerID, currency)
	if err != nil {
		return err
	}
	debit, _ := ledger.NewEntry(vault, ledger.SideDebit, amount)
	credit, _ := ledger.NewEntry(available, ledger.SideCredit, amount)
	journal, err := ledger.NewJournal(
		ledger.JournalID(completionJournalID), "pact-completion:"+string(pactID),
		ledger.Reference{Type: "PACT_COMPLETION", ID: string(pactID)},
		"Release completed pact stake", now, []ledger.Entry{debit, credit},
	)
	if err != nil {
		return err
	}
	posted, err := r.ledger.PostInTx(ctx, tx, journal)
	if err != nil {
		return err
	}
	if err := cancelPactRequests(ctx, tx, string(pactID), now); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE stake_allocations
		   SET status = 'RELEASED', resolution_transaction_id = $2::text::uuid, resolved_at = $3
		 WHERE pact_id = $1 AND status = 'ACTIVE'
	`, parsedPactID, string(posted.JournalID), now)
	if err != nil {
		return fmt.Errorf("release completed pact stake: %w", err)
	}
	if err := r.recoverCommitmentReserve(
		ctx, tx, pactID, ownerID, currency, amountMinor,
		stakeCreatedAt, recoveryJournalID, now,
	); err != nil {
		return err
	}
	outcome := OutcomeKept
	if current.EmergencyCount() > 0 {
		outcome = OutcomeKeptWithExceptions
	}
	_, err = tx.Exec(ctx, `
		UPDATE pacts SET status = 'RESOLVED', outcome = $2,
		       version = version + 1, updated_at = $3
		 WHERE id = $1 AND status IN ('SCHEDULED', 'ACTIVE')
	`, parsedPactID, outcome, now)
	if err != nil {
		return fmt.Errorf("complete due pact: %w", err)
	}
	return tx.Commit(ctx)
}

type recoveryApplication struct {
	sourcePactID string
	amountMinor  int64
}

func (r *PostgresRepository) recoverCommitmentReserve(
	ctx context.Context,
	tx pgx.Tx,
	recoveryPactID PactID,
	ownerID pgtype.UUID,
	currency string,
	maximumMinor int64,
	recoveryStakeCreatedAt time.Time,
	recoveryJournalID string,
	now time.Time,
) error {
	rows, err := tx.Query(ctx, `
		SELECT pact_id::text, amount_minor - recovered_minor
		  FROM stake_allocations
		 WHERE owner_id = $1
		   AND currency = $2
		   AND status = 'COMMITMENT_RESERVED'
		   AND recovered_minor < amount_minor
		   AND resolved_at < $3
		 ORDER BY resolved_at, pact_id
		 FOR UPDATE
	`, ownerID, currency, recoveryStakeCreatedAt)
	if err != nil {
		return fmt.Errorf("lock commitment recovery queue: %w", err)
	}
	remaining := maximumMinor
	applications := make([]recoveryApplication, 0)
	for rows.Next() && remaining > 0 {
		var item recoveryApplication
		var outstandingMinor int64
		if err := rows.Scan(&item.sourcePactID, &outstandingMinor); err != nil {
			return err
		}
		item.amountMinor = min(remaining, outstandingMinor)
		applications = append(applications, item)
		remaining -= item.amountMinor
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(applications) == 0 {
		return nil
	}

	recoveredMinor := maximumMinor - remaining
	recovered, err := NewMoney(recoveredMinor, currency)
	if err != nil {
		return err
	}
	available, reserve, err := existingRecoveryAccounts(ctx, tx, ownerID, currency)
	if err != nil {
		return err
	}
	debit, _ := ledger.NewEntry(reserve, ledger.SideDebit, recovered)
	credit, _ := ledger.NewEntry(available, ledger.SideCredit, recovered)
	journal, err := ledger.NewJournal(
		ledger.JournalID(recoveryJournalID), "pact-stake-recovery:"+string(recoveryPactID),
		ledger.Reference{Type: "PACT_STAKE_RECOVERY", ID: string(recoveryPactID)},
		"Recover stake from Commitment Reserve", now, []ledger.Entry{debit, credit},
	)
	if err != nil {
		return err
	}
	posted, err := r.ledger.PostInTx(ctx, tx, journal)
	if err != nil {
		return err
	}
	for _, application := range applications {
		result, err := tx.Exec(ctx, `
			UPDATE stake_allocations
			   SET recovered_minor = recovered_minor + $2,
			       fully_recovered_at = CASE
			           WHEN recovered_minor + $2 = amount_minor THEN $3
			           ELSE NULL
			       END
			 WHERE pact_id = $1::text::uuid
			   AND status = 'COMMITMENT_RESERVED'
			   AND recovered_minor + $2 <= amount_minor
		`, application.sourcePactID, application.amountMinor, now)
		if err != nil {
			return fmt.Errorf("apply commitment recovery: %w", err)
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("apply commitment recovery: %w", ErrInvalidStakeReservation)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO stake_recovery_applications (
				source_pact_id, recovery_pact_id, amount_minor, ledger_transaction_id, created_at
			) VALUES ($1::text::uuid, $2::text::uuid, $3, $4::text::uuid, $5)
		`, application.sourcePactID, string(recoveryPactID), application.amountMinor, string(posted.JournalID), now)
		if err != nil {
			return fmt.Errorf("record commitment recovery: %w", err)
		}
	}
	return nil
}

func existingRecoveryAccounts(
	ctx context.Context,
	tx pgx.Tx,
	ownerID pgtype.UUID,
	currency string,
) (ledger.AccountID, ledger.AccountID, error) {
	var available, reserve string
	err := tx.QueryRow(ctx, `
		SELECT available.id::text, reserve.id::text
		  FROM wallets AS wallet
		  JOIN ledger_accounts AS available
		    ON available.wallet_id = wallet.id AND available.kind = 'AVAILABLE'
		  JOIN ledger_accounts AS reserve
		    ON reserve.wallet_id = wallet.id AND reserve.kind = 'COMMITMENT_RESERVE'
		 WHERE wallet.owner_id = $1 AND wallet.currency = $2
	`, ownerID, currency).Scan(&available, &reserve)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ledger.ErrAccountNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("read commitment recovery accounts: %w", err)
	}
	return ledger.AccountID(available), ledger.AccountID(reserve), nil
}

func cancelPactRequests(ctx context.Context, tx pgx.Tx, pactID string, now time.Time) error {
	rows, err := tx.Query(ctx, `
		SELECT request.id::text, request.intent_id::text
		  FROM accountability_requests AS request
		  JOIN accountability_request_pacts AS blocked ON blocked.request_id = request.id
		 WHERE blocked.pact_id = $1::text::uuid AND request.status = 'ASSIGNED'
		 FOR UPDATE OF request
	`, pactID)
	if err != nil {
		return fmt.Errorf("lock pact accountability requests: %w", err)
	}
	type pending struct{ requestID, intentID string }
	var items []pending
	for rows.Next() {
		var item pending
		if err := rows.Scan(&item.requestID, &item.intentID); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range items {
		if _, err := tx.Exec(ctx, `UPDATE accountability_assignments SET status = 'SUPERSEDED', responded_at = $2 WHERE request_id = $1::text::uuid AND status = 'ACTIVE'`, item.requestID, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE accountability_requests SET status = 'CANCELLED', resolved_at = $2 WHERE id = $1::text::uuid`, item.requestID, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE transaction_intents SET status = 'CANCELLED', updated_at = $2 WHERE id = $1::text::uuid`, item.intentID, now); err != nil {
			return err
		}
	}
	return nil
}
