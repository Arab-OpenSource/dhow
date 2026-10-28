-- name: LockOrganization :one
SELECT "Id", "Name", "Slug", "CreatedAt"
FROM "Organizations"
WHERE "Id" = @id AND NOT "IsDeleted"
FOR UPDATE;

-- name: GetMember :one
SELECT m."UserId", m."Role", m."CreatedAt" AS "JoinedAt", u."Email", u."Name", u."Disabled", u."IsInstanceOwner",
       ARRAY(SELECT r."Role" FROM "OrganizationTeamRoles" r
             WHERE r."UserId" = m."UserId" AND r."OrganizationId" = m."OrganizationId")::text[] AS "TeamRoles"
FROM "Memberships" m
JOIN "Users" u ON u."Id" = m."UserId" AND NOT u."IsDeleted"
WHERE m."OrganizationId" = @organization_id AND m."UserId" = @user_id AND NOT m."IsDeleted";

-- name: ListMemberRoles :many
SELECT m."UserId", m."Role",
       ARRAY(SELECT r."Role" FROM "OrganizationTeamRoles" r
             WHERE r."UserId" = m."UserId" AND r."OrganizationId" = m."OrganizationId")::text[] AS "TeamRoles"
FROM "Memberships" m
WHERE m."OrganizationId" = @organization_id AND m."UserId" = ANY(@user_ids::uuid[]) AND NOT m."IsDeleted";

-- name: CountOtherActiveOwners :one
SELECT count(*)
FROM "Memberships" m
JOIN "Users" u ON u."Id" = m."UserId" AND NOT u."IsDeleted" AND NOT u."Disabled"
WHERE m."OrganizationId" = @organization_id AND m."Role" = 'owner' AND NOT m."IsDeleted" AND m."UserId" <> @user_id;

-- name: SetMemberRole :execrows
UPDATE "Memberships"
SET "Role" = @role, "UpdatedAt" = @now
WHERE "OrganizationId" = @organization_id AND "UserId" = @user_id AND NOT "IsDeleted";

-- name: RemoveMembership :execrows
UPDATE "Memberships"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "OrganizationId" = @organization_id AND "UserId" = @user_id AND NOT "IsDeleted";

-- name: RemoveMemberTeams :exec
UPDATE "TeamMembers"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "OrganizationId" = @organization_id AND "UserId" = @user_id AND NOT "IsDeleted";

-- name: RevokeAllUserSessions :execrows
UPDATE "Sessions"
SET "RevokedAt" = @now::timestamptz, "UpdatedAt" = @now
WHERE "UserId" = @user_id AND "RevokedAt" IS NULL AND NOT "IsDeleted";

-- name: RevokeUserSessionsExcept :execrows
UPDATE "Sessions"
SET "RevokedAt" = @now::timestamptz, "UpdatedAt" = @now
WHERE "UserId" = @user_id AND "Id" <> @except_id AND "RevokedAt" IS NULL AND NOT "IsDeleted";

-- name: ListUserRoles :many
SELECT m."OrganizationId", m."Role", u."IsInstanceOwner",
       ARRAY(SELECT r."Role" FROM "OrganizationTeamRoles" r
             WHERE r."UserId" = m."UserId" AND r."OrganizationId" = m."OrganizationId")::text[] AS "TeamRoles"
FROM "Memberships" m
JOIN "Organizations" o ON o."Id" = m."OrganizationId" AND NOT o."IsDeleted"
JOIN "Users" u ON u."Id" = m."UserId" AND NOT u."IsDeleted"
WHERE m."UserId" = @user_id AND NOT m."IsDeleted";

-- name: GetUserMfaState :one
SELECT EXISTS (SELECT 1 FROM "TotpFactors" t
               WHERE t."UserId" = u."Id" AND t."ConfirmedAt" IS NOT NULL AND NOT t."IsDeleted") AS "HasTotp",
       EXISTS (SELECT 1 FROM "Passkeys" p WHERE p."UserId" = u."Id" AND NOT p."IsDeleted") AS "HasPasskey",
       EXISTS (SELECT 1 FROM "RecoveryCodes" c WHERE c."UserId" = u."Id" AND c."UsedAt" IS NULL AND NOT c."IsDeleted") AS "HasRecoveryCodes"
FROM "Users" u
WHERE u."Id" = @id AND NOT u."IsDeleted";

-- name: RetireUserTotp :execrows
UPDATE "TotpFactors"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "UserId" = @user_id AND NOT "IsDeleted";

-- name: RetireUserPasskeys :execrows
UPDATE "Passkeys"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "UserId" = @user_id AND NOT "IsDeleted";

-- name: RetireUserRecoveryCodes :execrows
UPDATE "RecoveryCodes"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "UserId" = @user_id AND NOT "IsDeleted";

-- name: ExpireUserAuthChallenges :exec
UPDATE "AuthChallenges"
SET "UsedAt" = @now::timestamptz, "UpdatedAt" = @now
WHERE "UserId" = @user_id AND "UsedAt" IS NULL AND NOT "IsDeleted";

-- name: MarkMfaReset :exec
UPDATE "Users"
SET "MfaResetAt" = @now::timestamptz, "UpdatedAt" = @now
WHERE "Id" = @id AND NOT "IsDeleted";

-- name: GetOrganizationByID :one
SELECT "Id", "Name", "Slug", "CreatedAt"
FROM "Organizations"
WHERE "Id" = @id AND NOT "IsDeleted";

-- name: GetUserByID :one
SELECT "Id", "Email", "Name", "Locale"
FROM "Users"
WHERE "Id" = @id AND NOT "IsDeleted";

-- name: RevokeUserApiTokens :execrows
UPDATE "ApiTokens"
SET "RevokedAt" = @now::timestamptz, "RevokedByUserId" = @actor_id, "UpdatedAt" = @now
WHERE "UserId" = @user_id AND NOT "IsDeleted" AND "RevokedAt" IS NULL;
