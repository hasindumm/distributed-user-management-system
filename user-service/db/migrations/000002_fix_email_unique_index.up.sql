-- Remove the old unique index that incorrectly enforces uniqueness
-- across ALL rows including soft-deleted users.
-- This caused ALREADY_EXISTS errors when trying to recreate a user
-- with the same email after soft-deleting them.
DROP INDEX IF EXISTS idx_users_email;

-- Create a partial unique index that only enforces uniqueness
-- on NON-deleted users (deleted_at IS NULL).
-- This allows a deleted user's email to be reused
-- while still preventing duplicate active accounts.
CREATE UNIQUE INDEX idx_users_email
    ON users(email)
    WHERE deleted_at IS NULL;
