-- +goose Up
CREATE UNIQUE INDEX "UX_JobAttempts_Id_Job_Organization" ON "JobAttempts" ("Id", "JobId", "OrganizationId");

ALTER TABLE "JobAttempts"
    ADD COLUMN "LogTruncatedAt" timestamptz,
    ADD COLUMN "LateResultLoggedAt" timestamptz;

CREATE TABLE "LogChunks" (
    "Id"             uuid NOT NULL DEFAULT uuidv7(),
    "CreatedAt"      timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"      timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"      boolean NOT NULL DEFAULT false,
    "OrganizationId" uuid NOT NULL,
    "JobId"          uuid NOT NULL,
    "AttemptId"      uuid NOT NULL,
    "Seq"            bigint NOT NULL CHECK ("Seq" >= 1),
    "Stream"         TEXT NOT NULL CHECK ("Stream" IN ('stdout', 'stderr', 'system')),
    "StepIndex"      integer CHECK ("StepIndex" BETWEEN 0 AND 99),
    "EmittedAt"      timestamptz,
    "Data"           bytea NOT NULL CHECK (octet_length("Data") BETWEEN 1 AND 65536),
    PRIMARY KEY ("Id", "CreatedAt"),
    FOREIGN KEY ("AttemptId", "JobId", "OrganizationId") REFERENCES "JobAttempts" ("Id", "JobId", "OrganizationId")
) PARTITION BY RANGE ("CreatedAt");

CREATE INDEX "IX_LogChunks_Attempt_Seq" ON "LogChunks" ("AttemptId", "Seq");

-- +goose StatementBegin
CREATE FUNCTION "LogPartitionName"("Day" date) RETURNS TEXT LANGUAGE sql IMMUTABLE AS $$
    SELECT 'LogChunks_' || to_char("Day", 'YYYYMMDD')
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION "LogPartitionBound"("Day" date) RETURNS TEXT LANGUAGE sql IMMUTABLE AS $$
    SELECT to_char("Day", 'YYYY-MM-DD') || ' 00:00:00+00'
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION "LogPartitionToday"() RETURNS date LANGUAGE sql STABLE AS $$
    SELECT (now() AT TIME ZONE 'UTC')::date
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION "EnsureLogPartitions"("DaysAhead" integer) RETURNS integer LANGUAGE plpgsql AS $$
DECLARE
    "Parent"  regclass := '"LogChunks"'::regclass;
    "Schema"  TEXT;
    "Target"  date;
    "Created" integer := 0;
BEGIN
    IF "DaysAhead" < 0 OR "DaysAhead" > 31 THEN
        RAISE EXCEPTION 'log partitions are made from 0 to 31 days ahead, not %', "DaysAhead"
            USING ERRCODE = 'invalid_parameter_value';
    END IF;
    SELECT n.nspname INTO "Schema" FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE c.oid = "Parent";
    FOR "Offset" IN 0.."DaysAhead" LOOP
        "Target" := "LogPartitionToday"() + "Offset";
        IF to_regclass(format('%I.%I', "Schema", "LogPartitionName"("Target"))) IS NULL THEN
            EXECUTE format('CREATE TABLE %I.%I PARTITION OF %s FOR VALUES FROM (%L) TO (%L)',
                "Schema", "LogPartitionName"("Target"), "Parent",
                "LogPartitionBound"("Target"), "LogPartitionBound"("Target" + 1));
            "Created" := "Created" + 1;
        END IF;
    END LOOP;
    RETURN "Created";
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION "DropLogPartitions"("RetentionDays" integer) RETURNS integer LANGUAGE plpgsql AS $$
DECLARE
    "Child"   record;
    "Dropped" integer := 0;
BEGIN
    IF "RetentionDays" < 1 THEN
        RAISE EXCEPTION 'log retention is at least one day, not %', "RetentionDays"
            USING ERRCODE = 'invalid_parameter_value';
    END IF;
    FOR "Child" IN
        SELECT c.oid::regclass AS "Partition"
        FROM pg_inherits i
        JOIN pg_class c ON c.oid = i.inhrelid
        WHERE i.inhparent = '"LogChunks"'::regclass AND c.relname ~ '^LogChunks_[0-9]{8}$'
          AND to_date(substring(c.relname FROM 11), 'YYYYMMDD') < "LogPartitionToday"() - "RetentionDays"
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
CREATE FUNCTION "LogPartitionReady"() RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1
        FROM pg_inherits i
        JOIN pg_class c ON c.oid = i.inhrelid
        WHERE i.inhparent = '"LogChunks"'::regclass AND c.relname = "LogPartitionName"("LogPartitionToday"())
    )
$$;
-- +goose StatementEnd

SELECT "EnsureLogPartitions"(3);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION "GuardAttemptState"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW."State" = 'leased' THEN
            RETURN NEW;
        END IF;
        RAISE EXCEPTION 'an attempt starts leased, not %', NEW."State" USING ERRCODE = 'check_violation';
    END IF;
    IF NEW."LastSeq" < OLD."LastSeq" OR NEW."LogBytes" < OLD."LogBytes"
        OR (OLD."LogTruncatedAt" IS NOT NULL AND NEW."LogTruncatedAt" IS DISTINCT FROM OLD."LogTruncatedAt")
        OR (OLD."LateResultLoggedAt" IS NOT NULL AND NEW."LateResultLoggedAt" IS DISTINCT FROM OLD."LateResultLoggedAt") THEN
        RAISE EXCEPTION 'the log of an attempt only grows' USING ERRCODE = 'check_violation';
    END IF;
    IF OLD."State" IN ('succeeded', 'failed', 'canceled', 'timed_out', 'lost') THEN
        IF (to_jsonb(NEW) - 'IsDeleted' - 'UpdatedAt' - 'LastSeq' - 'LogBytes' - 'LateResultLoggedAt')
            IS DISTINCT FROM (to_jsonb(OLD) - 'IsDeleted' - 'UpdatedAt' - 'LastSeq' - 'LogBytes' - 'LateResultLoggedAt') THEN
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

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION "GuardAttemptState"() RETURNS trigger LANGUAGE plpgsql AS $$
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
DROP TABLE "LogChunks";
DROP FUNCTION "LogPartitionReady"();
DROP FUNCTION "DropLogPartitions"(integer);
DROP FUNCTION "EnsureLogPartitions"(integer);
DROP FUNCTION "LogPartitionToday"();
DROP FUNCTION "LogPartitionBound"(date);
DROP FUNCTION "LogPartitionName"(date);
ALTER TABLE "JobAttempts" DROP COLUMN "LateResultLoggedAt", DROP COLUMN "LogTruncatedAt";
DROP INDEX "UX_JobAttempts_Id_Job_Organization";
