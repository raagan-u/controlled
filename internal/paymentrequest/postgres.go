package paymentrequest

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, candidate Request) (Request, error) {
	if r == nil || r.pool == nil {
		return Request{}, ErrUnavailable
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO payment_requests (id, requester_id, recipient_id, idempotency_key, amount_minor, currency, note, status, created_at)
		VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid, $4, $5, $6, $7, 'PENDING', $8)
	`, candidate.ID, candidate.RequesterID, candidate.RecipientID, candidate.IdempotencyKey, candidate.AmountMinor, candidate.Currency, candidate.Note, candidate.CreatedAt)
	if err == nil {
		return candidate, nil
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		existing, loadErr := r.byKey(ctx, candidate.RequesterID, candidate.IdempotencyKey)
		if loadErr != nil {
			return Request{}, loadErr
		}
		if existing.RecipientID != candidate.RecipientID || existing.AmountMinor != candidate.AmountMinor || existing.Currency != candidate.Currency || existing.Note != candidate.Note {
			return Request{}, ErrIdempotencyConflict
		}
		existing.Duplicate = true
		return existing, nil
	}
	if errors.As(err, &databaseError) && databaseError.Code == "23503" {
		return Request{}, ErrInvalidRequest
	}
	return Request{}, fmt.Errorf("create payment request: %w", err)
}

func (r *PostgresRepository) byKey(ctx context.Context, requesterID interface{}, key string) (Request, error) {
	var request Request
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, requester_id::text, recipient_id::text, amount_minor, currency, note, status, created_at, idempotency_key
		FROM payment_requests WHERE requester_id = $1::text::uuid AND idempotency_key = $2
	`, requesterID, key).Scan(&request.ID, &request.RequesterID, &request.RecipientID, &request.AmountMinor, &request.Currency, &request.Note, &request.Status, &request.CreatedAt, &request.IdempotencyKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrInvalidRequest
	}
	return request, err
}
