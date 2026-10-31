package accountability

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) MaintainRequests(ctx context.Context, now time.Time) (MaintenanceResult, error) {
	if r == nil || r.pool == nil {
		return MaintenanceResult{}, ErrUnavailable
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return MaintenanceResult{}, fmt.Errorf("begin accountability maintenance: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result := MaintenanceResult{}
	expiredRows, err := tx.Query(ctx, `
		SELECT id::text, intent_id::text
		  FROM accountability_requests
		 WHERE status = 'ASSIGNED' AND expires_at <= $1
		 ORDER BY expires_at, id
		 FOR UPDATE
	`, now)
	if err != nil {
		return result, fmt.Errorf("lock expired accountability requests: %w", err)
	}
	type expiredRequest struct{ requestID, intentID string }
	var expired []expiredRequest
	for expiredRows.Next() {
		var item expiredRequest
		if err := expiredRows.Scan(&item.requestID, &item.intentID); err != nil {
			expiredRows.Close()
			return result, err
		}
		expired = append(expired, item)
	}
	if err := expiredRows.Err(); err != nil {
		expiredRows.Close()
		return result, err
	}
	expiredRows.Close()
	for _, item := range expired {
		if _, err := tx.Exec(ctx, `
			UPDATE accountability_assignments SET status = 'SUPERSEDED', responded_at = $2
			 WHERE request_id = $1::text::uuid AND status = 'ACTIVE'
		`, item.requestID, now); err != nil {
			return result, err
		}
		if _, err := tx.Exec(ctx, `UPDATE accountability_requests SET status = 'EXPIRED', resolved_at = $2 WHERE id = $1::text::uuid`, item.requestID, now); err != nil {
			return result, err
		}
		if _, err := tx.Exec(ctx, `UPDATE transaction_intents SET status = 'EXPIRED', updated_at = $2 WHERE id = $1::text::uuid`, item.intentID, now); err != nil {
			return result, err
		}
		result.Expired++
	}

	rows, err := tx.Query(ctx, `
		SELECT assignment.id::text, assignment.request_id::text,
		       assignment.reviewer_position, request.expires_at
		  FROM accountability_assignments AS assignment
		  JOIN accountability_requests AS request ON request.id = assignment.request_id
		 WHERE assignment.status = 'ACTIVE' AND assignment.expires_at <= $1
		   AND request.status = 'ASSIGNED' AND request.expires_at > $1
		 ORDER BY assignment.expires_at, assignment.id
		 FOR UPDATE OF assignment, request
	`, now)
	if err != nil {
		return result, fmt.Errorf("lock timed out accountability assignments: %w", err)
	}
	type timedOutAssignment struct {
		assignmentID, requestID string
		position                int
		requestExpires          time.Time
	}
	var timedOut []timedOutAssignment
	for rows.Next() {
		var item timedOutAssignment
		if err := rows.Scan(&item.assignmentID, &item.requestID, &item.position, &item.requestExpires); err != nil {
			rows.Close()
			return result, err
		}
		timedOut = append(timedOut, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	for _, item := range timedOut {
		if _, err := tx.Exec(ctx, `UPDATE accountability_assignments SET status = 'TIMED_OUT', responded_at = $2 WHERE id = $1::text::uuid`, item.assignmentID, now); err != nil {
			return result, err
		}
		var nextPosition int
		var nextPartner string
		err := tx.QueryRow(ctx, `
			SELECT position, partner_id::text FROM accountability_request_reviewers
			 WHERE request_id = $1::text::uuid AND position > $2
			 ORDER BY position LIMIT 1
		`, item.requestID, item.position).Scan(&nextPosition, &nextPartner)
		if err == nil {
			expiresAt := now.Add(assignmentLifetime)
			if expiresAt.After(item.requestExpires) {
				expiresAt = item.requestExpires
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO accountability_assignments (
					id, request_id, reviewer_position, partner_id, status, assigned_at, expires_at
				) VALUES (gen_random_uuid(), $1::text::uuid, $2, $3::text::uuid, 'ACTIVE', $4, $5)
			`, item.requestID, nextPosition, nextPartner, now, expiresAt)
		}
		if err != nil && err != pgx.ErrNoRows {
			return result, fmt.Errorf("advance timed out accountability assignment: %w", err)
		}
		result.TimedOut++
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit accountability maintenance: %w", err)
	}
	return result, nil
}
