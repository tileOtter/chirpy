-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email)
VALUES (
    ngen_random_uuid(),
    NOW(),
    NOW(),
    $1
)
RETURNING *;