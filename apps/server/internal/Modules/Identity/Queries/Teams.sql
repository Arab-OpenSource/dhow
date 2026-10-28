-- name: ListTeams :many
SELECT t."Id", t."Name", t."Description", t."CreatedAt",
       (SELECT count(*)
        FROM "TeamMembers" tm
        JOIN "Memberships" m ON m."UserId" = tm."UserId" AND m."OrganizationId" = t."OrganizationId" AND NOT m."IsDeleted"
        WHERE tm."TeamId" = t."Id" AND NOT tm."IsDeleted")::bigint AS "MemberCount"
FROM "Teams" t
WHERE t."OrganizationId" = @organization_id AND NOT t."IsDeleted"
ORDER BY lower(t."Name"), t."Id";

-- name: ListOrganizationBindings :many
SELECT rb."Id", rb."TeamId", rb."Scope", rb."Role", rb."CreatedAt"
FROM "RoleBindings" rb
JOIN "Teams" t ON t."Id" = rb."TeamId" AND NOT t."IsDeleted"
WHERE rb."OrganizationId" = @organization_id AND NOT rb."IsDeleted"
ORDER BY rb."CreatedAt", rb."Id";

-- name: GetTeam :one
SELECT t."Id", t."Name", t."Description", t."CreatedAt"
FROM "Teams" t
WHERE t."Id" = @id AND t."OrganizationId" = @organization_id AND NOT t."IsDeleted";

-- name: LockTeam :one
SELECT t."Id", t."Name", t."Description", t."CreatedAt"
FROM "Teams" t
WHERE t."Id" = @id AND t."OrganizationId" = @organization_id AND NOT t."IsDeleted"
FOR UPDATE;

-- name: ListTeamBindings :many
SELECT "Id", "TeamId", "Scope", "Role", "CreatedAt"
FROM "RoleBindings"
WHERE "TeamId" = @team_id AND "OrganizationId" = @organization_id AND NOT "IsDeleted"
ORDER BY "CreatedAt", "Id";

-- name: ListTeamMembers :many
SELECT u."Id" AS "UserId", u."Name", u."Email", m."Role", tm."CreatedAt" AS "AddedAt"
FROM "TeamMembers" tm
JOIN "Memberships" m ON m."UserId" = tm."UserId" AND m."OrganizationId" = tm."OrganizationId" AND NOT m."IsDeleted"
JOIN "Users" u ON u."Id" = tm."UserId" AND NOT u."IsDeleted"
WHERE tm."TeamId" = @team_id AND tm."OrganizationId" = @organization_id AND NOT tm."IsDeleted"
ORDER BY lower(u."Name"), u."Id";

-- name: InsertTeam :one
INSERT INTO "Teams" ("OrganizationId", "Name", "Description", "CreatedAt", "UpdatedAt")
VALUES (@organization_id, @name, @description, @now, @now)
RETURNING "Id", "Name", "Description", "CreatedAt";

-- name: UpdateTeam :one
UPDATE "Teams"
SET "Name" = @name, "Description" = @description, "UpdatedAt" = @now
WHERE "Id" = @id AND "OrganizationId" = @organization_id AND NOT "IsDeleted"
RETURNING "Id", "Name", "Description", "CreatedAt";

-- name: DeleteTeam :execrows
UPDATE "Teams"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "Id" = @id AND "OrganizationId" = @organization_id AND NOT "IsDeleted";

-- name: RemoveAllTeamMembers :exec
UPDATE "TeamMembers"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "TeamId" = @team_id AND "OrganizationId" = @organization_id AND NOT "IsDeleted";

-- name: RemoveAllTeamBindings :exec
UPDATE "RoleBindings"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "TeamId" = @team_id AND "OrganizationId" = @organization_id AND NOT "IsDeleted";

-- name: InsertTeamMember :exec
INSERT INTO "TeamMembers" ("OrganizationId", "TeamId", "UserId", "CreatedAt", "UpdatedAt")
VALUES (@organization_id, @team_id, @user_id, @now, @now);

-- name: RemoveTeamMember :execrows
UPDATE "TeamMembers"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "TeamId" = @team_id AND "OrganizationId" = @organization_id AND "UserId" = @user_id AND NOT "IsDeleted";

-- name: InsertTeamBinding :one
INSERT INTO "RoleBindings" ("OrganizationId", "TeamId", "Scope", "Role", "CreatedAt", "UpdatedAt")
VALUES (@organization_id, @team_id, @scope, @role, @now, @now)
RETURNING "Id";

-- name: LockTeamBinding :one
SELECT "Id", "Role"
FROM "RoleBindings"
WHERE "Id" = @id AND "TeamId" = @team_id AND "OrganizationId" = @organization_id AND NOT "IsDeleted"
FOR UPDATE;

-- name: SetTeamBindingRole :execrows
UPDATE "RoleBindings"
SET "Role" = @role, "UpdatedAt" = @now
WHERE "Id" = @id AND "OrganizationId" = @organization_id AND NOT "IsDeleted";

-- name: DeleteTeamBinding :execrows
UPDATE "RoleBindings"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "Id" = @id AND "OrganizationId" = @organization_id AND NOT "IsDeleted";
