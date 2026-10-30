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
		SELECT pact_id::text, limit_minor, used_minor, proposed_minor, overage_minor
		  FROM accountability_request_pacts
		 WHERE request_id = $1::text::uuid
		 ORDER BY pact_id
	`, requestID)
	if err != nil {
		return Request{}, fmt.Errorf("read accountability request pacts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var pact RequestPact
		if err := rows.Scan(&pact.PactID, &pact.LimitMinor, &pact.UsedMinor, &pact.ProposedMinor, &pact.OverageMinor); err != nil {
			return Request{}, err
		}
		item.Pacts = append(item.Pacts, pact)
	}
	return item, rows.Err()
}
