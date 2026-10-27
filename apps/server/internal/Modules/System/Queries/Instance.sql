-- name: GetInstance :one
SELECT "Id", "CreatedAt", "UpdatedAt", "SetupCompletedAt"
FROM "Instance"
WHERE "Id" = 1;
