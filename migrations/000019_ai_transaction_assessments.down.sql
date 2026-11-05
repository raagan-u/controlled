ALTER TABLE internal_transfers
    DROP COLUMN is_purchase,
    DROP COLUMN guidance_assessment_id;

DROP TABLE ai_transaction_assessments;
