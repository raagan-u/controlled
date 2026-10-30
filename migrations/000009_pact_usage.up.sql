CREATE TABLE pact_transaction_usage (
    pact_id uuid NOT NULL REFERENCES pacts (id) ON DELETE RESTRICT,
    owner_id uuid NOT NULL REFERENCES users (id),
    operation_type text NOT NULL CHECK (operation_type IN ('INTERNAL_TRANSFER', 'WITHDRAWAL')),
    operation_id uuid NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL CHECK (currency = 'USD'),
    status text NOT NULL CHECK (status IN ('COUNTED', 'REVERSED')),
    ledger_transaction_id uuid NOT NULL REFERENCES ledger_transactions (id),
    reversal_transaction_id uuid REFERENCES ledger_transactions (id),
    counted_at timestamptz NOT NULL,
    reversed_at timestamptz,
    PRIMARY KEY (pact_id, operation_type, operation_id),
    CHECK (
        (status = 'COUNTED' AND reversal_transaction_id IS NULL AND reversed_at IS NULL)
        OR
        (status = 'REVERSED' AND reversal_transaction_id IS NOT NULL AND reversed_at IS NOT NULL)
    )
);

CREATE INDEX pact_transaction_usage_owner_status_index
    ON pact_transaction_usage (owner_id, status, counted_at);

CREATE INDEX pact_transaction_usage_operation_index
    ON pact_transaction_usage (operation_type, operation_id);

