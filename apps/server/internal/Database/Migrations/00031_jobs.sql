-- +goose Up
CREATE SEQUENCE "FencingTokens" AS bigint START WITH 1 INCREMENT BY 1 NO CYCLE;

CREATE TABLE "Jobs" (
    "Id"                uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"         timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"         timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"         boolean NOT NULL DEFAULT false,
    "OrganizationId"    uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "CreatedByUserId"   uuid NOT NULL REFERENCES "Users" ("Id"),
    "Name"              TEXT NOT NULL CHECK (char_length("Name") BETWEEN 1 AND 100),
    "Spec"              jsonb NOT NULL CHECK (jsonb_typeof("Spec") = 'object'),
    "SpecDigest"        bytea NOT NULL CHECK (octet_length("SpecDigest") = 32),
    "Image"             TEXT NOT NULL CHECK (char_length("Image") BETWEEN 1 AND 512),
    "Labels"            TEXT[] NOT NULL DEFAULT '{}' CHECK (cardinality("Labels") <= 32),
    "Isolation"         TEXT NOT NULL CHECK ("Isolation" IN ('container', 'gvisor')),
    "Priority"          integer NOT NULL DEFAULT 0 CHECK ("Priority" BETWEEN 0 AND 100),
    "State"             TEXT NOT NULL DEFAULT 'queued'
                        CHECK ("State" IN ('queued', 'leased', 'running', 'succeeded', 'failed', 'canceled', 'timed_out', 'lost')),
    "FailureReason"     TEXT NOT NULL DEFAULT '' CHECK (char_length("FailureReason") <= 64),
    "QueuedAt"          timestamptz NOT NULL DEFAULT now(),
    "StartedAt"         timestamptz,
    "FinishedAt"        timestamptz,
    "Deadline"          timestamptz,
    "QueueDeadline"     timestamptz NOT NULL,
    "CancelRequestedAt" timestamptz,
    "CancelRequestedBy" uuid REFERENCES "Users" ("Id"),
    "AttemptCount"      integer NOT NULL DEFAULT 0 CHECK ("AttemptCount" >= 0),
    "MaxInfraRetries"   integer NOT NULL DEFAULT 2 CHECK ("MaxInfraRetries" BETWEEN 0 AND 5),
    CHECK (("FinishedAt" IS NOT NULL) = ("State" IN ('succeeded', 'failed', 'canceled', 'timed_out', 'lost'))),
    CHECK (("CancelRequestedAt" IS NULL) = ("CancelRequestedBy" IS NULL)),
    CHECK ("QueueDeadline" > "QueuedAt"),
    CHECK ("State" <> 'queued' OR "Deadline" IS NULL),
    CHECK ("AttemptCount" <= "MaxInfraRetries" + 1)
);

CREATE INDEX "IX_Jobs_Queue" ON "Jobs" ("OrganizationId", "State", "Priority" DESC, "QueuedAt")
    WHERE "State" = 'queued' AND NOT "IsDeleted";
CREATE INDEX "IX_Jobs_QueueDeadline" ON "Jobs" ("QueueDeadline") WHERE "State" = 'queued' AND NOT "IsDeleted";
CREATE INDEX "IX_Jobs_Organization" ON "Jobs" ("OrganizationId", "CreatedAt" DESC) WHERE NOT "IsDeleted";
CREATE UNIQUE INDEX "UX_Jobs_Id_Organization" ON "Jobs" ("Id", "OrganizationId");
CREATE UNIQUE INDEX "UX_Runners_Id_Organization" ON "Runners" ("Id", "OrganizationId");

CREATE TABLE "JobAttempts" (
    "Id"                uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"         timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"         timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"         boolean NOT NULL DEFAULT false,
    "JobId"             uuid NOT NULL,
    "OrganizationId"    uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "RunnerId"          uuid NOT NULL,
    "Number"            integer NOT NULL CHECK ("Number" >= 1),
    "FencingToken"      bigint NOT NULL DEFAULT nextval('"FencingTokens"') CHECK ("FencingToken" > 0),
    "State"             TEXT NOT NULL DEFAULT 'leased'
                        CHECK ("State" IN ('leased', 'running', 'succeeded', 'failed', 'canceled', 'timed_out', 'lost')),
    "FailureReason"     TEXT NOT NULL DEFAULT '' CHECK (char_length("FailureReason") <= 64),
    "ExitCode"          integer,
    "LeasedAt"          timestamptz NOT NULL DEFAULT now(),
    "StartedAt"         timestamptz,
    "FinishedAt"        timestamptz,
    "MutationStartedAt" timestamptz,
    "CompletionDigest"  bytea CHECK (octet_length("CompletionDigest") = 32),
    "LastSeq"           bigint NOT NULL DEFAULT 0 CHECK ("LastSeq" >= 0),
    "LogBytes"          bigint NOT NULL DEFAULT 0 CHECK ("LogBytes" >= 0),
    CHECK (("FinishedAt" IS NOT NULL) = ("State" IN ('succeeded', 'failed', 'canceled', 'timed_out', 'lost'))),
    CHECK ("State" <> 'running' OR "StartedAt" IS NOT NULL),
    CHECK ("MutationStartedAt" IS NULL OR "StartedAt" IS NOT NULL),
    FOREIGN KEY ("JobId", "OrganizationId") REFERENCES "Jobs" ("Id", "OrganizationId"),
    FOREIGN KEY ("RunnerId", "OrganizationId") REFERENCES "Runners" ("Id", "OrganizationId")
);

CREATE UNIQUE INDEX "UX_JobAttempts_FencingToken" ON "JobAttempts" ("FencingToken");
CREATE UNIQUE INDEX "UX_JobAttempts_Job_Number" ON "JobAttempts" ("JobId", "Number");
CREATE UNIQUE INDEX "UX_JobAttempts_Job_Open" ON "JobAttempts" ("JobId")
    WHERE "State" IN ('leased', 'running') AND NOT "IsDeleted";
CREATE INDEX "IX_JobAttempts_Job" ON "JobAttempts" ("JobId", "Number");
CREATE UNIQUE INDEX "UX_JobAttempts_Lease_Key" ON "JobAttempts" ("Id", "JobId", "OrganizationId", "RunnerId", "FencingToken");

CREATE TABLE "Leases" (
    "Id"              uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"       timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"       timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"       boolean NOT NULL DEFAULT false,
    "AttemptId"       uuid NOT NULL,
    "JobId"           uuid NOT NULL,
    "OrganizationId"  uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "RunnerId"        uuid NOT NULL,
    "FencingToken"    bigint NOT NULL CHECK ("FencingToken" > 0),
    "GrantDigest"     bytea NOT NULL CHECK (octet_length("GrantDigest") = 32),
    "State"           TEXT NOT NULL DEFAULT 'active' CHECK ("State" IN ('active', 'released', 'expired', 'revoked')),
    "ExpiresAt"       timestamptz NOT NULL,
    "LastHeartbeatAt" timestamptz NOT NULL DEFAULT now(),
    "EndedAt"         timestamptz,
    CHECK (("EndedAt" IS NULL) = ("State" = 'active')),
    FOREIGN KEY ("AttemptId", "JobId", "OrganizationId", "RunnerId", "FencingToken")
        REFERENCES "JobAttempts" ("Id", "JobId", "OrganizationId", "RunnerId", "FencingToken")
);

CREATE UNIQUE INDEX "UX_Leases_AttemptId" ON "Leases" ("AttemptId");
CREATE UNIQUE INDEX "UX_Leases_FencingToken" ON "Leases" ("FencingToken");
CREATE INDEX "IX_Leases_Active_ExpiresAt" ON "Leases" ("ExpiresAt") WHERE "State" = 'active' AND NOT "IsDeleted";
CREATE INDEX "IX_Leases_Runner_Active" ON "Leases" ("RunnerId") WHERE "State" = 'active' AND NOT "IsDeleted";

CREATE TABLE "AttemptSteps" (
    "Id"            uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"     timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"     timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"     boolean NOT NULL DEFAULT false,
    "AttemptId"     uuid NOT NULL REFERENCES "JobAttempts" ("Id"),
    "Index"         integer NOT NULL CHECK ("Index" BETWEEN 0 AND 99),
    "Name"          TEXT NOT NULL CHECK (char_length("Name") BETWEEN 1 AND 100),
    "Mutating"      boolean NOT NULL DEFAULT false,
    "State"         TEXT NOT NULL DEFAULT 'running' CHECK ("State" IN ('running', 'succeeded', 'failed', 'interrupted')),
    "StartedAt"     timestamptz NOT NULL DEFAULT now(),
    "FinishedAt"    timestamptz,
    "ExitCode"      integer,
    "FailureReason" TEXT NOT NULL DEFAULT '' CHECK (char_length("FailureReason") <= 64),
    CHECK (("FinishedAt" IS NULL) = ("State" = 'running'))
);

CREATE UNIQUE INDEX "UX_AttemptSteps_Attempt_Index" ON "AttemptSteps" ("AttemptId", "Index");

-- +goose StatementBegin
CREATE FUNCTION "GuardJobState"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW."State" = 'queued' THEN
            RETURN NEW;
        END IF;
        RAISE EXCEPTION 'a job starts queued, not %', NEW."State" USING ERRCODE = 'check_violation';
    END IF;
    IF OLD."State" IN ('succeeded', 'failed', 'canceled', 'timed_out', 'lost') THEN
        IF (to_jsonb(NEW) - 'IsDeleted' - 'UpdatedAt') IS DISTINCT FROM (to_jsonb(OLD) - 'IsDeleted' - 'UpdatedAt') THEN
            RAISE EXCEPTION 'a finished job cannot change' USING ERRCODE = 'check_violation';
        END IF;
        RETURN NEW;
    END IF;
    IF NEW."State" = OLD."State" OR (OLD."State", NEW."State") IN (
        ('queued', 'leased'), ('queued', 'canceled'), ('queued', 'failed'),
        ('leased', 'running'), ('leased', 'queued'), ('leased', 'canceled'), ('leased', 'failed'), ('leased', 'lost'),
        ('running', 'queued'), ('running', 'succeeded'), ('running', 'failed'), ('running', 'canceled'),
        ('running', 'timed_out'), ('running', 'lost')
    ) THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'a job cannot move from % to %', OLD."State", NEW."State" USING ERRCODE = 'check_violation';
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION "GuardAttemptState"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW."State" = 'leased' THEN
            RETURN NEW;
        END IF;
        RAISE EXCEPTION 'an attempt starts leased, not %', NEW."State" USING ERRCODE = 'check_violation';
    END IF;
    IF OLD."State" IN ('succeeded', 'failed', 'canceled', 'timed_out', 'lost') THEN
        IF (to_jsonb(NEW) - 'IsDeleted' - 'UpdatedAt') IS DISTINCT FROM (to_jsonb(OLD) - 'IsDeleted' - 'UpdatedAt') THEN
            RAISE EXCEPTION 'a finished attempt cannot change' USING ERRCODE = 'check_violation';
        END IF;
        RETURN NEW;
    END IF;
    IF NEW."State" = OLD."State" OR (OLD."State", NEW."State") IN (
        ('leased', 'running'), ('leased', 'failed'), ('leased', 'canceled'), ('leased', 'lost'),
        ('running', 'succeeded'), ('running', 'failed'), ('running', 'canceled'), ('running', 'timed_out'),
        ('running', 'lost')
    ) THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'an attempt cannot move from % to %', OLD."State", NEW."State" USING ERRCODE = 'check_violation';
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION "GuardLeaseState"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW."State" = 'active' THEN
            RETURN NEW;
        END IF;
        RAISE EXCEPTION 'a lease starts active, not %', NEW."State" USING ERRCODE = 'check_violation';
    END IF;
    IF OLD."State" IN ('released', 'expired', 'revoked') THEN
        IF (to_jsonb(NEW) - 'IsDeleted' - 'UpdatedAt') IS DISTINCT FROM (to_jsonb(OLD) - 'IsDeleted' - 'UpdatedAt') THEN
            RAISE EXCEPTION 'an ended lease cannot change' USING ERRCODE = 'check_violation';
        END IF;
        RETURN NEW;
    END IF;
    IF NEW."State" = OLD."State" OR (OLD."State" = 'active' AND NEW."State" IN ('released', 'expired', 'revoked')) THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'a lease cannot move from % to %', OLD."State", NEW."State" USING ERRCODE = 'check_violation';
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION "GuardStepState"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW."State" = 'running' THEN
            RETURN NEW;
        END IF;
        RAISE EXCEPTION 'a step starts running, not %', NEW."State" USING ERRCODE = 'check_violation';
    END IF;
    IF OLD."State" IN ('succeeded', 'failed', 'interrupted') THEN
        IF (to_jsonb(NEW) - 'IsDeleted' - 'UpdatedAt') IS DISTINCT FROM (to_jsonb(OLD) - 'IsDeleted' - 'UpdatedAt') THEN
            RAISE EXCEPTION 'a finished step cannot change' USING ERRCODE = 'check_violation';
        END IF;
        RETURN NEW;
    END IF;
    IF NEW."State" = OLD."State" OR (OLD."State" = 'running' AND NEW."State" IN ('succeeded', 'failed', 'interrupted')) THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'a step cannot move from % to %', OLD."State", NEW."State" USING ERRCODE = 'check_violation';
END
$$;
-- +goose StatementEnd

CREATE TRIGGER "TR_Jobs_State" BEFORE INSERT OR UPDATE ON "Jobs"
    FOR EACH ROW EXECUTE FUNCTION "GuardJobState"();
CREATE TRIGGER "TR_JobAttempts_State" BEFORE INSERT OR UPDATE ON "JobAttempts"
    FOR EACH ROW EXECUTE FUNCTION "GuardAttemptState"();
CREATE TRIGGER "TR_Leases_State" BEFORE INSERT OR UPDATE ON "Leases"
    FOR EACH ROW EXECUTE FUNCTION "GuardLeaseState"();
CREATE TRIGGER "TR_AttemptSteps_State" BEFORE INSERT OR UPDATE ON "AttemptSteps"
    FOR EACH ROW EXECUTE FUNCTION "GuardStepState"();

-- +goose Down
DROP TABLE "AttemptSteps";
DROP TABLE "Leases";
DROP TABLE "JobAttempts";
DROP TABLE "Jobs";
DROP INDEX "UX_Runners_Id_Organization";
DROP FUNCTION "GuardStepState"();
DROP FUNCTION "GuardLeaseState"();
DROP FUNCTION "GuardAttemptState"();
DROP FUNCTION "GuardJobState"();
DROP SEQUENCE "FencingTokens";
