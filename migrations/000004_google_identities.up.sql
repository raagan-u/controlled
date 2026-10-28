CREATE TABLE oauth_identities (
    provider text NOT NULL CHECK (provider IN ('google')),
    subject text NOT NULL CHECK (btrim(subject) <> ''),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email_at_link text NOT NULL CHECK (email_at_link = lower(btrim(email_at_link))),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject),
    UNIQUE (provider, user_id)
);

