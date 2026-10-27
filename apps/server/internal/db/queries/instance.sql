-- name: GetInstance :one
SELECT "Id", "CreatedAt", "SetupCompletedAt"
FROM "Instance"
WHERE "Id" = 1;
