CREATE TABLE funding_intents (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users (id),
    wallet_id uuid NOT NULL REFERENCES wallets (id),
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    status text NOT NULL CHECK (status IN ('CREATED', 'AWAITING_CONFIRMATION', 'COMPLETED', 'FAILED')),
    provider_order_id text UNIQUE,
    approval_url text,
    ledger_transaction_id uuid UNIQUE REFERENCES ledger_transactions (id),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    completed_at timestamptz,
    UNIQUE (owner_id, idempotency_key),
    CHECK (updated_at >= created_at),
    CHECK (
        status = 'CREATED'
        OR (provider_order_id IS NOT NULL AND approval_url IS NOT NULL)
    ),
    CHECK (
        status <> 'COMPLETED'
        OR (ledger_transaction_id IS NOT NULL AND completed_at IS NOT NULL)
    )
);

CREATE TABLE internal_transfers (
    id uuid PRIMARY KEY,
    sender_wallet_id uuid NOT NULL REFERENCES wallets (id),
    recipient_wallet_id uuid NOT NULL REFERENCES wallets (id),
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    ledger_transaction_id uuid NOT NULL UNIQUE REFERENCES ledger_transactions (id),
    created_at timestamptz NOT NULL,
    UNIQUE (sender_wallet_id, idempotency_key),
    CHECK (sender_wallet_id <> recipient_wallet_id)
);

CREATE TABLE withdrawals (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users (id),
    wallet_id uuid NOT NULL REFERENCES wallets (id),
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    destination text NOT NULL CHECK (btrim(destination) <> ''),
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    status text NOT NULL CHECK (status IN ('RESERVED', 'PENDING', 'COMPLETED', 'FAILED')),
    provider_payout_id text UNIQUE,
    reservation_transaction_id uuid NOT NULL UNIQUE REFERENCES ledger_transactions (id),
    settlement_transaction_id uuid UNIQUE REFERENCES ledger_transactions (id),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    completed_at timestamptz,
    UNIQUE (owner_id, idempotency_key),
    CHECK (updated_at >= created_at),
    CHECK (status = 'RESERVED' OR provider_payout_id IS NOT NULL),
    CHECK (
        status NOT IN ('COMPLETED', 'FAILED')
        OR (settlement_transaction_id IS NOT NULL AND completed_at IS NOT NULL)
    )
);

CREATE INDEX funding_intents_owner_created_index
    ON funding_intents (owner_id, created_at DESC);

CREATE INDEX internal_transfers_sender_created_index
    ON internal_transfers (sender_wallet_id, created_at DESC);

CREATE INDEX withdrawals_owner_created_index
    ON withdrawals (owner_id, created_at DESC);
