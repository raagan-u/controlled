CREATE TABLE pacts (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users (id),
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    goal text NOT NULL CHECK (btrim(goal) <> ''),
    intent text NOT NULL CHECK (btrim(intent) <> ''),
    spending_limit_minor bigint NOT NULL CHECK (spending_limit_minor > 0),
    stake_minor bigint NOT NULL CHECK (stake_minor > 0),
    currency text NOT NULL CHECK (currency = 'USD'),
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    accountability_circle_id uuid NOT NULL REFERENCES accountability_circles (id) ON DELETE RESTRICT,
    break_consequence text NOT NULL CHECK (break_consequence = 'COMMITMENT_RESERVE'),
    status text NOT NULL CHECK (status IN ('DRAFT', 'AWAITING_STAKE', 'SCHEDULED', 'ACTIVE', 'RESOLVED', 'CANCELLED')),
    outcome text NOT NULL CHECK (outcome IN ('NONE', 'KEPT', 'KEPT_WITH_EXCEPTIONS', 'BROKEN', 'CANCELLED')),
    stake_reservation_id uuid UNIQUE REFERENCES ledger_transactions (id),
    emergency_count integer NOT NULL DEFAULT 0 CHECK (emergency_count >= 0),
    version bigint NOT NULL CHECK (version > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (owner_id, idempotency_key),
    CHECK (starts_at < ends_at),
    CHECK (updated_at >= created_at),
    CHECK (
        (status IN ('DRAFT', 'AWAITING_STAKE') AND outcome = 'NONE' AND stake_reservation_id IS NULL)
        OR (status IN ('SCHEDULED', 'ACTIVE') AND outcome = 'NONE' AND stake_reservation_id IS NOT NULL)
        OR (status = 'RESOLVED' AND outcome IN ('KEPT', 'KEPT_WITH_EXCEPTIONS', 'BROKEN') AND stake_reservation_id IS NOT NULL)
        OR (status = 'CANCELLED' AND outcome = 'CANCELLED')
    )
);

CREATE INDEX pacts_owner_status_period_index
    ON pacts (owner_id, status, starts_at, ends_at);

CREATE FUNCTION protect_confirmed_pact_terms()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.status <> 'DRAFT' AND (
        NEW.goal IS DISTINCT FROM OLD.goal
        OR NEW.intent IS DISTINCT FROM OLD.intent
        OR NEW.spending_limit_minor IS DISTINCT FROM OLD.spending_limit_minor
        OR NEW.stake_minor IS DISTINCT FROM OLD.stake_minor
        OR NEW.currency IS DISTINCT FROM OLD.currency
        OR NEW.starts_at IS DISTINCT FROM OLD.starts_at
        OR NEW.ends_at IS DISTINCT FROM OLD.ends_at
        OR NEW.accountability_circle_id IS DISTINCT FROM OLD.accountability_circle_id
        OR NEW.break_consequence IS DISTINCT FROM OLD.break_consequence
    ) THEN
        RAISE EXCEPTION 'confirmed pact terms are immutable'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER pacts_protect_confirmed_terms
BEFORE UPDATE ON pacts
FOR EACH ROW
EXECUTE FUNCTION protect_confirmed_pact_terms();
