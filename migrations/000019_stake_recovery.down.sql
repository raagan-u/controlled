DROP INDEX IF EXISTS stake_allocations_recovery_queue_index;
DROP INDEX IF EXISTS stake_recovery_applications_recovery_pact_index;
DROP TABLE IF EXISTS stake_recovery_applications;

ALTER TABLE stake_allocations
    DROP CONSTRAINT IF EXISTS stake_allocations_recovered_time_check,
    DROP CONSTRAINT IF EXISTS stake_allocations_recovered_amount_check,
    DROP COLUMN IF EXISTS fully_recovered_at,
    DROP COLUMN IF EXISTS recovered_minor;
