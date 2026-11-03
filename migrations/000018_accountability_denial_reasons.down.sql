ALTER TABLE accountability_assignments
    DROP CONSTRAINT accountability_assignment_decision_reason_check;

ALTER TABLE accountability_assignments
    DROP COLUMN decision_reason;
