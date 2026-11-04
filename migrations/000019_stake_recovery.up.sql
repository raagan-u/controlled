ALTER TABLE stake_allocations
    ADD COLUMN recovered_minor bigint NOT NULL DEFAULT 0,
    ADD COLUMN fully_recovered_at timestamptz,
    ADD CONSTRAINT stake_allocations_recovered_amount_check
        CHECK (recovered_minor >= 0 AND recovered_minor <= amount_minor),
    ADD CONSTRAINT stake_allocations_recovered_time_check
        CHECK (
            (recovered_minor < amount_minor AND fully_recovered_at IS NULL)
            OR (recovered_minor = amount_minor AND fully_recovered_at IS NOT NULL)
        );

CREATE TABLE stake_recovery_applications (
    source_pact_id uuid NOT NULL REFERENCES stake_allocations (pact_id) ON DELETE RESTRICT,
    recovery_pact_id uuid NOT NULL REFERENCES pacts (id) ON DELETE RESTRICT,
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    ledger_transaction_id uuid NOT NULL REFERENCES ledger_transactions (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (source_pact_id, recovery_pact_id)
);

CREATE INDEX stake_recovery_applications_recovery_pact_index
    ON stake_recovery_applications (recovery_pact_id, created_at);

CREATE INDEX stake_allocations_recovery_queue_index
    ON stake_allocations (owner_id, resolved_at, pact_id)
    WHERE status = 'COMMITMENT_RESERVED';
