-- name: GetAlertRecipient :one
SELECT "Email", "Name", "Locale"
FROM "Users"
WHERE "Id" = @id AND NOT "IsDeleted" AND NOT "Disabled";
