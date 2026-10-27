-- name: GetInstance :one
SELECT id, created_at, setup_completed_at
FROM instance
WHERE id = 1;
