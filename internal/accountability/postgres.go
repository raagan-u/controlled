package accountability

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/controlled/internal/identity"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) UserByEmail(ctx context.Context, email string) (identity.User, error) {
	if r == nil || r.pool == nil {
		return identity.User{}, ErrUnavailable
	}
	row := r.pool.QueryRow(ctx, `
		SELECT id::text, email, display_name, created_at
		  FROM users
		 WHERE email = $1
	`, email)
	var id, storedEmail, displayName string
	var createdAt pgtype.Timestamptz
	if err := row.Scan(&id, &storedEmail, &displayName, &createdAt); errors.Is(err, pgx.ErrNoRows) {
		return identity.User{}, identity.ErrInvalidCredentials
	} else if err != nil {
		return identity.User{}, fmt.Errorf("read accountability partner: %w", err)
	}
	return identity.NewUser(identity.UserID(id), storedEmail, displayName, createdAt.Time)
}

func (r *PostgresRepository) CreateInvitation(
	ctx context.Context,
	idempotencyKey string,
	invitation Invitation,
) (Invitation, bool, error) {
	if r == nil || r.pool == nil {
		return Invitation{}, false, ErrUnavailable
	}
	id, ownerID, partnerID, err := invitationUUIDs(invitation)
	if err != nil {
		return Invitation{}, false, err
	}
	result, err := r.pool.Exec(ctx, `
		INSERT INTO accountability_invitations (
			id, owner_id, partner_id, idempotency_key, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (owner_id, idempotency_key) DO NOTHING
	`, id, ownerID, partnerID, idempotencyKey, invitation.Status(), invitation.CreatedAt())
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.ConstraintName == "accountability_active_relationship_unique" {
			return Invitation{}, false, ErrInvitationExists
		}
		return Invitation{}, false, fmt.Errorf("create accountability invitation: %w", err)
	}
	if result.RowsAffected() == 1 {
		return invitation, false, nil
	}
	existing, err := scanInvitation(r.pool.QueryRow(ctx, `
		SELECT id::text, owner_id::text, partner_id::text, status, created_at, resolved_at
		  FROM accountability_invitations
		 WHERE owner_id = $1 AND idempotency_key = $2
	`, ownerID, idempotencyKey))
	if err != nil {
		return Invitation{}, false, err
	}
	if existing.PartnerID() != invitation.PartnerID() {
		return Invitation{}, false, ErrIdempotencyConflict
	}
	return existing, true, nil
}

func (r *PostgresRepository) InvitationByID(ctx context.Context, id InvitationID) (Invitation, error) {
	if r == nil || r.pool == nil {
		return Invitation{}, ErrUnavailable
	}
	parsedID, err := accountabilityUUID(string(id))
	if err != nil {
		return Invitation{}, err
	}
	return scanInvitation(r.pool.QueryRow(ctx, `
		SELECT id::text, owner_id::text, partner_id::text, status, created_at, resolved_at
		  FROM accountability_invitations
		 WHERE id = $1
	`, parsedID))
}

func (r *PostgresRepository) SaveInvitation(ctx context.Context, invitation Invitation) error {
	if r == nil || r.pool == nil {
		return ErrUnavailable
	}
	id, _, partnerID, err := invitationUUIDs(invitation)
	if err != nil {
		return err
	}
	result, err := r.pool.Exec(ctx, `
		UPDATE accountability_invitations
		   SET status = $3, resolved_at = $4
		 WHERE id = $1
		   AND partner_id = $2
		   AND status = 'PENDING'
	`, id, partnerID, invitation.Status(), invitation.ResolvedAt())
	if err != nil {
		return fmt.Errorf("resolve accountability invitation: %w", err)
	}
	if result.RowsAffected() == 1 {
		return nil
	}
	existing, err := r.InvitationByID(ctx, invitation.ID())
	if err != nil {
		return err
	}
	if existing.Status() == invitation.Status() {
		return nil
	}
	return ErrInvitationFinal
}

func (r *PostgresRepository) AcceptedPartners(
	ctx context.Context,
	ownerID identity.UserID,
	partners []identity.UserID,
) (map[identity.UserID]bool, error) {
	if r == nil || r.pool == nil {
		return nil, ErrUnavailable
	}
	parsedOwner, err := accountabilityUUID(string(ownerID))
	if err != nil {
		return nil, err
	}
	partnerValues := make([]string, len(partners))
	for index, partnerID := range partners {
		if _, err := accountabilityUUID(string(partnerID)); err != nil {
			return nil, err
		}
		partnerValues[index] = string(partnerID)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT partner_id::text
		  FROM accountability_invitations
		 WHERE owner_id = $1
		   AND partner_id = ANY($2::uuid[])
		   AND status = 'ACCEPTED'
	`, parsedOwner, partnerValues)
	if err != nil {
		return nil, fmt.Errorf("read accepted accountability partners: %w", err)
	}
	defer rows.Close()
	result := make(map[identity.UserID]bool, len(partners))
	for rows.Next() {
		var partnerID string
		if err := rows.Scan(&partnerID); err != nil {
			return nil, fmt.Errorf("scan accepted accountability partner: %w", err)
		}
		result[identity.UserID(partnerID)] = true
	}
	return result, rows.Err()
}

func (r *PostgresRepository) CreateCircle(
	ctx context.Context,
	idempotencyKey string,
	circle Circle,
) (Circle, bool, error) {
	if r == nil || r.pool == nil {
		return Circle{}, false, ErrUnavailable
	}
	id, err := accountabilityUUID(string(circle.ID()))
	if err != nil {
		return Circle{}, false, err
	}
	ownerID, err := accountabilityUUID(string(circle.OwnerID()))
	if err != nil {
		return Circle{}, false, err
	}
	transaction, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Circle{}, false, fmt.Errorf("begin accountability circle: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	result, err := transaction.Exec(ctx, `
		INSERT INTO accountability_circles (id, owner_id, name, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (owner_id, idempotency_key) DO NOTHING
	`, id, ownerID, circle.Name(), idempotencyKey, circle.CreatedAt())
	if err != nil {
		return Circle{}, false, fmt.Errorf("create accountability circle: %w", err)
	}
	if result.RowsAffected() == 0 {
		existing, err := circleByKey(ctx, transaction, ownerID, idempotencyKey)
		if err != nil {
			return Circle{}, false, err
		}
		if existing.Name() != circle.Name() || !slices.Equal(existing.Partners(), circle.Partners()) {
			return Circle{}, false, ErrIdempotencyConflict
		}
		return existing, true, nil
	}
	for position, partner := range circle.Partners() {
		partnerID, err := accountabilityUUID(string(partner))
		if err != nil {
			return Circle{}, false, err
		}
		if _, err := transaction.Exec(ctx, `
			INSERT INTO accountability_circle_members (circle_id, position, partner_id)
			VALUES ($1, $2, $3)
		`, id, position, partnerID); err != nil {
			return Circle{}, false, fmt.Errorf("add accountability circle member: %w", err)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return Circle{}, false, fmt.Errorf("commit accountability circle: %w", err)
	}
	return circle, false, nil
}

func circleByKey(ctx context.Context, transaction pgx.Tx, ownerID pgtype.UUID, key string) (Circle, error) {
	row := transaction.QueryRow(ctx, `
		SELECT id::text, owner_id::text, name, created_at
		  FROM accountability_circles
		 WHERE owner_id = $1 AND idempotency_key = $2
	`, ownerID, key)
	var id, owner, name string
	var createdAt pgtype.Timestamptz
	if err := row.Scan(&id, &owner, &name, &createdAt); err != nil {
		return Circle{}, fmt.Errorf("read accountability circle: %w", err)
	}
	rows, err := transaction.Query(ctx, `
		SELECT partner_id::text
		  FROM accountability_circle_members
		 WHERE circle_id = $1
		 ORDER BY position
	`, id)
	if err != nil {
		return Circle{}, fmt.Errorf("read accountability circle members: %w", err)
	}
	defer rows.Close()
	var partners []identity.UserID
	for rows.Next() {
		var partner string
		if err := rows.Scan(&partner); err != nil {
			return Circle{}, err
		}
		partners = append(partners, identity.UserID(partner))
	}
	if err := rows.Err(); err != nil {
		return Circle{}, err
	}
	return NewCircle(CircleID(id), identity.UserID(owner), name, partners, createdAt.Time)
}

func scanInvitation(row pgx.Row) (Invitation, error) {
	var id, ownerID, partnerID string
	var status InvitationStatus
	var createdAt pgtype.Timestamptz
	var resolvedAt pgtype.Timestamptz
	if err := row.Scan(&id, &ownerID, &partnerID, &status, &createdAt, &resolvedAt); errors.Is(err, pgx.ErrNoRows) {
		return Invitation{}, ErrInvitationNotFound
	} else if err != nil {
		return Invitation{}, fmt.Errorf("read accountability invitation: %w", err)
	}
	return RestoreInvitation(
		InvitationID(id), identity.UserID(ownerID), identity.UserID(partnerID),
		status, createdAt.Time, resolvedAt.Time,
	)
}

func invitationUUIDs(invitation Invitation) (pgtype.UUID, pgtype.UUID, pgtype.UUID, error) {
	id, err := accountabilityUUID(string(invitation.ID()))
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	ownerID, err := accountabilityUUID(string(invitation.OwnerID()))
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	partnerID, err := accountabilityUUID(string(invitation.PartnerID()))
	return id, ownerID, partnerID, err
}

func accountabilityUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil || !parsed.Valid {
		return pgtype.UUID{}, fmt.Errorf("%w: %q", ErrPersistenceID, value)
	}
	return parsed, nil
}
