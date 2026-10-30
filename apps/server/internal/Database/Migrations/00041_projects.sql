-- +goose Up
CREATE TABLE "Projects" (
    "Id"              uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"       timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"       timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"       boolean NOT NULL DEFAULT false,
    "OrganizationId"  uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "Slug"            TEXT NOT NULL CHECK ("Slug" ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length("Slug") <= 63),
    "Name"            TEXT NOT NULL CHECK (char_length("Name") BETWEEN 1 AND 100),
    "PipelinePath"    TEXT NOT NULL DEFAULT '.dhow.yml' CHECK (char_length("PipelinePath") BETWEEN 1 AND 255),
    "ForkPolicy"      TEXT NOT NULL DEFAULT 'approval' CHECK ("ForkPolicy" IN ('approval', 'automatic')),
    "State"           TEXT NOT NULL DEFAULT 'active' CHECK ("State" IN ('active', 'archived')),
    "ArchivedAt"      timestamptz,
    "CreatedByUserId" uuid NOT NULL REFERENCES "Users" ("Id"),
    CHECK (("State" = 'archived') = ("ArchivedAt" IS NOT NULL))
);

CREATE UNIQUE INDEX "UX_Projects_Organization_Slug" ON "Projects" ("OrganizationId", "Slug") WHERE NOT "IsDeleted";
CREATE UNIQUE INDEX "UX_Projects_Id_Organization" ON "Projects" ("Id", "OrganizationId");
CREATE INDEX "IX_Projects_Organization_Name" ON "Projects" ("OrganizationId", lower("Name"), "Id") WHERE NOT "IsDeleted";
CREATE INDEX "IX_RoleBindings_Project" ON "RoleBindings" ("ProjectId") WHERE "ProjectId" IS NOT NULL AND NOT "IsDeleted";

ALTER TABLE "Jobs"
    ADD COLUMN "OriginKind" TEXT NOT NULL DEFAULT 'manual' CHECK ("OriginKind" IN ('manual', 'pipeline')),
    ADD COLUMN "OriginId" uuid,
    ADD COLUMN "RequiredFeatures" TEXT[] NOT NULL DEFAULT '{}' CHECK (
        cardinality("RequiredFeatures") <= 8
        AND "RequiredFeatures" <@ ARRAY['checkout', 'plugins', 'artifacts', 'cache']::text[]
    ),
    ADD CONSTRAINT "CK_Jobs_Origin" CHECK (("OriginKind" = 'manual') = ("OriginId" IS NULL));

CREATE INDEX "IX_Jobs_Origin" ON "Jobs" ("OriginId", "CreatedAt" DESC) WHERE "OriginId" IS NOT NULL AND NOT "IsDeleted";

ALTER TABLE "ApiTokens" DROP CONSTRAINT "ApiTokens_Scopes_check";
ALTER TABLE "ApiTokens" ADD CONSTRAINT "ApiTokens_Scopes_check" CHECK (
    cardinality("Scopes") BETWEEN 1 AND 16
    AND "Scopes" <@ ARRAY['org.read', 'org.update', 'member.read', 'member.manage', 'audit.read', 'runner.read', 'runner.manage',
                          'job.read', 'job.cancel', 'project.read', 'project.manage']::text[]
);

-- +goose Down
UPDATE "ApiTokens" SET "Scopes" = array_remove(array_remove("Scopes", 'project.read'), 'project.manage');
UPDATE "ApiTokens" SET "IsDeleted" = true, "Scopes" = ARRAY['org.read']::text[] WHERE cardinality("Scopes") = 0;
ALTER TABLE "ApiTokens" DROP CONSTRAINT "ApiTokens_Scopes_check";
ALTER TABLE "ApiTokens" ADD CONSTRAINT "ApiTokens_Scopes_check" CHECK (
    cardinality("Scopes") BETWEEN 1 AND 16
    AND "Scopes" <@ ARRAY['org.read', 'org.update', 'member.read', 'member.manage', 'audit.read', 'runner.read', 'runner.manage', 'job.read', 'job.cancel']::text[]
);
DROP INDEX "IX_Jobs_Origin";
ALTER TABLE "Jobs" DROP CONSTRAINT "CK_Jobs_Origin";
ALTER TABLE "Jobs" DROP COLUMN "RequiredFeatures";
ALTER TABLE "Jobs" DROP COLUMN "OriginId";
ALTER TABLE "Jobs" DROP COLUMN "OriginKind";
DROP INDEX "IX_RoleBindings_Project";
DROP TABLE "Projects";
