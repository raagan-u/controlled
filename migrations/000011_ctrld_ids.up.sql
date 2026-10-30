ALTER TABLE users ADD COLUMN ctrld_id text;

CREATE UNIQUE INDEX users_ctrld_id_unique
    ON users (ctrld_id)
    WHERE ctrld_id IS NOT NULL;

DO $$
DECLARE
    target record;
    candidate text;
BEGIN
    FOR target IN SELECT id FROM users WHERE ctrld_id IS NULL LOOP
        LOOP
            candidate := 'ctrld' || lpad(floor(random() * 100000)::integer::text, 5, '0');
            BEGIN
                UPDATE users SET ctrld_id = candidate WHERE id = target.id;
                EXIT;
            EXCEPTION WHEN unique_violation THEN
                NULL;
            END;
        END LOOP;
    END LOOP;
END;
$$;

ALTER TABLE users
    ALTER COLUMN ctrld_id SET NOT NULL,
    ADD CONSTRAINT users_ctrld_id_format CHECK (ctrld_id ~ '^ctrld[0-9]{5}$');

CREATE FUNCTION assign_ctrld_id()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    candidate text;
BEGIN
    IF NEW.ctrld_id IS NOT NULL THEN
        RETURN NEW;
    END IF;
    PERFORM pg_advisory_xact_lock(hashtext('users_ctrld_id_generation'));
    LOOP
        candidate := 'ctrld' || lpad(floor(random() * 100000)::integer::text, 5, '0');
        IF NOT EXISTS (SELECT 1 FROM users WHERE ctrld_id = candidate) THEN
            NEW.ctrld_id := candidate;
            RETURN NEW;
        END IF;
    END LOOP;
END;
$$;

CREATE TRIGGER users_assign_ctrld_id
BEFORE INSERT ON users
FOR EACH ROW
EXECUTE FUNCTION assign_ctrld_id();
