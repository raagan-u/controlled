DROP INDEX payment_requests_transfer_unique;
UPDATE payment_requests SET status = 'PENDING', resolved_at = NULL
WHERE status IN ('PROCESSING', 'AWAITING_APPROVAL');
UPDATE payment_requests SET status = 'CANCELLED'
WHERE status = 'EXPIRED';
ALTER TABLE payment_requests
    DROP CONSTRAINT payment_requests_expiry_check,
    DROP CONSTRAINT payment_requests_resolution_check,
    DROP CONSTRAINT payment_requests_status_check,
    DROP COLUMN accountability_request_id,
    DROP COLUMN transfer_id,
    DROP COLUMN payment_key,
    DROP COLUMN resolved_at,
    DROP COLUMN updated_at,
    DROP COLUMN expires_at,
    ADD CONSTRAINT payment_requests_status_check CHECK (status IN ('PENDING', 'PAID', 'DECLINED', 'CANCELLED'));
