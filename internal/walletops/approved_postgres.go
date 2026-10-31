package walletops

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/ledger"
	"github.com/raagan-u/controlled/internal/money"
)

func (r *PostgresRepository) authorizePactDebit(
	ctx context.Context,
	tx pgx.Tx,
	ownerID identity.UserID,
	kind DebitKind,
	destination string,
	amount money.Money,
	at time.Time,
) ([]string, error) {
	pactIDs, blocks, err := evaluatePactPolicy(ctx, tx, string(ownerID), amount, at)
	if err != nil {
		return nil, err
	}
	approval := approvalFromContext(ctx)
	if approval == nil {
		if len(blocks) > 0 {
			return nil, &PolicyBlockedError{Blocks: blocks}
		}
		return pactIDs, nil
	}
	if approval.OwnerID != ownerID || approval.Kind != kind || approval.Destination != destination ||
		!approval.Amount.Equal(amount) {
		return nil, ErrPolicyDenied
	}
	for _, block := range blocks {
		if !slices.Contains(approval.PactIDs, block.PactID) {
			return nil, &PolicyBlockedError{Blocks: blocks}
		}
	}
	for _, approvedPactID := range approval.PactIDs {
		if !slices.Contains(pactIDs, approvedPactID) {
			return nil, ErrPolicyDenied
		}
	}
	if err := r.applyApprovedOverrides(ctx, tx, approval, at); err != nil {
		return nil, err
	}
	return pactIDs, nil
}

func (r *PostgresRepository) applyApprovedOverrides(
	ctx context.Context,
	tx pgx.Tx,
	approval *debitApproval,
	at time.Time,
) error {
	for _, pactID := range approval.PactIDs {
		switch approval.Classification {
		case "EMERGENCY":
			result, err := tx.Exec(ctx, `
				UPDATE pacts
				   SET status = 'ACTIVE', emergency_count = emergency_count + 1,
				       version = version + 1, updated_at = $2
				 WHERE id = $1::text::uuid AND owner_id = $3::text::uuid
				   AND status IN ('SCHEDULED', 'ACTIVE')
			`, pactID, at, string(approval.OwnerID))
			if err != nil {
				return fmt.Errorf("apply emergency override: %w", err)
			}
			if result.RowsAffected() != 1 {
				return ErrPolicyDenied
			}
		case "CONSCIOUS_BREAK":
			if err := r.breakPact(ctx, tx, approval, pactID, at); err != nil {
				return err
			}
		default:
			return ErrInvalidOperation
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO controlled_audit_events (
				actor_id, aggregate_type, aggregate_id, event_type, facts, occurred_at
			) VALUES ($1::text::uuid, 'PACT', $2::text::uuid, $3,
			          jsonb_build_object('requestId', $4::text), $5)
		`, string(approval.OwnerID), pactID, approval.Classification, approval.RequestID, at)
		if err != nil {
			return fmt.Errorf("audit pact override: %w", err)
		}
	}
	return nil
}

func (r *PostgresRepository) breakPact(
	ctx context.Context,
	tx pgx.Tx,
	approval *debitApproval,
	pactID string,
	at time.Time,
) error {
	var amountMinor int64
	var currency string
	err := tx.QueryRow(ctx, `
		SELECT amount_minor, currency
		  FROM stake_allocations
		 WHERE pact_id = $1::text::uuid AND owner_id = $2::text::uuid AND status = 'ACTIVE'
		 FOR UPDATE
	`, pactID, string(approval.OwnerID)).Scan(&amountMinor, &currency)
	if err != nil {
		return fmt.Errorf("lock stake for conscious break: %w", err)
	}
	amount, err := money.New(amountMinor, currency)
	if err != nil {
		return err
	}
	var vaultID, reserveID string
	err = tx.QueryRow(ctx, `
		SELECT vault.id::text, reserve.id::text
		  FROM ledger_accounts AS vault
		  JOIN wallets AS wallet ON wallet.owner_id = $1::text::uuid AND wallet.currency = $2
		  JOIN ledger_accounts AS reserve ON reserve.wallet_id = wallet.id AND reserve.kind = 'COMMITMENT_RESERVE'
		 WHERE vault.wallet_id IS NULL AND vault.kind = 'STAKE_VAULT' AND vault.currency = $2
	`, string(approval.OwnerID), currency).Scan(&vaultID, &reserveID)
	if err != nil {
		return fmt.Errorf("read conscious break accounts: %w", err)
	}
	debit, _ := ledger.NewEntry(ledger.AccountID(vaultID), ledger.SideDebit, amount)
	credit, _ := ledger.NewEntry(ledger.AccountID(reserveID), ledger.SideCredit, amount)
	journalID := approval.ResolutionJournalIDs[pactID]
	journal, err := ledger.NewJournal(
		journalID, "pact-conscious-break:"+pactID,
		ledger.Reference{Type: "PACT_CONSCIOUS_BREAK", ID: pactID},
		"Move broken pact stake to Commitment Reserve", at, []ledger.Entry{debit, credit},
	)
	if err != nil {
		return err
	}
	posted, err := r.ledger.PostInTx(ctx, tx, journal)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE stake_allocations
		   SET status = 'COMMITMENT_RESERVED', resolution_transaction_id = $2::text::uuid, resolved_at = $3
		 WHERE pact_id = $1::text::uuid AND status = 'ACTIVE'
	`, pactID, string(posted.JournalID), at)
	if err != nil {
		return fmt.Errorf("resolve conscious break stake: %w", err)
	}
	if err := cancelPendingRequestsForPact(ctx, tx, pactID, approval.RequestID, at); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `
		UPDATE pacts
		   SET status = 'RESOLVED', outcome = 'BROKEN', version = version + 1, updated_at = $2
		 WHERE id = $1::text::uuid AND owner_id = $3::text::uuid
		   AND status IN ('SCHEDULED', 'ACTIVE')
	`, pactID, at, string(approval.OwnerID))
	if err != nil {
		return fmt.Errorf("resolve consciously broken pact: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrPolicyDenied
	}
	return nil
}

func cancelPendingRequestsForPact(ctx context.Context, tx pgx.Tx, pactID, exceptRequestID string, at time.Time) error {
	rows, err := tx.Query(ctx, `
		SELECT request.id::text, request.intent_id::text
		  FROM accountability_requests AS request
		  JOIN accountability_request_pacts AS blocked ON blocked.request_id = request.id
		 WHERE blocked.pact_id = $1::text::uuid AND request.status = 'ASSIGNED'
		   AND request.id <> $2::text::uuid
		 FOR UPDATE OF request
	`, pactID, exceptRequestID)
	if err != nil {
		return fmt.Errorf("lock requests for broken pact: %w", err)
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
		if _, err := tx.Exec(ctx, `UPDATE accountability_assignments SET status = 'SUPERSEDED', responded_at = $2 WHERE request_id = $1::text::uuid AND status = 'ACTIVE'`, item.requestID, at); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE accountability_requests SET status = 'CANCELLED', resolved_at = $2 WHERE id = $1::text::uuid`, item.requestID, at); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE transaction_intents SET status = 'CANCELLED', updated_at = $2 WHERE id = $1::text::uuid`, item.intentID, at); err != nil {
			return err
		}
	}
	return nil
}
