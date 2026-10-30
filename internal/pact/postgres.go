package pact

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	idempotencyKey string,
	pact *Pact,
) (*Pact, bool, error) {
	if r == nil || r.pool == nil {
		return nil, false, ErrUnavailable
	}
	id, ownerID, circleID, err := pactUUIDs(pact)
	if err != nil {
		return nil, false, err
	}
	events := pact.Events()
	if len(events) == 0 {
		return nil, false, ErrInvalidTimestamp
	}
	terms := pact.Terms()
	result, err := r.pool.Exec(ctx, `
		INSERT INTO pacts (
			id, owner_id, idempotency_key, goal, intent,
			spending_limit_minor, stake_minor, currency,
			starts_at, ends_at, accountability_circle_id, break_consequence,
			status, outcome, emergency_count, version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15, $16, $17, $17
		)
		ON CONFLICT (owner_id, idempotency_key) DO NOTHING
	`, id, ownerID, idempotencyKey, terms.Goal(), terms.Intent(),
		terms.SpendingLimit().AmountMinor(), terms.Stake().AmountMinor(), terms.SpendingLimit().Currency(),
		terms.StartsAt(), terms.EndsAt(), circleID, terms.BreakConsequence(),
		pact.Status(), pact.Outcome(), pact.EmergencyCount(), pact.Version(), events[0].OccurredAt)
	if err != nil {
		return nil, false, fmt.Errorf("create pact: %w", err)
	}
	if result.RowsAffected() == 1 {
		pact.ClearEvents()
		return pact, false, nil
	}
	existing, err := r.byKey(ctx, ownerID, idempotencyKey)
	if err != nil {
		return nil, false, err
	}
	if !existing.Terms().Equal(pact.Terms()) {
		return nil, false, ErrIdempotencyConflict
	}
	return existing, true, nil
}

func (r *PostgresRepository) Save(ctx context.Context, pact *Pact, expectedVersion uint64) error {
	if r == nil || r.pool == nil {
		return ErrUnavailable
	}
	id, ownerID, circleID, err := pactUUIDs(pact)
	if err != nil {
		return err
	}
	var reservationID any
	if pact.StakeReservationID() != "" {
		reservationID, err = pactUUID(pact.StakeReservationID())
		if err != nil {
			return err
		}
	}
	terms := pact.Terms()
	result, err := r.pool.Exec(ctx, `
		UPDATE pacts
		   SET goal = $4,
		       intent = $5,
		       spending_limit_minor = $6,
		       stake_minor = $7,
		       currency = $8,
		       starts_at = $9,
		       ends_at = $10,
		       accountability_circle_id = $11,
		       break_consequence = $12,
		       status = $13,
		       outcome = $14,
		       stake_reservation_id = $15,
		       emergency_count = $16,
		       version = $17,
		       updated_at = now()
		 WHERE id = $1 AND owner_id = $2 AND version = $3
	`, id, ownerID, expectedVersion, terms.Goal(), terms.Intent(),
		terms.SpendingLimit().AmountMinor(), terms.Stake().AmountMinor(), terms.SpendingLimit().Currency(),
		terms.StartsAt(), terms.EndsAt(), circleID, terms.BreakConsequence(),
		pact.Status(), pact.Outcome(), reservationID, pact.EmergencyCount(), pact.Version())
	if err != nil {
		return fmt.Errorf("save pact: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrVersionConflict
	}
	pact.ClearEvents()
	return nil
}

func (r *PostgresRepository) ByID(ctx context.Context, owner UserID, id PactID) (*Pact, error) {
	if r == nil || r.pool == nil {
		return nil, ErrUnavailable
	}
	ownerID, err := pactUUID(string(owner))
	if err != nil {
		return nil, err
	}
	pactID, err := pactUUID(string(id))
	if err != nil {
		return nil, err
	}
	return scanPact(r.pool.QueryRow(ctx, pactSelect+` WHERE id = $1 AND owner_id = $2`, pactID, ownerID))
}

func (r *PostgresRepository) ForOwner(ctx context.Context, owner UserID) ([]*Pact, error) {
	if r == nil || r.pool == nil {
		return nil, ErrUnavailable
	}
	ownerID, err := pactUUID(string(owner))
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, pactSelect+` WHERE owner_id = $1 ORDER BY created_at DESC, id`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list pacts: %w", err)
	}
	defer rows.Close()
	var pacts []*Pact
	for rows.Next() {
		value, err := scanPact(rows)
		if err != nil {
			return nil, err
		}
		pacts = append(pacts, value)
	}
	return pacts, rows.Err()
}

func (r *PostgresRepository) CircleFacts(
	ctx context.Context,
	owner UserID,
	circle CircleID,
) (ConfirmationFacts, error) {
	if r == nil || r.pool == nil {
		return ConfirmationFacts{}, ErrUnavailable
	}
	ownerID, err := pactUUID(string(owner))
	if err != nil {
		return ConfirmationFacts{}, err
	}
	circleID, err := pactUUID(string(circle))
	if err != nil {
		return ConfirmationFacts{}, err
	}
	var partnerCount, distinctPartnerCount int
	var ownerIsPartner bool
	err = r.pool.QueryRow(ctx, `
		SELECT count(member.partner_id)::int,
		       count(DISTINCT member.partner_id)::int,
		       COALESCE(bool_or(member.partner_id = circle.owner_id), false)
		  FROM accountability_circles AS circle
		  LEFT JOIN accountability_circle_members AS member ON member.circle_id = circle.id
		 WHERE circle.id = $1 AND circle.owner_id = $2
		 GROUP BY circle.id
	`, circleID, ownerID).Scan(&partnerCount, &distinctPartnerCount, &ownerIsPartner)
	if errors.Is(err, pgx.ErrNoRows) {
		return ConfirmationFacts{}, ErrAccountabilityNotReady
	}
	if err != nil {
		return ConfirmationFacts{}, fmt.Errorf("read pact circle facts: %w", err)
	}
	return ConfirmationFacts{
		PartnerCount: partnerCount, DistinctPartnerCount: distinctPartnerCount, OwnerIsPartner: ownerIsPartner,
	}, nil
}

func (r *PostgresRepository) byKey(ctx context.Context, ownerID pgtype.UUID, key string) (*Pact, error) {
	return scanPact(r.pool.QueryRow(ctx, pactSelect+` WHERE owner_id = $1 AND idempotency_key = $2`, ownerID, key))
}

const pactSelect = `
	SELECT id::text, owner_id::text, goal, intent,
	       spending_limit_minor, stake_minor, currency,
	       starts_at, ends_at, accountability_circle_id::text, break_consequence,
	       status, outcome, stake_reservation_id::text,
	       emergency_count, version
	  FROM pacts`

func scanPact(row pgx.Row) (*Pact, error) {
	var id, ownerID, goal, intent, currency, circleID string
	var limitMinor, stakeMinor int64
	var startsAt, endsAt pgtype.Timestamptz
	var consequence BreakConsequence
	var status Status
	var outcome Outcome
	var reservationID pgtype.Text
	var emergencyCount int
	var version int64
	if err := row.Scan(
		&id, &ownerID, &goal, &intent, &limitMinor, &stakeMinor, &currency,
		&startsAt, &endsAt, &circleID, &consequence, &status, &outcome,
		&reservationID, &emergencyCount, &version,
	); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPactNotFound
	} else if err != nil {
		return nil, fmt.Errorf("read pact: %w", err)
	}
	limit, err := NewMoney(limitMinor, currency)
	if err != nil {
		return nil, err
	}
	stake, err := NewMoney(stakeMinor, currency)
	if err != nil {
		return nil, err
	}
	terms, err := NewTerms(goal, intent, limit, stake, startsAt.Time, endsAt.Time, CircleID(circleID), consequence)
	if err != nil {
		return nil, err
	}
	return Restore(
		PactID(id), UserID(ownerID), terms, status, outcome,
		reservationID.String, emergencyCount, uint64(version),
	)
}

func pactUUIDs(value *Pact) (pgtype.UUID, pgtype.UUID, pgtype.UUID, error) {
	if value == nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, ErrInvalidPactID
	}
	id, err := pactUUID(string(value.ID()))
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	ownerID, err := pactUUID(string(value.OwnerID()))
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, err
	}
	circleID, err := pactUUID(string(value.Terms().AccountabilityCircleID()))
	return id, ownerID, circleID, err
}

func pactUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil || !parsed.Valid {
		return pgtype.UUID{}, fmt.Errorf("%w: %q", ErrPersistenceID, value)
	}
	return parsed, nil
}
