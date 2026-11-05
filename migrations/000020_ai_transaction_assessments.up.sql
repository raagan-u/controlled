CREATE TABLE ai_transaction_assessments (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    primary_pact_id uuid REFERENCES pacts(id) ON DELETE SET NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL DEFAULT 'USD',
    remark text NOT NULL,
    is_purchase boolean NOT NULL DEFAULT false,
    relationship text NOT NULL CHECK (relationship IN ('ALIGNED', 'CONFLICTING', 'EXCEPTIONAL', 'UNCLEAR')),
    confidence text NOT NULL CHECK (confidence IN ('HIGH', 'MEDIUM', 'LOW')),
    classifier_version text NOT NULL,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL
);

CREATE INDEX ai_transaction_assessments_owner_created_idx
    ON ai_transaction_assessments (owner_id, created_at DESC);

ALTER TABLE internal_transfers
    ADD COLUMN guidance_assessment_id uuid REFERENCES ai_transaction_assessments(id) ON DELETE SET NULL,
    ADD COLUMN is_purchase boolean NOT NULL DEFAULT false;

ALTER TABLE withdrawals
    ADD COLUMN remark text NOT NULL DEFAULT '';
