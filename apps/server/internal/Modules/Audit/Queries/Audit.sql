-- name: LockAuditChain :exec
SELECT pg_advisory_xact_lock(hashtextextended(@chain_key::text, 0));

-- name: GetLastAuditHash :one
SELECT "Hash"
FROM "AuditEvents"
WHERE ("OrganizationId" = sqlc.narg(organization_id)::uuid OR (sqlc.narg(organization_id)::uuid IS NULL AND "OrganizationId" IS NULL))
ORDER BY "Seq" DESC
LIMIT 1;

-- name: NextAuditId :one
SELECT uuidv7()::uuid AS "Id";

-- name: InsertAuditEvent :exec
INSERT INTO "AuditEvents" ("Id", "CreatedAt", "UpdatedAt", "OrganizationId", "ActorUserId", "Action", "TargetType",
                           "TargetId", "Ip", "Data", "PrevHash", "Hash")
VALUES (@id, @created_at, @created_at, sqlc.narg(organization_id), sqlc.narg(actor_user_id), @action, @target_type, @target_id, @ip,
        @data, @prev_hash, @hash);

-- name: ListAuditEvents :many
SELECT "Id", "CreatedAt", "OrganizationId", "ActorUserId", "Action", "TargetType", "TargetId", "Ip", "Data"
FROM "AuditEvents"
WHERE "OrganizationId" = @organization_id
ORDER BY "Seq" DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: CountAuditEvents :one
SELECT count(*)
FROM "AuditEvents"
WHERE "OrganizationId" = @organization_id;

-- name: WalkAuditChain :many
SELECT "Id", "Seq", "CreatedAt", "OrganizationId", "ActorUserId", "Action", "TargetType", "TargetId", "Ip", "Data",
       "PrevHash", "Hash"
FROM "AuditEvents"
WHERE ("OrganizationId" = sqlc.narg(organization_id)::uuid OR (sqlc.narg(organization_id)::uuid IS NULL AND "OrganizationId" IS NULL))
  AND "Seq" > @after_seq
ORDER BY "Seq"
LIMIT @page_limit;
