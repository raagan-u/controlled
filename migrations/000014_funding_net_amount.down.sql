ALTER TABLE funding_intents
    DROP CONSTRAINT funding_intents_net_amount_check,
    DROP COLUMN net_amount_minor;
