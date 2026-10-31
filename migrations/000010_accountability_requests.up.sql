CREATE TABLE transaction_intents (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users (id),
    operation_type text NOT NULL CHECK (operation_type IN ('INTERNAL_TRANSFER', 'WITHDRAWAL')),
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    destination text NOT NULL CHECK (btrim(destination) <> ''),
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL CHECK (currency = 'USD'),
    status text NOT NULL CHECK (status IN ('APPROVAL_REQUIRED', 'APPROVED', 'COMPLETED', 'FAILED', 'CANCELLED', 'EXPIRED')),
    executed_operation_id uuid,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (owner_id, operation_type, idempotency_key),
    CHECK (updated_at >= created_at)
);

CREATE TABLE accountability_requests (
    id uuid PRIMARY KEY,
    intent_id uuid NOT NULL UNIQUE REFERENCES transaction_intents (id) ON DELETE RESTRICT,
    owner_id uuid NOT NULL REFERENCES users (id),
    owner_reason text NOT NULL CHECK (btrim(owner_reason) <> ''),
    status text NOT NULL CHECK (status IN ('ASSIGNED', 'APPROVED', 'DENIED', 'EXPIRED', 'CANCELLED', 'FAILED')),
    classification text CHECK (classification IN ('EMERGENCY', 'CONSCIOUS_BREAK')),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    resolved_at timestamptz,
    CHECK (expires_at > created_at),
    CHECK (
        (status = 'ASSIGNED' AND classification IS NULL AND resolved_at IS NULL)
        OR
        (status = 'APPROVED' AND classification IS NOT NULL AND resolved_at IS NOT NULL)
        OR
        (status IN ('DENIED', 'EXPIRED', 'CANCELLED', 'FAILED') AND classification IS NULL AND resolved_at IS NOT NULL)
    )
);

CREATE INDEX accountability_requests_owner_status_index
    ON accountability_requests (owner_id, status, created_at DESC);

CREATE TABLE accountability_request_pacts (
    request_id uuid NOT NULL REFERENCES accountability_requests (id) ON DELETE RESTRICT,
    pact_id uuid NOT NULL REFERENCES pacts (id) ON DELETE RESTRICT,
    limit_minor bigint NOT NULL CHECK (limit_minor > 0),
    used_minor bigint NOT NULL CHECK (used_minor >= 0),
    proposed_minor bigint NOT NULL CHECK (proposed_minor > limit_minor),
    overage_minor bigint NOT NULL CHECK (overage_minor = proposed_minor - limit_minor),
    PRIMARY KEY (request_id, pact_id)
);

CREATE TABLE accountability_request_reviewers (
    request_id uuid NOT NULL REFERENCES accountability_requests (id) ON DELETE RESTRICT,
    position integer NOT NULL CHECK (position >= 0),
    partner_id uuid NOT NULL REFERENCES users (id),
    PRIMARY KEY (request_id, position),
    UNIQUE (request_id, partner_id)
);

CREATE TABLE accountability_assignments (
    id uuid PRIMARY KEY,
    request_id uuid NOT NULL REFERENCES accountability_requests (id) ON DELETE RESTRICT,
    reviewer_position integer NOT NULL,
    partner_id uuid NOT NULL REFERENCES users (id),
    status text NOT NULL CHECK (status IN ('ACTIVE', 'RESPONDED', 'TIMED_OUT', 'SUPERSEDED')),
    assigned_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    responded_at timestamptz,
    decision text CHECK (decision IN ('APPROVE_EMERGENCY', 'APPROVE_CONSCIOUS_BREAK', 'DENY')),
    FOREIGN KEY (request_id, reviewer_position)
        REFERENCES accountability_request_reviewers (request_id, position),
    CHECK (expires_at > assigned_at),
    CHECK (
        (status = 'ACTIVE' AND responded_at IS NULL AND decision IS NULL)
        OR
        (status = 'RESPONDED' AND responded_at IS NOT NULL AND decision IS NOT NULL)
        OR
        (status IN ('TIMED_OUT', 'SUPERSEDED') AND responded_at IS NOT NULL AND decision IS NULL)
    )
);

CREATE UNIQUE INDEX accountability_assignment_active_unique
    ON accountability_assignments (request_id)
    WHERE status = 'ACTIVE';

CREATE INDEX accountability_assignment_partner_index
    ON accountability_assignments (partner_id, status, assigned_at DESC);

CREATE TABLE controlled_audit_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id uuid REFERENCES users (id),
    aggregate_type text NOT NULL CHECK (btrim(aggregate_type) <> ''),
    aggregate_id uuid NOT NULL,
    event_type text NOT NULL CHECK (btrim(event_type) <> ''),
    facts jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL
);

CREATE INDEX controlled_audit_events_aggregate_index
    ON controlled_audit_events (aggregate_type, aggregate_id, occurred_at, id);

