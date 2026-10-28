-- +goose Up
ALTER TABLE "Users" ADD COLUMN "IsInstanceOwner" boolean NOT NULL DEFAULT false;
ALTER TABLE "Users" ADD COLUMN "MfaResetAt" timestamptz;

UPDATE "Users" u
SET "IsInstanceOwner" = true
WHERE EXISTS (
    SELECT 1
    FROM "Memberships" m
    WHERE m."UserId" = u."Id" AND m."Role" = 'owner' AND NOT m."IsDeleted"
      AND m."OrganizationId" = (SELECT o."Id" FROM "Organizations" o ORDER BY o."CreatedAt", o."Id" LIMIT 1)
);

CREATE INDEX "IX_Memberships_Organization_Owner" ON "Memberships" ("OrganizationId")
    WHERE "Role" = 'owner' AND NOT "IsDeleted";

CREATE TABLE "Teams" (
    "Id"             uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"      timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"      timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"      boolean NOT NULL DEFAULT false,
    "OrganizationId" uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "Name"           TEXT NOT NULL CHECK (char_length("Name") BETWEEN 1 AND 100),
    "Description"    TEXT NOT NULL DEFAULT '' CHECK (char_length("Description") <= 500)
);

CREATE UNIQUE INDEX "UX_Teams_Organization_Name" ON "Teams" ("OrganizationId", lower("Name")) WHERE NOT "IsDeleted";

CREATE TABLE "TeamMembers" (
    "Id"             uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"      timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"      timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"      boolean NOT NULL DEFAULT false,
    "OrganizationId" uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "TeamId"         uuid NOT NULL REFERENCES "Teams" ("Id"),
    "UserId"         uuid NOT NULL REFERENCES "Users" ("Id")
);

CREATE UNIQUE INDEX "UX_TeamMembers_Team_User" ON "TeamMembers" ("TeamId", "UserId") WHERE NOT "IsDeleted";
CREATE INDEX "IX_TeamMembers_Organization_User" ON "TeamMembers" ("OrganizationId", "UserId") WHERE NOT "IsDeleted";

CREATE TABLE "RoleBindings" (
    "Id"             uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"      timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"      timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"      boolean NOT NULL DEFAULT false,
    "OrganizationId" uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "TeamId"         uuid REFERENCES "Teams" ("Id"),
    "UserId"         uuid REFERENCES "Users" ("Id"),
    "Scope"          TEXT NOT NULL CHECK ("Scope" IN ('org', 'project', 'environment')),
    "ProjectId"      uuid,
    "EnvironmentId"  uuid,
    "Role"           TEXT NOT NULL CHECK ("Role" IN ('admin', 'maintainer', 'developer', 'viewer')),
    CHECK (num_nonnulls("TeamId", "UserId") = 1),
    CHECK (
        ("Scope" = 'org' AND "ProjectId" IS NULL AND "EnvironmentId" IS NULL)
        OR ("Scope" = 'project' AND "ProjectId" IS NOT NULL AND "EnvironmentId" IS NULL)
        OR ("Scope" = 'environment' AND "ProjectId" IS NOT NULL AND "EnvironmentId" IS NOT NULL)
    )
);

CREATE UNIQUE INDEX "UX_RoleBindings_Team_Target" ON "RoleBindings"
    ("TeamId", "Scope", COALESCE("ProjectId", '00000000-0000-0000-0000-000000000000'::uuid),
     COALESCE("EnvironmentId", '00000000-0000-0000-0000-000000000000'::uuid))
    WHERE "TeamId" IS NOT NULL AND NOT "IsDeleted";
CREATE UNIQUE INDEX "UX_RoleBindings_User_Target" ON "RoleBindings"
    ("OrganizationId", "UserId", "Scope", COALESCE("ProjectId", '00000000-0000-0000-0000-000000000000'::uuid),
     COALESCE("EnvironmentId", '00000000-0000-0000-0000-000000000000'::uuid))
    WHERE "UserId" IS NOT NULL AND NOT "IsDeleted";
CREATE INDEX "IX_RoleBindings_Organization" ON "RoleBindings" ("OrganizationId") WHERE NOT "IsDeleted";

CREATE VIEW "OrganizationTeamRoles" AS
SELECT tm."UserId", rb."OrganizationId", rb."Role", rb."TeamId"
FROM "RoleBindings" rb
JOIN "Teams" t ON t."Id" = rb."TeamId" AND t."OrganizationId" = rb."OrganizationId" AND NOT t."IsDeleted"
JOIN "TeamMembers" tm ON tm."TeamId" = rb."TeamId" AND tm."OrganizationId" = rb."OrganizationId" AND NOT tm."IsDeleted"
JOIN "Memberships" m ON m."UserId" = tm."UserId" AND m."OrganizationId" = rb."OrganizationId" AND NOT m."IsDeleted"
WHERE rb."Scope" = 'org' AND NOT rb."IsDeleted";

CREATE TABLE "Invitations" (
    "Id"               uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"        timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"        timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"        boolean NOT NULL DEFAULT false,
    "OrganizationId"   uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "Email"            TEXT NOT NULL CHECK ("Email" = lower("Email") AND char_length("Email") BETWEEN 3 AND 254),
    "Role"             TEXT NOT NULL CHECK ("Role" IN ('admin', 'maintainer', 'developer', 'viewer')),
    "TokenHash"        bytea NOT NULL CHECK (octet_length("TokenHash") = 32),
    "InvitedByUserId"  uuid NOT NULL REFERENCES "Users" ("Id"),
    "ExpiresAt"        timestamptz NOT NULL,
    "SentAt"           timestamptz NOT NULL DEFAULT now(),
    "SendCount"        integer NOT NULL DEFAULT 1 CHECK ("SendCount" >= 1),
    "EmailStatus"      TEXT NOT NULL DEFAULT 'disabled' CHECK ("EmailStatus" IN ('disabled', 'sent', 'failed')),
    "AcceptedAt"       timestamptz,
    "AcceptedByUserId" uuid REFERENCES "Users" ("Id"),
    "RevokedAt"        timestamptz,
    CHECK (("AcceptedAt" IS NULL) = ("AcceptedByUserId" IS NULL))
);

CREATE UNIQUE INDEX "UX_Invitations_TokenHash" ON "Invitations" ("TokenHash");
CREATE UNIQUE INDEX "UX_Invitations_Organization_Email_Pending" ON "Invitations" ("OrganizationId", "Email")
    WHERE "AcceptedAt" IS NULL AND "RevokedAt" IS NULL AND NOT "IsDeleted";
CREATE INDEX "IX_Invitations_Organization" ON "Invitations" ("OrganizationId", "CreatedAt" DESC) WHERE NOT "IsDeleted";

-- +goose Down
DROP TABLE "Invitations";
DROP VIEW "OrganizationTeamRoles";
DROP TABLE "RoleBindings";
DROP TABLE "TeamMembers";
DROP TABLE "Teams";
DROP INDEX "IX_Memberships_Organization_Owner";
ALTER TABLE "Users" DROP COLUMN "MfaResetAt";
ALTER TABLE "Users" DROP COLUMN "IsInstanceOwner";
