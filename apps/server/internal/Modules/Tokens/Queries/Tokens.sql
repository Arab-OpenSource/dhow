-- name: GetTokenMembership :one
SELECT m."Id", m."Role",
       ARRAY(SELECT r."Role" FROM "OrganizationTeamRoles" r
             WHERE r."UserId" = m."UserId" AND r."OrganizationId" = m."OrganizationId")::text[] AS "TeamRoles"
FROM "Memberships" m
JOIN "Organizations" o ON o."Id" = m."OrganizationId" AND NOT o."IsDeleted"
JOIN "Users" u ON u."Id" = m."UserId" AND NOT u."IsDeleted" AND NOT u."Disabled"
WHERE m."UserId" = @user_id AND m."OrganizationId" = @organization_id AND NOT m."IsDeleted";

-- name: CountActiveUserTokens :one
SELECT count(*)
FROM "ApiTokens"
WHERE "UserId" = @user_id AND NOT "IsDeleted" AND "RevokedAt" IS NULL AND "ExpiresAt" > @now;

-- name: InsertApiToken :one
INSERT INTO "ApiTokens" ("UserId", "OrganizationId", "MembershipId", "Name", "TokenHash", "Scopes", "ExpiresAt", "CreatedAt",
                         "UpdatedAt")
VALUES (@user_id, @organization_id, @membership_id, @name, @token_hash, @scopes::text[], @expires_at, @now, @now)
RETURNING "Id", "CreatedAt";

-- name: ResolveApiToken :one
SELECT t."Id", t."UserId", t."OrganizationId", t."Scopes", t."ExpiresAt", t."LastUsedAt",
       u."Email", u."Name", u."Locale", m."Role",
       ARRAY(SELECT r."Role" FROM "OrganizationTeamRoles" r
             WHERE r."UserId" = m."UserId" AND r."OrganizationId" = m."OrganizationId")::text[] AS "TeamRoles"
FROM "ApiTokens" t
JOIN "Users" u ON u."Id" = t."UserId" AND NOT u."IsDeleted" AND NOT u."Disabled"
JOIN "Memberships" m ON m."Id" = t."MembershipId" AND m."UserId" = t."UserId" AND m."OrganizationId" = t."OrganizationId"
                     AND NOT m."IsDeleted"
JOIN "Organizations" o ON o."Id" = t."OrganizationId" AND NOT o."IsDeleted"
WHERE t."TokenHash" = @token_hash AND NOT t."IsDeleted" AND t."RevokedAt" IS NULL AND t."ExpiresAt" > @now;

-- name: TouchApiToken :exec
UPDATE "ApiTokens"
SET "LastUsedAt" = @now::timestamptz, "LastUsedIp" = @ip, "UpdatedAt" = @now
WHERE "Id" = @id AND ("LastUsedAt" IS NULL OR "LastUsedAt" <= @touch_before::timestamptz);

-- name: ListApiTokens :many
SELECT t."Id", t."OrganizationId", o."Name" AS "OrganizationName", t."Name", t."Scopes", t."CreatedAt", t."ExpiresAt",
       t."RevokedAt", t."LastUsedAt", t."LastUsedIp", t."UserId", u."Name" AS "UserName", u."Email" AS "UserEmail",
       (m."Id" IS NOT NULL)::boolean AS "MembershipActive"
FROM "ApiTokens" t
JOIN "Organizations" o ON o."Id" = t."OrganizationId"
JOIN "Users" u ON u."Id" = t."UserId"
LEFT JOIN "Memberships" m ON m."Id" = t."MembershipId" AND NOT m."IsDeleted"
WHERE NOT t."IsDeleted"
  AND (sqlc.narg(user_id)::uuid IS NULL OR t."UserId" = sqlc.narg(user_id)::uuid)
  AND (sqlc.narg(organization_id)::uuid IS NULL OR t."OrganizationId" = sqlc.narg(organization_id)::uuid)
  AND (sqlc.narg(id)::uuid IS NULL OR t."Id" = sqlc.narg(id)::uuid)
ORDER BY t."CreatedAt" DESC, t."Id" DESC
LIMIT @max_rows;

-- name: LockUserForTokens :one
SELECT "Id"
FROM "Users"
WHERE "Id" = @id AND NOT "IsDeleted" AND NOT "Disabled"
FOR UPDATE;

-- name: RevokeApiToken :one
UPDATE "ApiTokens"
SET "RevokedAt" = @now::timestamptz, "RevokedByUserId" = @actor_id, "UpdatedAt" = @now
WHERE "Id" = @id AND NOT "IsDeleted" AND "RevokedAt" IS NULL
  AND (sqlc.narg(user_id)::uuid IS NULL OR "UserId" = sqlc.narg(user_id)::uuid)
  AND (sqlc.narg(organization_id)::uuid IS NULL OR "OrganizationId" = sqlc.narg(organization_id)::uuid)
RETURNING "Id", "UserId", "OrganizationId", "Name";
