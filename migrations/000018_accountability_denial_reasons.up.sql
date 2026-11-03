ALTER TABLE accountability_assignments
    ADD COLUMN decision_reason text;

ALTER TABLE accountability_assignments
    ADD CONSTRAINT accountability_assignment_decision_reason_check
    CHECK (decision_reason IS NULL OR (decision = 'DENY' AND btrim(decision_reason) <> ''));
