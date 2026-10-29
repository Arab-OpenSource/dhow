-- name: IsMemberByEmail :one
SELECT EXISTS (
    SELECT 1
    FROM "Memberships" m
    JOIN "Users" u ON u."Id" = m."UserId" AND NOT u."IsDeleted"
    WHERE m."OrganizationId" = @organization_id AND u."Email" = @email AND NOT m."IsDeleted"
);

-- name: RetireExpiredInvitations :exec
UPDATE "Invitations"
SET "RevokedAt" = @now::timestamptz, "UpdatedAt" = @now
WHERE "OrganizationId" = @organization_id AND "Email" = @email AND "AcceptedAt" IS NULL AND "RevokedAt" IS NULL
  AND NOT "IsDeleted" AND "ExpiresAt" <= @now;

-- name: InsertInvitation :one
INSERT INTO "Invitations" ("OrganizationId", "Email", "Role", "TokenHash", "InvitedByUserId", "ExpiresAt", "SentAt",
                           "EmailStatus", "CreatedAt", "UpdatedAt")
VALUES (@organization_id, @email, @role, @token_hash, @invited_by_user_id, @expires_at, @now, @email_status, @now, @now)
RETURNING "Id";

-- name: SetInvitationEmailStatus :exec
UPDATE "Invitations"
SET "EmailStatus" = @email_status, "UpdatedAt" = @now
WHERE "Id" = @id;

-- name: ListOpenInvitations :many
SELECT i."Id", i."Email", i."Role", i."CreatedAt", i."ExpiresAt", i."SentAt", i."SendCount", i."EmailStatus",
       u."Name" AS "InvitedByName"
FROM "Invitations" i
JOIN "Users" u ON u."Id" = i."InvitedByUserId"
WHERE i."OrganizationId" = @organization_id AND i."AcceptedAt" IS NULL AND i."RevokedAt" IS NULL AND NOT i."IsDeleted"
ORDER BY i."CreatedAt" DESC, i."Id" DESC
LIMIT @max_rows;

-- name: ListAcceptedInvitations :many
SELECT i."Id", i."Email", i."Role", i."CreatedAt", i."ExpiresAt", i."SentAt", i."SendCount", i."EmailStatus",
       u."Name" AS "InvitedByName", i."AcceptedAt"::timestamptz AS "AcceptedAt", i."AcceptedByUserId"::uuid AS "AcceptedByUserId",
       COALESCE(a."Name", '') AS "AcceptedByName"
FROM "Invitations" i
JOIN "Users" u ON u."Id" = i."InvitedByUserId"
LEFT JOIN "Users" a ON a."Id" = i."AcceptedByUserId" AND NOT a."IsDeleted"
WHERE i."OrganizationId" = @organization_id AND i."AcceptedAt" IS NOT NULL AND NOT i."IsDeleted"
ORDER BY i."AcceptedAt" DESC, i."Id" DESC
LIMIT @max_rows;

-- name: GetOpenInvitation :one
SELECT i."Id", i."Email", i."Role", i."CreatedAt", i."ExpiresAt", i."SentAt", i."SendCount", i."EmailStatus",
       u."Name" AS "InvitedByName"
FROM "Invitations" i
JOIN "Users" u ON u."Id" = i."InvitedByUserId"
WHERE i."Id" = @id AND i."OrganizationId" = @organization_id AND i."AcceptedAt" IS NULL AND i."RevokedAt" IS NULL
  AND NOT i."IsDeleted";

-- name: LockOpenInvitation :one
SELECT "Id", "Role", "SendCount"
FROM "Invitations"
WHERE "Id" = @id AND "OrganizationId" = @organization_id AND "AcceptedAt" IS NULL AND "RevokedAt" IS NULL
  AND NOT "IsDeleted"
FOR UPDATE;

-- name: RevokeInvitation :execrows
UPDATE "Invitations"
SET "RevokedAt" = @now::timestamptz, "UpdatedAt" = @now
WHERE "Id" = @id AND "OrganizationId" = @organization_id AND "AcceptedAt" IS NULL AND "RevokedAt" IS NULL;

-- name: ReissueInvitation :execrows
UPDATE "Invitations"
SET "TokenHash" = @token_hash, "ExpiresAt" = @expires_at, "SentAt" = @now, "SendCount" = "SendCount" + 1,
    "EmailStatus" = @email_status, "InvitedByUserId" = @invited_by_user_id, "UpdatedAt" = @now
WHERE "Id" = @id AND "OrganizationId" = @organization_id AND "AcceptedAt" IS NULL AND "RevokedAt" IS NULL;

-- name: FindUsableInvitation :one
SELECT i."Id", i."OrganizationId", i."Email", i."Role", i."ExpiresAt", i."InvitedByUserId",
       o."Name" AS "OrganizationName", o."Slug" AS "OrganizationSlug", o."CreatedAt" AS "OrganizationCreatedAt",
       u."Name" AS "InviterName",
       EXISTS (SELECT 1 FROM "Users" x WHERE x."Email" = i."Email" AND NOT x."IsDeleted") AS "AccountExists"
FROM "Invitations" i
JOIN "Organizations" o ON o."Id" = i."OrganizationId" AND NOT o."IsDeleted"
JOIN "Users" u ON u."Id" = i."InvitedByUserId"
WHERE i."TokenHash" = @token_hash AND i."AcceptedAt" IS NULL AND i."RevokedAt" IS NULL AND NOT i."IsDeleted"
  AND i."ExpiresAt" > @now;

-- name: LockUsableInvitation :one
SELECT i."Id", i."OrganizationId", i."Email", i."Role", i."InvitedByUserId"
FROM "Invitations" i
WHERE i."TokenHash" = @token_hash AND i."AcceptedAt" IS NULL AND i."RevokedAt" IS NULL AND NOT i."IsDeleted"
  AND i."ExpiresAt" > @now
FOR UPDATE;

-- name: MarkInvitationAccepted :execrows
UPDATE "Invitations"
SET "AcceptedAt" = @now::timestamptz, "AcceptedByUserId" = @user_id, "UpdatedAt" = @now
WHERE "Id" = @id AND "AcceptedAt" IS NULL AND "RevokedAt" IS NULL;

-- name: UserExistsByEmail :one
SELECT EXISTS (SELECT 1 FROM "Users" WHERE "Email" = @email AND NOT "IsDeleted");
