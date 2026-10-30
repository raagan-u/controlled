DROP TRIGGER IF EXISTS users_assign_ctrld_id ON users;
DROP FUNCTION IF EXISTS assign_ctrld_id();
DROP INDEX IF EXISTS users_ctrld_id_unique;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_ctrld_id_format;
ALTER TABLE users DROP COLUMN IF EXISTS ctrld_id;
