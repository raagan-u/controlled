package accountability

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/walletops"
)

func (r *PostgresRepository) RequestsForUser(ctx context.Context, userID identity.UserID) ([]Request, error) {
	if r == nil || r.pool == nil {
		return nil, ErrUnavailable
	}
	rows, err := r.pool.Query(ctx, `
		SELECT request.id::text
		  FROM accountability_requests AS request
		 WHERE request.owner_id = $1::text::uuid
		    OR EXISTS (
		        SELECT 1 FROM accountability_request_reviewers AS reviewer
		         WHERE reviewer.request_id = request.id AND reviewer.partner_id = $1::text::uuid
		    )
		 ORDER BY request.created_at DESC, request.id
	`, string(userID))
	if err != nil {
		return nil, fmt.Errorf("list accountability requests: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	requests := make([]Request, 0, len(ids))
	for _, id := range ids {
		item, err := r.RequestForUser(ctx, userID, id)
		if err != nil {
			return nil, err
		}
		requests = append(requests, item)
	}
	return requests, nil
}

func (r *PostgresRepository) RequestForUser(ctx context.Context, userID identity.UserID, requestID string) (Request, error) {
	if r == nil || r.pool == nil {
		return Request{}, ErrUnavailable
	}
	var item Request
	var operationType string
	var executedOperationID string
	var assignedPartner string
	var resolvedAt pgtype.Timestamptz
	err := r.pool.QueryRow(ctx, `
		SELECT request.id::text, intent.id::text, request.owner_id::text,
		       intent.operation_type, intent.idempotency_key, intent.destination, intent.amount_minor, intent.currency,
		       request.owner_reason, request.status, COALESCE(request.classification, ''),
		       COALESCE(assignment.partner_id::text, ''),
		       request.created_at, request.expires_at, request.resolved_at,
		       COALESCE(intent.executed_operation_id::text, '')
		  FROM accountability_requests AS request
		  JOIN transaction_intents AS intent ON intent.id = request.intent_id
		  LEFT JOIN accountability_assignments AS assignment
		    ON assignment.request_id = request.id AND assignment.status = 'ACTIVE'
		 WHERE request.id = $1::text::uuid
		   AND (
		       request.owner_id = $2::text::uuid
		       OR EXISTS (
		           SELECT 1 FROM accountability_request_reviewers AS reviewer
		            WHERE reviewer.request_id = request.id AND reviewer.partner_id = $2::text::uuid
		       )
		   )
	`, requestID, string(userID)).Scan(
		&item.ID, &item.IntentID, &item.OwnerID, &operationType, &item.IdempotencyKey, &item.Destination,
		&item.AmountMinor, &item.Currency, &item.Reason, &item.Status, &item.Classification,
		&assignedPartner, &item.CreatedAt, &item.ExpiresAt, &resolvedAt, &executedOperationID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrRequestNotFound
	}
	if err != nil {
		return Request{}, fmt.Errorf("read accountability request: %w", err)
	}
	item.OperationType = walletops.DebitKind(operationType)
	item.AssignedPartnerID = identity.UserID(assignedPartner)
	item.ExecutedOperationID = walletops.OperationID(executedOperationID)
	if resolvedAt.Valid {
		item.ResolvedAt = resolvedAt.Time
	}
	rows, err := r.pool.Query(ctx, `
		SELECT blocked.pact_id::text, pact.goal, pact.intent,
		       blocked.limit_minor, blocked.used_minor, blocked.proposed_minor, blocked.overage_minor
		  FROM accountability_request_pacts AS blocked
		  JOIN pacts AS pact ON pact.id = blocked.pact_id
		 WHERE blocked.request_id = $1::text::uuid
		 ORDER BY blocked.pact_id
	`, requestID)
	if err != nil {
		return Request{}, fmt.Errorf("read accountability request pacts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var pact RequestPact
		if err := rows.Scan(
			&pact.PactID, &pact.Goal, &pact.Intent,
			&pact.LimitMinor, &pact.UsedMinor, &pact.ProposedMinor, &pact.OverageMinor,
		); err != nil {
			return Request{}, err
		}
		item.Pacts = append(item.Pacts, pact)
	}
	if err := rows.Err(); err != nil {
		return Request{}, err
	}
	rows.Close()
	reviewerRows, err := r.pool.Query(ctx, `
		SELECT reviewer.partner_id::text, reviewer.position,
		       COALESCE(assignment.status, 'WAITING'), COALESCE(assignment.decision, ''),
		       assignment.assigned_at, assignment.expires_at, assignment.responded_at
		  FROM accountability_request_reviewers AS reviewer
		  LEFT JOIN accountability_assignments AS assignment
		    ON assignment.request_id = reviewer.request_id
		   AND assignment.reviewer_position = reviewer.position
		 WHERE reviewer.request_id = $1::text::uuid
		 ORDER BY reviewer.position
	`, requestID)
	if err != nil {
		return Request{}, fmt.Errorf("read accountability reviewers: %w", err)
	}
	defer reviewerRows.Close()
	for reviewerRows.Next() {
		var reviewer RequestReviewer
		var assignedAt, expiresAt, respondedAt pgtype.Timestamptz
		if err := reviewerRows.Scan(
			&reviewer.PartnerID, &reviewer.Position, &reviewer.Status, &reviewer.Decision,
			&assignedAt, &expiresAt, &respondedAt,
		); err != nil {
			return Request{}, fmt.Errorf("scan accountability reviewer: %w", err)
		}
		if assignedAt.Valid {
			reviewer.AssignedAt = assignedAt.Time
		}
		if expiresAt.Valid {
			reviewer.ExpiresAt = expiresAt.Time
		}
		if respondedAt.Valid {
			reviewer.RespondedAt = respondedAt.Time
		}
		item.Reviewers = append(item.Reviewers, reviewer)
	}
	return item, reviewerRows.Err()
}
