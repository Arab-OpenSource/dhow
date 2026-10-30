-- +goose Up
CREATE TABLE "GitConnections" (
    "Id"              uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"       timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"       timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"       boolean NOT NULL DEFAULT false,
    "OrganizationId"  uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "Provider"        TEXT NOT NULL CHECK ("Provider" IN ('github', 'gitlab', 'generic')),
    "InstallationId"  bigint CHECK ("InstallationId" > 0),
    "BaseUrl"         TEXT CHECK (char_length("BaseUrl") BETWEEN 1 AND 2048),
    "TokenSealed"     bytea CHECK (octet_length("TokenSealed") BETWEEN 1 AND 8192),
    "TokenExpiresAt"  timestamptz,
    "State"           TEXT NOT NULL DEFAULT 'active' CHECK ("State" IN ('active', 'suspended', 'revoked')),
    "CreatedByUserId" uuid NOT NULL REFERENCES "Users" ("Id"),
    CHECK (("Provider" = 'github') = ("InstallationId" IS NOT NULL))
);

CREATE UNIQUE INDEX "UX_GitConnections_Installation" ON "GitConnections" ("InstallationId")
    WHERE "Provider" = 'github' AND NOT "IsDeleted";
CREATE UNIQUE INDEX "UX_GitConnections_Id_Organization" ON "GitConnections" ("Id", "OrganizationId");
CREATE INDEX "IX_GitConnections_Organization" ON "GitConnections" ("OrganizationId") WHERE NOT "IsDeleted";

CREATE TABLE "Repositories" (
    "Id"                         uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"                  timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"                  timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"                  boolean NOT NULL DEFAULT false,
    "OrganizationId"             uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "ProjectId"                  uuid NOT NULL,
    "Provider"                   TEXT NOT NULL CHECK ("Provider" IN ('github', 'gitlab', 'generic')),
    "ConnectionId"               uuid,
    "ExternalId"                 TEXT CHECK (char_length("ExternalId") BETWEEN 1 AND 255),
    "FullName"                   TEXT NOT NULL CHECK (char_length("FullName") BETWEEN 1 AND 2048),
    "CloneUrl"                   TEXT NOT NULL CHECK (char_length("CloneUrl") BETWEEN 1 AND 2048 AND "CloneUrl" ~ '^(https|ssh)://'),
    "Transport"                  TEXT NOT NULL CHECK ("Transport" IN ('https', 'ssh')),
    "DefaultBranch"              TEXT NOT NULL DEFAULT 'main' CHECK (char_length("DefaultBranch") BETWEEN 1 AND 244),
    "HostKey"                    TEXT CHECK (char_length("HostKey") BETWEEN 1 AND 16384),
    "HostKeyConfirmedAt"         timestamptz,
    "HostKeyConfirmedByUserId"   uuid REFERENCES "Users" ("Id"),
    "PendingHostKey"             TEXT CHECK (char_length("PendingHostKey") BETWEEN 1 AND 16384),
    "PendingHostKeyFetchedAt"    timestamptz,
    "DeployKeyPublic"            TEXT CHECK (char_length("DeployKeyPublic") BETWEEN 1 AND 1024),
    "DeployKeySealed"            bytea CHECK (octet_length("DeployKeySealed") BETWEEN 1 AND 8192),
    "PreviousDeployKeyPublic"    TEXT CHECK (char_length("PreviousDeployKeyPublic") BETWEEN 1 AND 1024),
    "PreviousDeployKeySealed"    bytea CHECK (octet_length("PreviousDeployKeySealed") BETWEEN 1 AND 8192),
    "PreviousDeployKeyExpiresAt" timestamptz,
    "WebhookSecretSealed"        bytea CHECK (octet_length("WebhookSecretSealed") BETWEEN 1 AND 1024),
    "HookId"                     TEXT CHECK ("HookId" ~ '^[0-9a-f]{32}$'),
    "LastDeliveryAt"             timestamptz,
    "CreatedByUserId"            uuid NOT NULL REFERENCES "Users" ("Id"),
    FOREIGN KEY ("ProjectId", "OrganizationId") REFERENCES "Projects" ("Id", "OrganizationId"),
    FOREIGN KEY ("ConnectionId", "OrganizationId") REFERENCES "GitConnections" ("Id", "OrganizationId"),
    CHECK (("Provider" = 'generic') = ("ConnectionId" IS NULL)),
    CHECK (("Provider" = 'github') = ("HookId" IS NULL)),
    CHECK (("HookId" IS NULL) = ("WebhookSecretSealed" IS NULL)),
    CHECK (("DeployKeyPublic" IS NULL) = ("DeployKeySealed" IS NULL)),
    CHECK (("PreviousDeployKeyPublic" IS NULL) = ("PreviousDeployKeySealed" IS NULL)),
    CHECK (("PreviousDeployKeyPublic" IS NULL) = ("PreviousDeployKeyExpiresAt" IS NULL)),
    CHECK ("PreviousDeployKeyPublic" IS NULL OR "DeployKeyPublic" IS NOT NULL),
    CHECK (("HostKey" IS NULL) = ("HostKeyConfirmedAt" IS NULL)),
    CHECK (("PendingHostKey" IS NULL) = ("PendingHostKeyFetchedAt" IS NULL)),
    CHECK ("Transport" = 'ssh' OR ("HostKey" IS NULL AND "PendingHostKey" IS NULL AND "DeployKeyPublic" IS NULL)),
    CHECK ("Transport" = 'https' OR "Provider" <> 'generic' OR "DeployKeyPublic" IS NOT NULL)
);

CREATE UNIQUE INDEX "UX_Repositories_Project" ON "Repositories" ("ProjectId") WHERE NOT "IsDeleted";
CREATE UNIQUE INDEX "UX_Repositories_HookId" ON "Repositories" ("HookId");
CREATE UNIQUE INDEX "UX_Repositories_Id_Organization" ON "Repositories" ("Id", "OrganizationId");
CREATE INDEX "IX_Repositories_Connection" ON "Repositories" ("ConnectionId") WHERE "ConnectionId" IS NOT NULL AND NOT "IsDeleted";

CREATE TABLE "GitSetupFlows" (
    "Id"             uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"      timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"      timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"      boolean NOT NULL DEFAULT false,
    "Kind"           TEXT NOT NULL CHECK ("Kind" IN ('manifest', 'install', 'gitlab_oauth')),
    "StateHash"      bytea NOT NULL CHECK (octet_length("StateHash") = 32),
    "UserId"         uuid NOT NULL REFERENCES "Users" ("Id"),
    "OrganizationId" uuid REFERENCES "Organizations" ("Id"),
    "ExpiresAt"      timestamptz NOT NULL,
    "ConsumedAt"     timestamptz,
    CHECK (("Kind" = 'manifest') = ("OrganizationId" IS NULL)),
    CHECK ("ExpiresAt" > "CreatedAt")
);

CREATE UNIQUE INDEX "UX_GitSetupFlows_StateHash" ON "GitSetupFlows" ("StateHash");
CREATE INDEX "IX_GitSetupFlows_Live" ON "GitSetupFlows" ("ExpiresAt") WHERE NOT "IsDeleted";

CREATE TABLE "WebhookDeliveries" (
    "Id"             uuid NOT NULL DEFAULT uuidv7(),
    "CreatedAt"      timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"      timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"      boolean NOT NULL DEFAULT false,
    "Provider"       TEXT NOT NULL CHECK ("Provider" IN ('github', 'gitlab', 'generic')),
    "HookId"         TEXT NOT NULL DEFAULT '' CHECK ("HookId" = '' OR "HookId" ~ '^[0-9a-f]{32}$'),
    "DeliveryId"     TEXT NOT NULL CHECK ("DeliveryId" ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'),
    "PayloadDigest"  bytea NOT NULL CHECK (octet_length("PayloadDigest") = 32),
    "OrganizationId" uuid,
    "RepositoryId"   uuid,
    "Event"          TEXT NOT NULL CHECK ("Event" ~ '^[a-z_]{1,32}$'),
    "Ref"            TEXT CHECK (char_length("Ref") BETWEEN 1 AND 255),
    "Sha"            TEXT CHECK ("Sha" ~ '^([0-9a-f]{40}|[0-9a-f]{64})$'),
    "Outcome"        TEXT NOT NULL CHECK ("Outcome" IN ('triggered', 'ignored')),
    "Reason"         TEXT CHECK ("Reason" ~ '^[a-z_]{1,32}$'),
    PRIMARY KEY ("Id", "CreatedAt"),
    CHECK (("Outcome" = 'ignored') = ("Reason" IS NOT NULL))
) PARTITION BY RANGE ("CreatedAt");

CREATE INDEX "IX_WebhookDeliveries_Delivery" ON "WebhookDeliveries" ("Provider", "HookId", "DeliveryId", "CreatedAt");
CREATE INDEX "IX_WebhookDeliveries_Digest" ON "WebhookDeliveries" ("Provider", "HookId", "PayloadDigest", "CreatedAt");
CREATE INDEX "IX_WebhookDeliveries_Repository" ON "WebhookDeliveries" ("RepositoryId", "CreatedAt") WHERE "RepositoryId" IS NOT NULL;

-- +goose StatementBegin
CREATE FUNCTION "WebhookPartitionName"("Day" date) RETURNS TEXT LANGUAGE sql IMMUTABLE AS $$
    SELECT 'WebhookDeliveries_' || to_char("Day", 'YYYYMMDD')
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION "WebhookPartitionDay"("Name" TEXT) RETURNS date LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
    "Digits" TEXT;
    "Parsed" date;
BEGIN
    IF "Name" IS NULL OR "Name" !~ '^WebhookDeliveries_[0-9]{8}$' THEN
        RETURN NULL;
    END IF;
    "Digits" := substring("Name" FROM 19);
    BEGIN
        "Parsed" := to_date("Digits", 'YYYYMMDD');
    EXCEPTION WHEN datetime_field_overflow OR invalid_datetime_format THEN
        RETURN NULL;
    END;
    IF to_char("Parsed", 'YYYYMMDD') <> "Digits" THEN
        RETURN NULL;
    END IF;
    RETURN "Parsed";
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION "EnsureWebhookPartitions"("DaysAhead" integer) RETURNS integer LANGUAGE plpgsql AS $$
DECLARE
    "Parent"   regclass := '"WebhookDeliveries"'::regclass;
    "Schema"   TEXT;
    "Target"   date;
    "Name"     TEXT;
    "Existing" regclass;
    "Created"  integer := 0;
BEGIN
    IF "DaysAhead" < 0 OR "DaysAhead" > 31 THEN
        RAISE EXCEPTION 'webhook partitions are made from 0 to 31 days ahead, not %', "DaysAhead"
            USING ERRCODE = 'invalid_parameter_value';
    END IF;
    SELECT n.nspname INTO "Schema" FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE c.oid = "Parent";
    FOR "Offset" IN 0.."DaysAhead" LOOP
        "Target" := "LogPartitionToday"() + "Offset";
        "Name" := "WebhookPartitionName"("Target");
        "Existing" := to_regclass(format('%I.%I', "Schema", "Name"));
        IF "Existing" IS NULL THEN
            EXECUTE format('CREATE TABLE %I.%I (LIKE %s INCLUDING ALL)', "Schema", "Name", "Parent");
            EXECUTE format('ALTER TABLE %s ATTACH PARTITION %I.%I FOR VALUES FROM (%L) TO (%L)',
                "Parent", "Schema", "Name", "LogPartitionBound"("Target"), "LogPartitionBound"("Target" + 1));
            "Created" := "Created" + 1;
        ELSIF NOT EXISTS (SELECT 1 FROM pg_inherits i WHERE i.inhrelid = "Existing" AND i.inhparent = "Parent") THEN
            RAISE EXCEPTION 'table %.% exists but is not a partition of "WebhookDeliveries"; attach it or rename it', "Schema", "Name"
                USING ERRCODE = 'object_not_in_prerequisite_state';
        END IF;
    END LOOP;
    RETURN "Created";
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION "DropWebhookPartitions"("RetentionDays" integer) RETURNS integer LANGUAGE plpgsql AS $$
DECLARE
    "Child"   record;
    "Dropped" integer := 0;
BEGIN
    IF "RetentionDays" < 4 THEN
        RAISE EXCEPTION 'webhook deliveries are kept at least 4 days, longer than the replay window, not %', "RetentionDays"
            USING ERRCODE = 'invalid_parameter_value';
    END IF;
    FOR "Child" IN
        SELECT c.oid::regclass AS "Partition"
        FROM pg_inherits i
        JOIN pg_class c ON c.oid = i.inhrelid
        WHERE i.inhparent = '"WebhookDeliveries"'::regclass
          AND "WebhookPartitionDay"(c.relname) < "LogPartitionToday"() - "RetentionDays"
        ORDER BY c.relname
    LOOP
        EXECUTE format('DROP TABLE %s', "Child"."Partition");
        "Dropped" := "Dropped" + 1;
    END LOOP;
    RETURN "Dropped";
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION "WebhookPartitionReady"() RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1
        FROM pg_inherits i
        JOIN pg_class c ON c.oid = i.inhrelid
        WHERE i.inhparent = '"WebhookDeliveries"'::regclass AND c.relname = "WebhookPartitionName"("LogPartitionToday"())
    )
$$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
DECLARE
    "Function" TEXT;
BEGIN
    FOREACH "Function" IN ARRAY ARRAY[
        '"WebhookPartitionName"(date)', '"WebhookPartitionDay"(text)', '"EnsureWebhookPartitions"(integer)',
        '"DropWebhookPartitions"(integer)', '"WebhookPartitionReady"()'
    ] LOOP
        EXECUTE format('ALTER FUNCTION %I.%s SET search_path = pg_catalog, %I, pg_temp', current_schema(), "Function", current_schema());
    END LOOP;
END
$$;
-- +goose StatementEnd

SELECT "EnsureWebhookPartitions"(3);

CREATE TABLE "CommitStatusReports" (
    "Id"             uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"      timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"      timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"      boolean NOT NULL DEFAULT false,
    "OrganizationId" uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "RepositoryId"   uuid NOT NULL,
    "Sha"            TEXT NOT NULL CHECK ("Sha" ~ '^([0-9a-f]{40}|[0-9a-f]{64})$'),
    "RunId"          uuid NOT NULL,
    "TargetUrl"      TEXT CHECK (char_length("TargetUrl") BETWEEN 1 AND 2048),
    "DesiredState"   TEXT NOT NULL CHECK ("DesiredState" IN ('pending', 'running', 'success', 'failed', 'canceled')),
    "DesiredAt"      timestamptz NOT NULL DEFAULT now(),
    "SentState"      TEXT CHECK ("SentState" IN ('pending', 'running', 'success', 'failed', 'canceled')),
    "Attempts"       integer NOT NULL DEFAULT 0 CHECK ("Attempts" >= 0),
    "NextAttemptAt"  timestamptz NOT NULL DEFAULT now(),
    "LastError"      TEXT CHECK (char_length("LastError") BETWEEN 1 AND 256),
    "AbandonedAt"    timestamptz,
    FOREIGN KEY ("RepositoryId", "OrganizationId") REFERENCES "Repositories" ("Id", "OrganizationId")
);

CREATE UNIQUE INDEX "UX_CommitStatusReports_Run" ON "CommitStatusReports" ("RepositoryId", "RunId") WHERE NOT "IsDeleted";
CREATE INDEX "IX_CommitStatusReports_Due" ON "CommitStatusReports" ("NextAttemptAt")
    WHERE NOT "IsDeleted" AND "AbandonedAt" IS NULL AND "SentState" IS DISTINCT FROM "DesiredState";

-- +goose Down
DROP TABLE "CommitStatusReports";
DROP FUNCTION "WebhookPartitionReady"();
DROP FUNCTION "DropWebhookPartitions"(integer);
DROP FUNCTION "EnsureWebhookPartitions"(integer);
DROP FUNCTION "WebhookPartitionDay"(text);
DROP FUNCTION "WebhookPartitionName"(date);
DROP TABLE "WebhookDeliveries";
DROP TABLE "GitSetupFlows";
DROP TABLE "Repositories";
DROP TABLE "GitConnections";
