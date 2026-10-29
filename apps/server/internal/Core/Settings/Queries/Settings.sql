-- name: ListInstanceSettings :many
SELECT "Group", "Value", "Secrets", "Version", "UpdatedAt", "UpdatedBy"
FROM "InstanceSettings"
WHERE NOT "IsDeleted"
ORDER BY "Group";

-- name: LockInstanceSetting :one
SELECT "Id", "Group", "Value", "Secrets", "Version"
FROM "InstanceSettings"
WHERE "Group" = @group_name AND NOT "IsDeleted"
FOR UPDATE;

-- name: InsertInstanceSetting :one
INSERT INTO "InstanceSettings" ("Group", "Value", "Secrets", "Version", "UpdatedBy")
VALUES (@group_name, @value, @secrets, 1, @updated_by)
ON CONFLICT ("Group") WHERE NOT "IsDeleted" DO NOTHING
RETURNING "Version";

-- name: UpdateInstanceSetting :one
UPDATE "InstanceSettings"
SET "Value" = @value, "Secrets" = @secrets, "Version" = "Version" + 1, "UpdatedBy" = @updated_by, "UpdatedAt" = now()
WHERE "Id" = @id AND "Version" = @version AND NOT "IsDeleted"
RETURNING "Version";

-- name: NotifySettingsChanged :exec
SELECT pg_notify('dhow_settings_changed', @group_name::text);

-- name: ListenSettingsChanged :exec
LISTEN dhow_settings_changed;
