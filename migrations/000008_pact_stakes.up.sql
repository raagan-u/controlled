CREATE TABLE stake_allocations (
    pact_id uuid PRIMARY KEY REFERENCES pacts (id) ON DELETE RESTRICT,
    owner_id uuid NOT NULL REFERENCES users (id),
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL CHECK (currency = 'USD'),
    status text NOT NULL CHECK (status IN ('ACTIVE', 'RELEASED', 'COMMITMENT_RESERVED')),
    reservation_transaction_id uuid NOT NULL UNIQUE REFERENCES ledger_transactions (id),
    resolution_transaction_id uuid UNIQUE REFERENCES ledger_transactions (id),
    created_at timestamptz NOT NULL,
    resolved_at timestamptz,
    CHECK (
        (status = 'ACTIVE' AND resolution_transaction_id IS NULL AND resolved_at IS NULL)
        OR (status IN ('RELEASED', 'COMMITMENT_RESERVED') AND resolution_transaction_id IS NOT NULL AND resolved_at IS NOT NULL)
    )
);

CREATE INDEX stake_allocations_owner_status_index
    ON stake_allocations (owner_id, status);

CREATE FUNCTION validate_stake_allocation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    pact_owner uuid;
    pact_stake bigint;
    pact_currency text;
    pact_reservation uuid;
BEGIN
    SELECT owner_id, stake_minor, currency, stake_reservation_id
      INTO pact_owner, pact_stake, pact_currency, pact_reservation
      FROM pacts
     WHERE id = NEW.pact_id;

    IF pact_owner IS NULL
       OR NEW.owner_id <> pact_owner
       OR NEW.amount_minor <> pact_stake
       OR NEW.currency <> pact_currency
       OR NEW.reservation_transaction_id <> pact_reservation THEN
        RAISE EXCEPTION 'stake allocation must match its pact and reservation'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER stake_allocation_validation
BEFORE INSERT OR UPDATE ON stake_allocations
FOR EACH ROW
EXECUTE FUNCTION validate_stake_allocation();
