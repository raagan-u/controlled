package accountability

import (
	"context"
	"fmt"
	"time"

	"github.com/raagan-u/controlled/internal/walletops"
)

func (r *PostgresRepository) MarkIntentExecuted(
	ctx context.Context,
	requestID string,
	operationID walletops.OperationID,
	now time.Time,
) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE transaction_intents AS intent
		   SET status = 'COMPLETED', executed_operation_id = $2::text::uuid, updated_at = $3
		  FROM accountability_requests AS request
		 WHERE request.id = $1::text::uuid AND request.intent_id = intent.id
		   AND request.status = 'APPROVED' AND intent.status IN ('APPROVED', 'FAILED')
	`, requestID, string(operationID), now)
	if err != nil {
		return fmt.Errorf("complete approved transaction intent: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrRequestFinal
	}
	return nil
}

func (r *PostgresRepository) MarkIntentFailed(ctx context.Context, requestID string, now time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin approved transaction failure: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(ctx, `
		UPDATE transaction_intents AS intent
		   SET status = 'FAILED', updated_at = $2
		  FROM accountability_requests AS request
		 WHERE request.id = $1::text::uuid AND request.intent_id = intent.id
		   AND request.status = 'APPROVED' AND intent.status = 'APPROVED'
	`, requestID, now)
	if err != nil {
		return fmt.Errorf("fail approved transaction intent: %w", err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE accountability_requests
		   SET status = 'FAILED', classification = NULL, resolved_at = COALESCE(resolved_at, $2)
		 WHERE id = $1::text::uuid AND status = 'APPROVED'
	`, requestID, now)
	if err != nil {
		return fmt.Errorf("fail approved accountability request: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit approved transaction failure: %w", err)
	}
	return nil
}
