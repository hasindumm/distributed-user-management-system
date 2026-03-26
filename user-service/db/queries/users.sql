-- name: CreateUser :one
INSERT INTO users (
    user_id,
    first_name,
    last_name,
    email,
    phone,
    age,
    status
) VALUES (
             gen_random_uuid(),
             $1, $2, $3, $4, $5, $6
         )
    RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE user_id = $1
  AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1
  AND deleted_at IS NULL;

-- name: ListUsers :many
SELECT * FROM users
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::VARCHAR IS NULL OR status = sqlc.narg('status')::VARCHAR)
ORDER BY created_at DESC
    LIMIT $1 OFFSET $2;

-- name: UpdateUser :one
UPDATE users
SET
    first_name = $2,
    last_name  = $3,
    email      = $4,
    phone      = $5,
    age        = $6,
    status     = $7,
    updated_at = NOW()
WHERE user_id = $1
  AND deleted_at IS NULL
    RETURNING *;

-- name: DeleteUser :execresult
UPDATE users
SET deleted_at = NOW()
WHERE user_id = $1
  AND deleted_at IS NULL;