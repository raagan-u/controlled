ALTER TABLE payment_requests DROP CONSTRAINT payment_requests_status_check;

ALTER TABLE payment_requests
    ADD COLUMN expires_at timestamptz,
    ADD COLUMN updated_at timestamptz,
    ADD COLUMN resolved_at timestamptz,
    ADD COLUMN payment_key text,
    ADD COLUMN transfer_id uuid REFERENCES internal_transfers(id),
    ADD COLUMN accountability_request_id uuid REFERENCES accountability_requests(id);

UPDATE payment_requests
SET expires_at = created_at + interval '7 days', updated_at = created_at;

ALTER TABLE payment_requests
    ALTER COLUMN expires_at SET NOT NULL,
    ALTER COLUMN updated_at SET NOT NULL,
    ADD CONSTRAINT payment_requests_status_check
        CHECK (status IN ('PENDING', 'PROCESSING', 'AWAITING_APPROVAL', 'PAID', 'DECLINED', 'CANCELLED', 'EXPIRED')),
    ADD CONSTRAINT payment_requests_resolution_check
        CHECK ((status IN ('PAID', 'DECLINED', 'CANCELLED', 'EXPIRED')) = (resolved_at IS NOT NULL)),
    ADD CONSTRAINT payment_requests_expiry_check CHECK (expires_at > created_at);

CREATE UNIQUE INDEX payment_requests_transfer_unique
    ON payment_requests (transfer_id) WHERE transfer_id IS NOT NULL;
