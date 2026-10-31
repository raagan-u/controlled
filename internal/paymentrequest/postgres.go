package paymentrequest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/controlled/internal/identity"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, candidate Request) (Request, error) {
	if r == nil || r.pool == nil {
		return Request{}, ErrUnavailable
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO payment_requests
		(id, requester_id, recipient_id, idempotency_key, amount_minor, currency, note, status, created_at, updated_at, expires_at)
		VALUES ($1::text::uuid,$2::text::uuid,$3::text::uuid,$4,$5,$6,$7,'PENDING',$8,$8,$9)`,
		candidate.ID, candidate.RequesterID, candidate.RecipientID, candidate.IdempotencyKey, candidate.AmountMinor, candidate.Currency, candidate.Note, candidate.CreatedAt, candidate.ExpiresAt)
	if err == nil {
		return r.ByID(ctx, candidate.RequesterID, candidate.ID)
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

func (r *PostgresRepository) ForUser(ctx context.Context, user identity.UserID) ([]Request, error) {
	rows, err := r.pool.Query(ctx, requestSelect+` WHERE request.requester_id=$1::text::uuid OR request.recipient_id=$1::text::uuid ORDER BY request.created_at DESC`, user)
	if err != nil {
		return nil, fmt.Errorf("list payment requests: %w", err)
	}
	defer rows.Close()
	result := make([]Request, 0)
	for rows.Next() {
		item, scanErr := scanRequest(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) ByID(ctx context.Context, user identity.UserID, id string) (Request, error) {
	return scanRequest(r.pool.QueryRow(ctx, requestSelect+` WHERE request.id=$1::text::uuid AND ($2::text::uuid IN (request.requester_id, request.recipient_id))`, id, user))
}

func (r *PostgresRepository) PreparePay(ctx context.Context, actor identity.UserID, id, paymentKey string, now time.Time) (Request, error) {
	result, err := r.pool.Exec(ctx, `UPDATE payment_requests SET status='PROCESSING', payment_key=$3, updated_at=$4
		WHERE id=$1::text::uuid AND recipient_id=$2::text::uuid AND status='PENDING' AND expires_at>$4`, id, actor, paymentKey, now)
	if err != nil {
		return Request{}, fmt.Errorf("prepare payment request: %w", err)
	}
	if result.RowsAffected() == 0 {
		return Request{}, r.transitionError(ctx, actor, id, true)
	}
	return r.ByID(ctx, actor, id)
}

func (r *PostgresRepository) CompletePay(ctx context.Context, id, transferID string, now time.Time) error {
	return r.updateOne(ctx, `UPDATE payment_requests SET status='PAID', transfer_id=$2::text::uuid, resolved_at=$3, updated_at=$3 WHERE id=$1::text::uuid AND status='PROCESSING'`, id, transferID, now)
}
func (r *PostgresRepository) AwaitApproval(ctx context.Context, id, accountabilityID string, now time.Time) error {
	return r.updateOne(ctx, `UPDATE payment_requests SET status='AWAITING_APPROVAL', accountability_request_id=$2::text::uuid, updated_at=$3 WHERE id=$1::text::uuid AND status='PROCESSING'`, id, accountabilityID, now)
}
func (r *PostgresRepository) ReleasePay(ctx context.Context, id string, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE payment_requests SET status='PENDING', payment_key=NULL, updated_at=$2 WHERE id=$1::text::uuid AND status='PROCESSING'`, id, now)
	return err
}

func (r *PostgresRepository) Resolve(ctx context.Context, actor identity.UserID, id, status string, now time.Time) error {
	role := "requester_id"
	if status == "DECLINED" {
		role = "recipient_id"
	}
	query := fmt.Sprintf(`UPDATE payment_requests SET status=$3, resolved_at=$4, updated_at=$4 WHERE id=$1::text::uuid AND %s=$2::text::uuid AND status='PENDING'`, role)
	result, err := r.pool.Exec(ctx, query, id, actor, status, now)
	if err != nil {
		return fmt.Errorf("resolve payment request: %w", err)
	}
	if result.RowsAffected() == 0 {
		return r.transitionError(ctx, actor, id, status == "DECLINED")
	}
	return nil
}

func (r *PostgresRepository) Maintain(ctx context.Context, now time.Time) error {
	statements := []string{
		`UPDATE payment_requests SET status='EXPIRED', resolved_at=$1, updated_at=$1 WHERE status='PENDING' AND expires_at<=$1`,
		`UPDATE payment_requests AS request SET status='PAID', transfer_id=transfer.id, resolved_at=transfer.created_at, updated_at=$1
		FROM internal_transfers AS transfer JOIN wallets AS sender ON sender.id=transfer.sender_wallet_id
		WHERE request.status IN ('PROCESSING','AWAITING_APPROVAL') AND request.payment_key=transfer.idempotency_key AND sender.owner_id=request.recipient_id`,
		`UPDATE payment_requests AS request SET status='AWAITING_APPROVAL', accountability_request_id=review.id, updated_at=$1
		FROM transaction_intents AS intent JOIN accountability_requests AS review ON review.intent_id=intent.id
		WHERE request.status='PROCESSING' AND request.payment_key=intent.idempotency_key AND intent.status IN ('APPROVAL_REQUIRED','APPROVED')`,
		`UPDATE payment_requests AS request SET status='PENDING', payment_key=NULL, accountability_request_id=NULL, updated_at=$1
		FROM transaction_intents AS intent WHERE request.status='AWAITING_APPROVAL' AND request.payment_key=intent.idempotency_key AND intent.status IN ('FAILED','CANCELLED','EXPIRED')`,
		`UPDATE payment_requests SET status='PENDING', payment_key=NULL, updated_at=$1
		WHERE status='PROCESSING' AND updated_at < $1::timestamptz - interval '5 minutes'
		AND NOT EXISTS (SELECT 1 FROM internal_transfers WHERE idempotency_key=payment_requests.payment_key)
		AND NOT EXISTS (SELECT 1 FROM transaction_intents WHERE idempotency_key=payment_requests.payment_key)`,
	}
	for _, statement := range statements {
		if _, err := r.pool.Exec(ctx, statement, now); err != nil {
			return fmt.Errorf("maintain payment requests: %w", err)
		}
	}
	return nil
}

func (r *PostgresRepository) byKey(ctx context.Context, requester identity.UserID, key string) (Request, error) {
	return scanRequest(r.pool.QueryRow(ctx, requestSelect+` WHERE request.requester_id=$1::text::uuid AND request.idempotency_key=$2`, requester, key))
}

func (r *PostgresRepository) transitionError(ctx context.Context, actor identity.UserID, id string, recipientRole bool) error {
	var requester, recipient identity.UserID
	var status string
	err := r.pool.QueryRow(ctx, `SELECT requester_id::text,recipient_id::text,status FROM payment_requests WHERE id=$1::text::uuid`, id).Scan(&requester, &recipient, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	wanted := requester
	if recipientRole {
		wanted = recipient
	}
	if actor != wanted {
		return ErrForbidden
	}
	return ErrInvalidState
}
func (r *PostgresRepository) updateOne(ctx context.Context, query string, args ...interface{}) error {
	result, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return nil
}

const requestSelect = `SELECT request.id::text,request.requester_id::text,request.recipient_id::text,request.amount_minor,request.currency,request.note,request.status,request.created_at,request.updated_at,request.expires_at,request.resolved_at,COALESCE(request.transfer_id::text,''),COALESCE(request.accountability_request_id::text,''),request.idempotency_key,COALESCE(request.payment_key,''), requester.id::text,requester.ctrld_id,requester.email,requester.display_name, recipient.id::text,recipient.ctrld_id,recipient.email,recipient.display_name FROM payment_requests AS request JOIN users AS requester ON requester.id=request.requester_id JOIN users AS recipient ON recipient.id=request.recipient_id`

func scanRequest(row pgx.Row) (Request, error) {
	var item Request
	var resolved *time.Time
	err := row.Scan(&item.ID, &item.RequesterID, &item.RecipientID, &item.AmountMinor, &item.Currency, &item.Note, &item.Status, &item.CreatedAt, &item.UpdatedAt, &item.ExpiresAt, &resolved, &item.TransferID, &item.AccountabilityRequestID, &item.IdempotencyKey, &item.PaymentKey, &item.Requester.ID, &item.Requester.CtrldID, &item.Requester.Email, &item.Requester.DisplayName, &item.Recipient.ID, &item.Recipient.CtrldID, &item.Recipient.Email, &item.Recipient.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	if err != nil {
		return Request{}, fmt.Errorf("read payment request: %w", err)
	}
	if resolved != nil {
		item.ResolvedAt = *resolved
	}
	return item, nil
}
