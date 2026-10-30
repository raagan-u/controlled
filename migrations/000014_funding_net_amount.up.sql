ALTER TABLE funding_intents
    ADD COLUMN net_amount_minor bigint;

UPDATE funding_intents
   SET net_amount_minor = amount_minor
 WHERE status = 'COMPLETED';

ALTER TABLE funding_intents
    ADD CONSTRAINT funding_intents_net_amount_check CHECK (
        (status <> 'COMPLETED' AND net_amount_minor IS NULL)
        OR (
            status = 'COMPLETED'
            AND net_amount_minor > 0
            AND net_amount_minor <= amount_minor
        )
    );
