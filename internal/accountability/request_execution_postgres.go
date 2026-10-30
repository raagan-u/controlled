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
		   AND request.status = 'APPROVED' AND intent.status = 'APPROVED'
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
	_, err := r.pool.Exec(ctx, `
		UPDATE transaction_intents AS intent
		   SET status = 'FAILED', updated_at = $2
		  FROM accountability_requests AS request
		 WHERE request.id = $1::text::uuid AND request.intent_id = intent.id
		   AND request.status = 'APPROVED' AND intent.status = 'APPROVED'
	`, requestID, now)
	if err != nil {
		return fmt.Errorf("fail approved transaction intent: %w", err)
	}
	return nil
}
