package accountability

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/walletops"
)

func (r *PostgresRepository) CreateDebitRequest(
	ctx context.Context,
	debit walletops.PendingDebit,
	requestID string,
	assignmentID string,
) (walletops.ReviewRequest, error) {
	if r == nil || r.pool == nil {
		return walletops.ReviewRequest{}, ErrUnavailable
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return walletops.ReviewRequest{}, fmt.Errorf("begin accountability request: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	lock := "accountability-request:" + string(debit.OwnerID) + ":" + string(debit.Kind) + ":" + debit.IdempotencyKey
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", lock); err != nil {
		return walletops.ReviewRequest{}, fmt.Errorf("lock accountability request: %w", err)
	}
	if existing, err := loadReviewRequestByKey(ctx, tx, debit); err == nil {
		existing.Duplicate = true
		return existing, nil
	} else if !errors.Is(err, ErrRequestNotFound) {
		return walletops.ReviewRequest{}, err
	}

	expiresAt := debit.CreatedAt.UTC().Add(requestLifetime)
	_, err = tx.Exec(ctx, `
		INSERT INTO transaction_intents (
			id, owner_id, operation_type, idempotency_key, destination,
			amount_minor, currency, status, created_at, updated_at
		) VALUES ($1::text::uuid, $2::text::uuid, $3, $4, $5, $6, $7, 'APPROVAL_REQUIRED', $8, $8)
	`, string(debit.IntentID), string(debit.OwnerID), debit.Kind, debit.IdempotencyKey,
		debit.Destination, debit.Amount.AmountMinor(), debit.Amount.Currency(), debit.CreatedAt.UTC())
	if err != nil {
		return walletops.ReviewRequest{}, fmt.Errorf("create transaction intent: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO accountability_requests (
			id, intent_id, owner_id, owner_reason, status, created_at, expires_at
		) VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid, $4, 'ASSIGNED', $5, $6)
	`, requestID, string(debit.IntentID), string(debit.OwnerID), debit.Reason, debit.CreatedAt.UTC(), expiresAt)
	if err != nil {
		return walletops.ReviewRequest{}, fmt.Errorf("create accountability request: %w", err)
	}
	for _, block := range debit.Blocks {
		_, err = tx.Exec(ctx, `
			INSERT INTO accountability_request_pacts (
				request_id, pact_id, limit_minor, used_minor, proposed_minor, overage_minor
			) VALUES ($1::text::uuid, $2::text::uuid, $3, $4, $5, $6)
		`, requestID, block.PactID, block.LimitMinor, block.UsedMinor, block.ProposedMinor, block.OverageMinor)
		if err != nil {
			return walletops.ReviewRequest{}, fmt.Errorf("record blocking pact: %w", err)
		}
	}
	reviewers, err := requestReviewers(ctx, tx, requestID)
	if err != nil {
		return walletops.ReviewRequest{}, err
	}
	if len(reviewers) == 0 {
		return walletops.ReviewRequest{}, ErrMissingPartners
	}
	for position, partnerID := range reviewers {
		_, err = tx.Exec(ctx, `
			INSERT INTO accountability_request_reviewers (request_id, position, partner_id)
			VALUES ($1::text::uuid, $2, $3::text::uuid)
		`, requestID, position, partnerID)
		if err != nil {
			return walletops.ReviewRequest{}, fmt.Errorf("record request reviewer: %w", err)
		}
	}
	assignmentExpires := debit.CreatedAt.UTC().Add(assignmentLifetime)
	if assignmentExpires.After(expiresAt) {
		assignmentExpires = expiresAt
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO accountability_assignments (
			id, request_id, reviewer_position, partner_id, status, assigned_at, expires_at
		) VALUES ($1::text::uuid, $2::text::uuid, 0, $3::text::uuid, 'ACTIVE', $4, $5)
	`, assignmentID, requestID, reviewers[0], debit.CreatedAt.UTC(), assignmentExpires)
	if err != nil {
		return walletops.ReviewRequest{}, fmt.Errorf("assign accountability request: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO controlled_audit_events (actor_id, aggregate_type, aggregate_id, event_type, facts, occurred_at)
		VALUES ($1::text::uuid, 'ACCOUNTABILITY_REQUEST', $2::text::uuid, 'REQUESTED',
		        jsonb_build_object('operationType', $3::text, 'amountMinor', $4::bigint, 'currency', $5::text), $6)
	`, string(debit.OwnerID), requestID, debit.Kind, debit.Amount.AmountMinor(), debit.Amount.Currency(), debit.CreatedAt.UTC())
	if err != nil {
		return walletops.ReviewRequest{}, fmt.Errorf("audit accountability request: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return walletops.ReviewRequest{}, fmt.Errorf("commit accountability request: %w", err)
	}
	return walletops.ReviewRequest{
		ID: requestID, IntentID: debit.IntentID, Status: "ASSIGNED",
		AssignedPartnerID: identity.UserID(reviewers[0]), CreatedAt: debit.CreatedAt.UTC(), ExpiresAt: expiresAt,
	}, nil
}

func requestReviewers(ctx context.Context, tx pgx.Tx, requestID string) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT member.partner_id::text
		  FROM accountability_request_pacts AS blocked
		  JOIN pacts AS pact ON pact.id = blocked.pact_id
		  JOIN accountability_circle_members AS member ON member.circle_id = pact.accountability_circle_id
		 WHERE blocked.request_id = $1::text::uuid
		 GROUP BY member.partner_id
		 ORDER BY min(pact.id::text), min(member.position), member.partner_id
	`, requestID)
	if err != nil {
		return nil, fmt.Errorf("load request reviewers: %w", err)
	}
	defer rows.Close()
	var reviewers []string
	for rows.Next() {
		var partnerID string
		if err := rows.Scan(&partnerID); err != nil {
			return nil, fmt.Errorf("scan request reviewer: %w", err)
		}
		reviewers = append(reviewers, partnerID)
	}
	return reviewers, rows.Err()
}

func loadReviewRequestByKey(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, debit walletops.PendingDebit) (walletops.ReviewRequest, error) {
	var result walletops.ReviewRequest
	var intentID, partnerID, destination, currency string
	var amount int64
	err := query.QueryRow(ctx, `
		SELECT request.id::text, intent.id::text, request.status,
		       COALESCE(assignment.partner_id::text, ''), request.created_at, request.expires_at,
		       intent.destination, intent.amount_minor, intent.currency
		  FROM transaction_intents AS intent
		  JOIN accountability_requests AS request ON request.intent_id = intent.id
		  LEFT JOIN accountability_assignments AS assignment
		    ON assignment.request_id = request.id AND assignment.status = 'ACTIVE'
		 WHERE intent.owner_id = $1::text::uuid AND intent.operation_type = $2 AND intent.idempotency_key = $3
	`, string(debit.OwnerID), debit.Kind, debit.IdempotencyKey).Scan(
		&result.ID, &intentID, &result.Status, &partnerID, &result.CreatedAt, &result.ExpiresAt,
		&destination, &amount, &currency,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return walletops.ReviewRequest{}, ErrRequestNotFound
	}
	if err != nil {
		return walletops.ReviewRequest{}, fmt.Errorf("read accountability request by key: %w", err)
	}
	if destination != debit.Destination || amount != debit.Amount.AmountMinor() || currency != debit.Amount.Currency() {
		return walletops.ReviewRequest{}, ErrIdempotencyConflict
	}
	result.IntentID = walletops.OperationID(intentID)
	result.AssignedPartnerID = identity.UserID(partnerID)
	return result, nil
}
