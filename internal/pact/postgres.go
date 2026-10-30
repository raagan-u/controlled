package pact

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raagan-u/controlled/internal/ledger"
)

type PostgresRepository struct {
	pool   *pgxpool.Pool
	ledger *ledger.PostgresBook
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool, ledger: ledger.NewPostgresBook(pool)}
}

func (r *PostgresRepository) ReserveStake(
	ctx context.Context,
	owner UserID,
	id PactID,
	journalID string,
	vaultCandidateID string,
	idempotencyKey string,
	now time.Time,
) (*Pact, bool, error) {
	if r == nil || r.pool == nil || r.ledger == nil {
		return nil, false, ErrUnavailable
	}
	ownerID, err := pactUUID(string(owner))
	if err != nil {
		return nil, false, err
	}
	pactID, err := pactUUID(string(id))
	if err != nil {
		return nil, false, err
	}
	transaction, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, false, fmt.Errorf("begin stake reservation: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	if _, err := transaction.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", "pact-stake:"+string(owner)+":"+string(id)); err != nil {
		return nil, false, fmt.Errorf("lock pact stake: %w", err)
	}
	current, err := scanPact(transaction.QueryRow(ctx, pactSelect+` WHERE id = $1 AND owner_id = $2 FOR UPDATE`, pactID, ownerID))
	if err != nil {
		return nil, false, err
	}
	if current.Status() == StatusScheduled || current.Status() == StatusActive {
		return current, true, nil
	}
	if current.Status() != StatusAwaitingStake {
		return nil, false, ErrInvalidTransition
	}
	availableAccount, vaultAccount, err := stakeAccounts(
		ctx, transaction, ownerID, current.Terms().Stake().Currency(), vaultCandidateID, now,
	)
	if err != nil {
		return nil, false, err
	}
	debit, err := ledger.NewEntry(availableAccount, ledger.SideDebit, current.Terms().Stake())
	if err != nil {
		return nil, false, err
	}
	credit, err := ledger.NewEntry(vaultAccount, ledger.SideCredit, current.Terms().Stake())
	if err != nil {
		return nil, false, err
	}
	journal, err := ledger.NewJournal(
		ledger.JournalID(journalID),
		"pact-stake:"+string(owner)+":"+idempotencyKey,
		ledger.Reference{Type: "PACT_STAKE_RESERVATION", ID: string(id)},
		"Reserve pact stake",
		now,
		[]ledger.Entry{debit, credit},
	)
	if err != nil {
		return nil, false, err
	}
	posted, err := r.ledger.PostInTx(ctx, transaction, journal)
	if errors.Is(err, ledger.ErrIdempotencyConflict) {
		return nil, false, ErrIdempotencyConflict
	}
	if err != nil {
		return nil, false, err
	}
	expectedVersion := current.Version()
	if err := current.RecordStakeReserved(StakeReservation{ID: string(posted.JournalID), Amount: current.Terms().Stake()}, now); err != nil {
		return nil, false, err
	}
	if err := updatePactLifecycle(ctx, transaction, current, ownerID, pactID, expectedVersion, now); err != nil {
		return nil, false, err
	}
	reservationID, err := pactUUID(string(posted.JournalID))
	if err != nil {
		return nil, false, err
	}
	_, err = transaction.Exec(ctx, `
		INSERT INTO stake_allocations (
			pact_id, owner_id, amount_minor, currency, status,
			reservation_transaction_id, created_at
		) VALUES ($1, $2, $3, $4, 'ACTIVE', $5, $6)
	`, pactID, ownerID, current.Terms().Stake().AmountMinor(), current.Terms().Stake().Currency(), reservationID, now)
	if err != nil {
		return nil, false, fmt.Errorf("create stake allocation: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit stake reservation: %w", err)
	}
	current.ClearEvents()
	return current, posted.Duplicate, nil
}

func (r *PostgresRepository) Cancel(
	ctx context.Context,
	owner UserID,
	id PactID,
	journalID string,
	now time.Time,
) (*Pact, bool, error) {
	if r == nil || r.pool == nil || r.ledger == nil {
		return nil, false, ErrUnavailable
	}
	ownerID, err := pactUUID(string(owner))
	if err != nil {
		return nil, false, err
	}
	pactID, err := pactUUID(string(id))
	if err != nil {
		return nil, false, err
	}
	transaction, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, false, fmt.Errorf("begin pact cancellation: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	current, err := scanPact(transaction.QueryRow(ctx, pactSelect+` WHERE id = $1 AND owner_id = $2 FOR UPDATE`, pactID, ownerID))
	if err != nil {
		return nil, false, err
	}
	if current.Status() == StatusCancelled {
		return current, true, nil
	}
	expectedVersion := current.Version()
	if err := current.Cancel(now); err != nil {
		return nil, false, err
	}
	events := current.Events()
	if len(events) == 0 {
		return nil, false, ErrInvalidTransition
	}
	if events[len(events)-1].StakeDisposition == StakeDispositionRelease {
		var amountMinor int64
		var currency string
		err := transaction.QueryRow(ctx, `
			SELECT amount_minor, currency
			  FROM stake_allocations
			 WHERE pact_id = $1 AND owner_id = $2 AND status = 'ACTIVE'
			 FOR UPDATE
		`, pactID, ownerID).Scan(&amountMinor, &currency)
		if err != nil {
			return nil, false, fmt.Errorf("read stake allocation for release: %w", err)
		}
		amount, err := NewMoney(amountMinor, currency)
		if err != nil {
			return nil, false, err
		}
		availableAccount, vaultAccount, err := existingStakeAccounts(ctx, transaction, ownerID, currency)
		if err != nil {
			return nil, false, err
		}
		debit, _ := ledger.NewEntry(vaultAccount, ledger.SideDebit, amount)
		credit, _ := ledger.NewEntry(availableAccount, ledger.SideCredit, amount)
		journal, err := ledger.NewJournal(
			ledger.JournalID(journalID), "pact-stake-release:"+string(id),
			ledger.Reference{Type: "PACT_STAKE_RELEASE", ID: string(id)},
			"Release cancelled pact stake", now, []ledger.Entry{debit, credit},
		)
		if err != nil {
			return nil, false, err
		}
		posted, err := r.ledger.PostInTx(ctx, transaction, journal)
		if err != nil {
			return nil, false, err
		}
		resolutionID, err := pactUUID(string(posted.JournalID))
		if err != nil {
			return nil, false, err
		}
		_, err = transaction.Exec(ctx, `
			UPDATE stake_allocations
			   SET status = 'RELEASED', resolution_transaction_id = $2, resolved_at = $3
			 WHERE pact_id = $1 AND status = 'ACTIVE'
		`, pactID, resolutionID, now)
		if err != nil {
			return nil, false, fmt.Errorf("release stake allocation: %w", err)
		}
	}
	if err := updatePactLifecycle(ctx, transaction, current, ownerID, pactID, expectedVersion, now); err != nil {
		return nil, false, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit pact cancellation: %w", err)
	}
	current.ClearEvents()
	return current, false, nil
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

func stakeAccounts(
	ctx context.Context,
	transaction pgx.Tx,
	ownerID pgtype.UUID,
	currency string,
	vaultCandidateID string,
	now time.Time,
) (ledger.AccountID, ledger.AccountID, error) {
	available, _, err := existingStakeAccounts(ctx, transaction, ownerID, currency)
	if err != nil && !errors.Is(err, ledger.ErrAccountNotFound) {
		return "", "", err
	}
	if available == "" {
		return "", "", ledger.ErrAccountNotFound
	}
	candidateID, err := pactUUID(vaultCandidateID)
	if err != nil {
		return "", "", err
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO ledger_accounts (id, kind, normal_side, currency, created_at)
		VALUES ($1, 'STAKE_VAULT', 'CREDIT', $2, $3)
		ON CONFLICT DO NOTHING
	`, candidateID, currency, now); err != nil {
		return "", "", fmt.Errorf("ensure stake vault account: %w", err)
	}
	var vault string
	if err := transaction.QueryRow(ctx, `
		SELECT id::text
		  FROM ledger_accounts
		 WHERE wallet_id IS NULL AND kind = 'STAKE_VAULT' AND currency = $1
	`, currency).Scan(&vault); err != nil {
		return "", "", fmt.Errorf("read stake vault account: %w", err)
	}
	return available, ledger.AccountID(vault), nil
}

func existingStakeAccounts(
	ctx context.Context,
	transaction pgx.Tx,
	ownerID pgtype.UUID,
	currency string,
) (ledger.AccountID, ledger.AccountID, error) {
	var available string
	err := transaction.QueryRow(ctx, `
		SELECT account.id::text
		  FROM wallets AS wallet
		  JOIN ledger_accounts AS account ON account.wallet_id = wallet.id
		 WHERE wallet.owner_id = $1
		   AND wallet.currency = $2
		   AND account.kind = 'AVAILABLE'
	`, ownerID, currency).Scan(&available)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ledger.ErrAccountNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("read available account for stake: %w", err)
	}
	var vault string
	err = transaction.QueryRow(ctx, `
		SELECT id::text
		  FROM ledger_accounts
		 WHERE wallet_id IS NULL AND kind = 'STAKE_VAULT' AND currency = $1
	`, currency).Scan(&vault)
	if errors.Is(err, pgx.ErrNoRows) {
		return ledger.AccountID(available), "", ledger.ErrAccountNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("read stake vault account: %w", err)
	}
	return ledger.AccountID(available), ledger.AccountID(vault), nil
}

func updatePactLifecycle(
	ctx context.Context,
	transaction pgx.Tx,
	value *Pact,
	ownerID pgtype.UUID,
	pactID pgtype.UUID,
	expectedVersion uint64,
	now time.Time,
) error {
	var reservationID any
	if value.StakeReservationID() != "" {
		parsed, err := pactUUID(value.StakeReservationID())
		if err != nil {
			return err
		}
		reservationID = parsed
	}
	result, err := transaction.Exec(ctx, `
		UPDATE pacts
		   SET status = $4,
		       outcome = $5,
		       stake_reservation_id = $6,
		       emergency_count = $7,
		       version = $8,
		       updated_at = $9
		 WHERE id = $1 AND owner_id = $2 AND version = $3
	`, pactID, ownerID, expectedVersion, value.Status(), value.Outcome(), reservationID,
		value.EmergencyCount(), value.Version(), now)
	if err != nil {
		return fmt.Errorf("update pact lifecycle: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrVersionConflict
	}
	return nil
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
