package identity

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresSessionStore struct {
	pool *pgxpool.Pool
}

func NewPostgresSessionStore(pool *pgxpool.Pool) *PostgresSessionStore {
	return &PostgresSessionStore{pool: pool}
}

func (s *PostgresSessionStore) UserByEmail(ctx context.Context, email string) (User, error) {
	if s == nil || s.pool == nil {
		return User{}, ErrSessionStore
	}
	return scanUser(s.pool.QueryRow(ctx, `
		SELECT id::text, email, display_name, created_at
		  FROM users
		 WHERE email = $1
	`, email))
}

func (s *PostgresSessionStore) CreateSession(ctx context.Context, session Session) error {
	if s == nil || s.pool == nil {
		return ErrSessionStore
	}
	sessionID, err := identityUUID(string(session.ID()))
	if err != nil {
		return err
	}
	userID, err := identityUUID(string(session.UserID()))
	if err != nil {
		return err
	}
	tokenHash := session.TokenHash()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, sessionID, userID, tokenHash[:], session.CreatedAt(), session.ExpiresAt())
	if err != nil {
		return mapSessionError(err)
	}
	return nil
}

func (s *PostgresSessionStore) UserBySession(
	ctx context.Context,
	tokenHash [sha256.Size]byte,
	now time.Time,
) (User, error) {
	if s == nil || s.pool == nil {
		return User{}, ErrSessionStore
	}
	return scanUser(s.pool.QueryRow(ctx, `
		SELECT users.id::text, users.email, users.display_name, users.created_at
		  FROM sessions
		  JOIN users ON users.id = sessions.user_id
		 WHERE sessions.token_hash = $1
		   AND sessions.revoked_at IS NULL
		   AND sessions.expires_at > $2
	`, tokenHash[:], now))
}

func (s *PostgresSessionStore) RevokeSession(
	ctx context.Context,
	tokenHash [sha256.Size]byte,
	revokedAt time.Time,
) error {
	if s == nil || s.pool == nil {
		return ErrSessionStore
	}
	result, err := s.pool.Exec(ctx, `
		UPDATE sessions
		   SET revoked_at = $2
		 WHERE token_hash = $1
		   AND revoked_at IS NULL
		   AND expires_at > $2
	`, tokenHash[:], revokedAt)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func scanUser(row pgx.Row) (User, error) {
	var id string
	var email string
	var displayName string
	var createdAt pgtype.Timestamptz
	if err := row.Scan(&id, &email, &displayName, &createdAt); errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	} else if err != nil {
		return User{}, fmt.Errorf("read identity user: %w", err)
	}
	return NewUser(UserID(id), email, displayName, createdAt.Time)
}

func identityUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil || !parsed.Valid {
		return pgtype.UUID{}, fmt.Errorf("%w: %q", ErrIdentityPersistence, value)
	}
	return parsed, nil
}

func mapSessionError(err error) error {
	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) {
		return err
	}
	if databaseError.ConstraintName == "sessions_token_hash_key" || databaseError.ConstraintName == "sessions_pkey" {
		return ErrInvalidSession
	}
	return err
}
