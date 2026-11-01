package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/controlled/internal/identity"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Register(ctx context.Context, userID identity.UserID, token, platform string, now time.Time) error {
	if r == nil || r.pool == nil {
		return ErrUnavailable
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO push_devices (token, user_id, platform, created_at, updated_at)
		VALUES ($1, $2::text::uuid, $3, $4, $4)
		ON CONFLICT (token) DO UPDATE
		SET user_id = EXCLUDED.user_id, platform = EXCLUDED.platform, updated_at = EXCLUDED.updated_at
	`, token, userID, platform, now)
	if err != nil {
		return fmt.Errorf("register push device: %w", err)
	}
	return nil
}

func (r *PostgresRepository) Unregister(ctx context.Context, userID identity.UserID, token string) error {
	if r == nil || r.pool == nil {
		return ErrUnavailable
	}
	if _, err := r.pool.Exec(ctx, `DELETE FROM push_devices WHERE token=$1 AND user_id=$2::text::uuid`, token, userID); err != nil {
		return fmt.Errorf("unregister push device: %w", err)
	}
	return nil
}

func (r *PostgresRepository) Tokens(ctx context.Context, userID identity.UserID) ([]string, error) {
	if r == nil || r.pool == nil {
		return nil, ErrUnavailable
	}
	rows, err := r.pool.Query(ctx, `SELECT token FROM push_devices WHERE user_id=$1::text::uuid ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("read push devices: %w", err)
	}
	defer rows.Close()
	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, fmt.Errorf("scan push device: %w", err)
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

func (r *PostgresRepository) RemoveTokens(ctx context.Context, tokens []string) error {
	if r == nil || r.pool == nil {
		return ErrUnavailable
	}
	if len(tokens) == 0 {
		return nil
	}
	if _, err := r.pool.Exec(ctx, `DELETE FROM push_devices WHERE token=ANY($1)`, tokens); err != nil {
		return fmt.Errorf("remove push devices: %w", err)
	}
	return nil
}
