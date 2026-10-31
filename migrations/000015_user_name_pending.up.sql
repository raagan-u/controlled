-- Marks users whose display name is a placeholder that they still need to choose.
-- Existing users already have a name they picked, so the default is false.
ALTER TABLE users ADD COLUMN name_pending boolean NOT NULL DEFAULT false;
