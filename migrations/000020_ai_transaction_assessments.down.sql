ALTER TABLE internal_transfers
    DROP COLUMN is_purchase,
    DROP COLUMN guidance_assessment_id;

ALTER TABLE withdrawals DROP COLUMN remark;

DROP TABLE ai_transaction_assessments;
