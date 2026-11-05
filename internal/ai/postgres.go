package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/controlled/internal/identity"
)

type StoredAssessment struct {
	ID                string
	OwnerID           identity.UserID
	PrimaryPactID     string
	AmountMinor       int64
	Remark            string
	IsPurchase        bool
	Relationship      Relationship
	Confidence        Confidence
	ClassifierVersion string
	CreatedAt         time.Time
	ExpiresAt         time.Time
}

type AssessmentStore interface {
	SaveAssessment(context.Context, StoredAssessment) error
}

type PostgresAssessmentStore struct{ pool *pgxpool.Pool }

func NewPostgresAssessmentStore(pool *pgxpool.Pool) *PostgresAssessmentStore {
	return &PostgresAssessmentStore{pool: pool}
}

func (s *PostgresAssessmentStore) SaveAssessment(ctx context.Context, value StoredAssessment) error {
	if s == nil || s.pool == nil {
		return ErrUnavailable
	}
	var pactID any
	if value.PrimaryPactID != "" {
		pactID = value.PrimaryPactID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ai_transaction_assessments (
			id, owner_id, primary_pact_id, amount_minor, currency, remark,
			is_purchase, relationship, confidence, classifier_version, created_at, expires_at
		) VALUES ($1::text::uuid, $2::text::uuid, $3::text::uuid, $4, 'USD', $5, $6, $7, $8, $9, $10, $11)
	`, value.ID, string(value.OwnerID), pactID, value.AmountMinor, value.Remark, value.IsPurchase,
		value.Relationship, value.Confidence, value.ClassifierVersion, value.CreatedAt, value.ExpiresAt)
	if err != nil {
		return fmt.Errorf("save AI transaction assessment: %w", err)
	}
	return nil
}
