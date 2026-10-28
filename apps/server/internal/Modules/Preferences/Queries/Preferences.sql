-- name: GetUserLocale :one
SELECT "Locale"
FROM "Users"
WHERE "Id" = @id AND NOT "IsDeleted";

-- name: GetUserPreferences :one
SELECT "ThemeId", "CustomThemes", "OrganizationId", "UpdatedAt"
FROM "UserPreferences"
WHERE "UserId" = @user_id AND NOT "IsDeleted";

-- name: IsActiveMember :one
SELECT EXISTS (
    SELECT 1 FROM "Memberships" m
    JOIN "Organizations" o ON o."Id" = m."OrganizationId" AND NOT o."IsDeleted"
    WHERE m."UserId" = @user_id AND m."OrganizationId" = @organization_id AND NOT m."IsDeleted"
) AS "Member";

-- name: UpsertUserPreferences :one
INSERT INTO "UserPreferences" ("UserId", "ThemeId", "CustomThemes", "OrganizationId", "CreatedAt", "UpdatedAt")
VALUES (@user_id, @theme_id, @custom_themes, sqlc.narg(organization_id), @now, @now)
ON CONFLICT ("UserId") WHERE NOT "IsDeleted"
DO UPDATE SET "ThemeId" = EXCLUDED."ThemeId", "CustomThemes" = EXCLUDED."CustomThemes",
              "OrganizationId" = EXCLUDED."OrganizationId", "UpdatedAt" = EXCLUDED."UpdatedAt"
RETURNING "UpdatedAt";

-- name: SetUserLocale :exec
UPDATE "Users"
SET "Locale" = @locale, "UpdatedAt" = @now
WHERE "Id" = @id AND NOT "IsDeleted" AND "Locale" <> @locale;
