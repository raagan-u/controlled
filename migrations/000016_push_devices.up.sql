CREATE TABLE push_devices (
    token text PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform text NOT NULL CHECK (platform IN ('android', 'ios')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE INDEX push_devices_user_id_idx ON push_devices(user_id);
