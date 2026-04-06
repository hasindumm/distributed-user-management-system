-- Revert to the original index that enforces uniqueness
-- across all rows including soft-deleted users.
DROP INDEX IF EXISTS idx_users_email;

CREATE UNIQUE INDEX idx_users_email ON users(email);
