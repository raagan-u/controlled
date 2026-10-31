CREATE TABLE accountability_invitations (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users (id),
    partner_id uuid NOT NULL REFERENCES users (id),
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    status text NOT NULL CHECK (status IN ('PENDING', 'ACCEPTED', 'DECLINED')),
    created_at timestamptz NOT NULL,
    resolved_at timestamptz,
    UNIQUE (owner_id, idempotency_key),
    CHECK (owner_id <> partner_id),
    CHECK (
        (status = 'PENDING' AND resolved_at IS NULL)
        OR (status IN ('ACCEPTED', 'DECLINED') AND resolved_at IS NOT NULL AND resolved_at >= created_at)
    )
);

CREATE UNIQUE INDEX accountability_active_relationship_unique
    ON accountability_invitations (owner_id, partner_id)
    WHERE status IN ('PENDING', 'ACCEPTED');

CREATE INDEX accountability_invitations_partner_index
    ON accountability_invitations (partner_id, status, created_at DESC);

CREATE TABLE accountability_circles (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users (id),
    name text NOT NULL CHECK (btrim(name) <> ''),
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    created_at timestamptz NOT NULL,
    UNIQUE (owner_id, idempotency_key)
);

CREATE TABLE accountability_circle_members (
    circle_id uuid NOT NULL REFERENCES accountability_circles (id) ON DELETE RESTRICT,
    position integer NOT NULL CHECK (position >= 0),
    partner_id uuid NOT NULL REFERENCES users (id),
    PRIMARY KEY (circle_id, position),
    UNIQUE (circle_id, partner_id)
);

CREATE FUNCTION validate_accountability_circle_member()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    circle_owner uuid;
BEGIN
    SELECT owner_id INTO circle_owner
      FROM accountability_circles
     WHERE id = NEW.circle_id;

    IF circle_owner IS NULL OR circle_owner = NEW.partner_id THEN
        RAISE EXCEPTION 'circle partner must be distinct from its owner'
            USING ERRCODE = '23514';
    END IF;

    IF NOT EXISTS (
        SELECT 1
          FROM accountability_invitations
         WHERE owner_id = circle_owner
           AND partner_id = NEW.partner_id
           AND status = 'ACCEPTED'
    ) THEN
        RAISE EXCEPTION 'circle partner must have accepted an invitation'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER accountability_circle_member_validation
BEFORE INSERT ON accountability_circle_members
FOR EACH ROW
EXECUTE FUNCTION validate_accountability_circle_member();

CREATE FUNCTION reject_accountability_circle_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'accountability circles are immutable; create a new circle instead'
        USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER accountability_circles_immutable
BEFORE UPDATE OR DELETE ON accountability_circles
FOR EACH ROW
EXECUTE FUNCTION reject_accountability_circle_mutation();

CREATE TRIGGER accountability_circle_members_immutable
BEFORE UPDATE OR DELETE ON accountability_circle_members
FOR EACH ROW
EXECUTE FUNCTION reject_accountability_circle_mutation();
