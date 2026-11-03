package accountability

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/raagan-u/controlled/internal/identity"
)

func (r *PostgresRepository) DecideRequest(
	ctx context.Context,
	actorID identity.UserID,
	requestID string,
	approve bool,
	classification string,
	reason string,
	nextAssignmentID string,
	now time.Time,
) error {
	if r == nil || r.pool == nil {
		return ErrUnavailable
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin accountability decision: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var status, intentID, assignmentID, partnerID string
	var position int
	var requestExpires time.Time
	err = tx.QueryRow(ctx, `
		SELECT status, intent_id::text, expires_at
		  FROM accountability_requests
		 WHERE id = $1::text::uuid
		 FOR UPDATE
	`, requestID).Scan(&status, &intentID, &requestExpires)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRequestNotFound
	}
	if err != nil {
		return fmt.Errorf("lock accountability request: %w", err)
	}
	if status != "ASSIGNED" {
		return ErrRequestFinal
	}
	err = tx.QueryRow(ctx, `
		SELECT id::text, partner_id::text, reviewer_position
		  FROM accountability_assignments
		 WHERE request_id = $1::text::uuid AND status = 'ACTIVE'
		 FOR UPDATE
	`, requestID).Scan(&assignmentID, &partnerID, &position)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRequestFinal
	}
	if err != nil {
		return fmt.Errorf("lock accountability assignment: %w", err)
	}
	if partnerID != string(actorID) {
		return ErrWrongReviewer
	}
	decision := "DENY"
	if approve && classification == "EMERGENCY" {
		decision = "APPROVE_EMERGENCY"
	} else if approve && classification == "CONSCIOUS_BREAK" {
		decision = "APPROVE_CONSCIOUS_BREAK"
	}
	_, err = tx.Exec(ctx, `
		UPDATE accountability_assignments
		   SET status = 'RESPONDED', responded_at = $2, decision = $3,
		       decision_reason = NULLIF($4, '')
		 WHERE id = $1::text::uuid AND status = 'ACTIVE'
	`, assignmentID, now, decision, reason)
	if err != nil {
		return fmt.Errorf("record accountability decision: %w", err)
	}
	if approve {
		_, err = tx.Exec(ctx, `
			UPDATE accountability_requests
			   SET status = 'APPROVED', classification = $2, resolved_at = $3
			 WHERE id = $1::text::uuid
		`, requestID, classification, now)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE transaction_intents SET status = 'APPROVED', updated_at = $2 WHERE id = $1::text::uuid`, intentID, now)
		}
		if err != nil {
			return fmt.Errorf("approve accountability request: %w", err)
		}
	} else {
		var nextPosition int
		var nextPartner string
		err = tx.QueryRow(ctx, `
			SELECT position, partner_id::text
			  FROM accountability_request_reviewers
			 WHERE request_id = $1::text::uuid AND position > $2
			 ORDER BY position
			 LIMIT 1
		`, requestID, position).Scan(&nextPosition, &nextPartner)
		if errors.Is(err, pgx.ErrNoRows) {
			var reviewerCount, denialCount int
			if err := tx.QueryRow(ctx, `
				SELECT (SELECT count(*) FROM accountability_request_reviewers WHERE request_id = $1::text::uuid),
				       (SELECT count(*) FROM accountability_assignments WHERE request_id = $1::text::uuid AND decision = 'DENY')
			`, requestID).Scan(&reviewerCount, &denialCount); err != nil {
				return fmt.Errorf("count accountability denials: %w", err)
			}
			if reviewerCount == denialCount {
				_, err = tx.Exec(ctx, `UPDATE accountability_requests SET status = 'DENIED', resolved_at = $2 WHERE id = $1::text::uuid`, requestID, now)
				if err == nil {
					_, err = tx.Exec(ctx, `UPDATE transaction_intents SET status = 'CANCELLED', updated_at = $2 WHERE id = $1::text::uuid`, intentID, now)
				}
				if err != nil {
					return fmt.Errorf("deny accountability request: %w", err)
				}
			}
		} else if err != nil {
			return fmt.Errorf("find next accountability reviewer: %w", err)
		} else {
			expiresAt := now.Add(assignmentLifetime)
			if expiresAt.After(requestExpires) {
				expiresAt = requestExpires
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO accountability_assignments (
					id, request_id, reviewer_position, partner_id, status, assigned_at, expires_at
				) VALUES ($1::text::uuid, $2::text::uuid, $3, $4::text::uuid, 'ACTIVE', $5, $6)
			`, nextAssignmentID, requestID, nextPosition, nextPartner, now, expiresAt)
			if err != nil {
				return fmt.Errorf("assign next accountability reviewer: %w", err)
			}
		}
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO controlled_audit_events (actor_id, aggregate_type, aggregate_id, event_type, facts, occurred_at)
		VALUES ($1::text::uuid, 'ACCOUNTABILITY_REQUEST', $2::text::uuid, $3,
		        jsonb_strip_nulls(jsonb_build_object('decision', $4::text, 'reason', NULLIF($5, ''))), $6)
	`, string(actorID), requestID, "PARTNER_DECIDED", decision, reason, now)
	if err != nil {
		return fmt.Errorf("audit accountability decision: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit accountability decision: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CancelRequest(
	ctx context.Context,
	ownerID identity.UserID,
	requestID string,
	now time.Time,
) error {
	if r == nil || r.pool == nil {
		return ErrUnavailable
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin accountability cancellation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var intentID string
	err = tx.QueryRow(ctx, `
		SELECT intent_id::text FROM accountability_requests
		 WHERE id = $1::text::uuid AND owner_id = $2::text::uuid AND status = 'ASSIGNED'
		 FOR UPDATE
	`, requestID, string(ownerID)).Scan(&intentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRequestFinal
	}
	if err != nil {
		return fmt.Errorf("lock accountability cancellation: %w", err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE accountability_assignments SET status = 'SUPERSEDED', responded_at = $2
		 WHERE request_id = $1::text::uuid AND status = 'ACTIVE'
	`, requestID, now)
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE accountability_requests SET status = 'CANCELLED', resolved_at = $2 WHERE id = $1::text::uuid`, requestID, now)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE transaction_intents SET status = 'CANCELLED', updated_at = $2 WHERE id = $1::text::uuid`, intentID, now)
	}
	if err != nil {
		return fmt.Errorf("cancel accountability request: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO controlled_audit_events (actor_id, aggregate_type, aggregate_id, event_type, occurred_at)
		VALUES ($1::text::uuid, 'ACCOUNTABILITY_REQUEST', $2::text::uuid, 'CANCELLED', $3)
	`, string(ownerID), requestID, now)
	if err != nil {
		return fmt.Errorf("audit accountability cancellation: %w", err)
	}
	return tx.Commit(ctx)
}
