-- name: GetUserForLogin :one
SELECT "Id", "Email", "Name", "Locale", "PasswordHash", "Disabled"
FROM "Users"
WHERE "Email" = @email AND NOT "IsDeleted";

-- name: ListUserMemberships :many
SELECT m."OrganizationId", o."Name" AS "OrganizationName", o."Slug" AS "OrganizationSlug", m."Role",
       ARRAY(SELECT r."Role" FROM "OrganizationTeamRoles" r
             WHERE r."UserId" = m."UserId" AND r."OrganizationId" = m."OrganizationId")::text[] AS "TeamRoles"
FROM "Memberships" m
JOIN "Organizations" o ON o."Id" = m."OrganizationId" AND NOT o."IsDeleted"
WHERE m."UserId" = @user_id AND NOT m."IsDeleted"
ORDER BY m."CreatedAt", m."Id";

-- name: InsertSession :one
INSERT INTO "Sessions" ("UserId", "TokenHash", "CsrfHash", "LastSeenAt", "ExpiresAt", "IdleExpiresAt", "UserAgent",
                        "IpAddress", "CreatedAt", "UpdatedAt")
VALUES (@user_id, @token_hash, @csrf_hash, @now, @expires_at, @idle_expires_at, @user_agent, @ip_address, @now, @now)
RETURNING "Id", "ExpiresAt";

-- name: ResolveSession :one
SELECT s."Id", s."UserId", s."CsrfHash", s."LastSeenAt", s."ExpiresAt", s."IdleExpiresAt",
       u."Email", u."Name", u."Locale", u."IsInstanceOwner"
FROM "Sessions" s
JOIN "Users" u ON u."Id" = s."UserId" AND NOT u."IsDeleted" AND NOT u."Disabled"
WHERE s."TokenHash" = @token_hash AND NOT s."IsDeleted" AND s."RevokedAt" IS NULL
  AND s."ExpiresAt" > @now AND s."IdleExpiresAt" > @now;

-- name: TouchSession :exec
UPDATE "Sessions"
SET "LastSeenAt" = @now, "IdleExpiresAt" = LEAST(@idle_expires_at::timestamptz, "ExpiresAt"), "UpdatedAt" = @now
WHERE "Id" = @id AND "RevokedAt" IS NULL AND "LastSeenAt" <= @touch_before;

-- name: RevokeSessionByTokenHash :execrows
UPDATE "Sessions"
SET "RevokedAt" = @now::timestamptz, "UpdatedAt" = @now
WHERE "TokenHash" = @token_hash AND "RevokedAt" IS NULL;

-- name: RevokeUserSession :execrows
UPDATE "Sessions"
SET "RevokedAt" = @now::timestamptz, "UpdatedAt" = @now
WHERE "Id" = @id AND "UserId" = @user_id AND "RevokedAt" IS NULL;

-- name: RevokeOtherUserSessions :execrows
UPDATE "Sessions"
SET "RevokedAt" = @now::timestamptz, "UpdatedAt" = @now
WHERE "UserId" = @user_id AND "Id" <> @except_id AND "RevokedAt" IS NULL;

-- name: ListActiveUserSessions :many
SELECT "Id", "CreatedAt", "LastSeenAt", "ExpiresAt", "UserAgent", "IpAddress"
FROM "Sessions"
WHERE "UserId" = @user_id AND NOT "IsDeleted" AND "RevokedAt" IS NULL AND "ExpiresAt" > @now AND "IdleExpiresAt" > @now
ORDER BY "CreatedAt" DESC, "Id" DESC;

-- name: GetUserPasswordHash :one
SELECT "PasswordHash"
FROM "Users"
WHERE "Id" = @id AND NOT "IsDeleted" AND NOT "Disabled";

-- name: SetUserPassword :exec
UPDATE "Users"
SET "PasswordHash" = @password_hash, "PasswordChangedAt" = @now, "UpdatedAt" = @now
WHERE "Id" = @id AND NOT "IsDeleted";

-- name: RehashUserPassword :exec
UPDATE "Users"
SET "PasswordHash" = @password_hash, "UpdatedAt" = @now
WHERE "Id" = @id AND NOT "IsDeleted" AND "PasswordHash" = @old_hash;

-- name: EnsureThrottle :exec
INSERT INTO "LoginThrottles" ("Key", "Failures", "LastFailureAt", "CreatedAt", "UpdatedAt")
VALUES (@key, 0, @now, @now, @now)
ON CONFLICT ("Key") DO NOTHING;

-- name: LockThrottle :one
SELECT "Failures", "LastFailureAt", "LockedUntil"
FROM "LoginThrottles"
WHERE "Key" = @key
FOR UPDATE;

-- name: UpdateThrottle :exec
UPDATE "LoginThrottles"
SET "Failures" = @failures, "LastFailureAt" = @now, "LockedUntil" = sqlc.narg(locked_until)::timestamptz, "UpdatedAt" = @now
WHERE "Key" = @key;

-- name: RefundThrottle :exec
UPDATE "LoginThrottles"
SET "Failures" = GREATEST("Failures" - 1, 0),
    "LockedUntil" = CASE WHEN "Failures" - 1 < @free::integer THEN NULL ELSE "LockedUntil" END,
    "UpdatedAt" = @now
WHERE "Key" = @key;

-- name: ResetThrottle :exec
UPDATE "LoginThrottles"
SET "Failures" = 0, "LockedUntil" = NULL, "UpdatedAt" = @now
WHERE "Key" = @key;

-- name: GetUserAccountFlags :one
SELECT "IsInstanceOwner", "MfaResetAt"
FROM "Users"
WHERE "Id" = @id AND NOT "IsDeleted";
