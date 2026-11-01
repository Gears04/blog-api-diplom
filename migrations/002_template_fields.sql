-- Keep existing users and their posts when upgrading the earlier project.
ALTER TABLE users ADD COLUMN IF NOT EXISTS username VARCHAR(50);
UPDATE users SET username = 'legacy_user_' || id WHERE username IS NULL;
ALTER TABLE users ALTER COLUMN username SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);

ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ;
UPDATE users SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE users ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE users ALTER COLUMN updated_at SET NOT NULL;

ALTER TABLE comments ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ;
UPDATE comments SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE comments ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE comments ALTER COLUMN updated_at SET NOT NULL;
