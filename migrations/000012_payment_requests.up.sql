CREATE TABLE payment_requests (
    id uuid PRIMARY KEY,
    requester_id uuid NOT NULL REFERENCES users(id),
    recipient_id uuid NOT NULL REFERENCES users(id),
    idempotency_key text NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL CHECK (currency = 'USD'),
    note text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PAID', 'DECLINED', 'CANCELLED')),
    created_at timestamptz NOT NULL,
    UNIQUE (requester_id, idempotency_key),
    CHECK (requester_id <> recipient_id)
);

CREATE INDEX payment_requests_recipient_created_idx
    ON payment_requests (recipient_id, created_at DESC);
