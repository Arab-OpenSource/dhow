-- +goose Up
CREATE TABLE "AuditEvents" (
    "Id"             uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"      timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"      timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"      boolean NOT NULL DEFAULT false,
    "Seq"            bigint GENERATED ALWAYS AS IDENTITY NOT NULL,
    "OrganizationId" uuid REFERENCES "Organizations" ("Id"),
    "ActorUserId"    uuid REFERENCES "Users" ("Id"),
    "Action"         TEXT NOT NULL CHECK (char_length("Action") BETWEEN 1 AND 100),
    "TargetType"     TEXT NOT NULL DEFAULT '',
    "TargetId"       TEXT NOT NULL DEFAULT '',
    "Ip"             TEXT NOT NULL DEFAULT '',
    "Data"           jsonb NOT NULL DEFAULT '{}'::jsonb,
    "PrevHash"       bytea NOT NULL CHECK (octet_length("PrevHash") = 32),
    "Hash"           bytea NOT NULL CHECK (octet_length("Hash") = 32)
);

CREATE UNIQUE INDEX "UX_AuditEvents_Seq" ON "AuditEvents" ("Seq");
CREATE INDEX "IX_AuditEvents_Organization_Seq" ON "AuditEvents" ("OrganizationId", "Seq" DESC);
CREATE INDEX "IX_AuditEvents_Actor" ON "AuditEvents" ("ActorUserId") WHERE "ActorUserId" IS NOT NULL;

-- +goose StatementBegin
CREATE FUNCTION "AuditEvents_RejectChange"() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'AuditEvents is append-only: % is not allowed', TG_OP USING ERRCODE = 'restrict_violation';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER "TR_AuditEvents_AppendOnly"
    BEFORE UPDATE OR DELETE ON "AuditEvents"
    FOR EACH ROW EXECUTE FUNCTION "AuditEvents_RejectChange"();

CREATE TRIGGER "TR_AuditEvents_NoTruncate"
    BEFORE TRUNCATE ON "AuditEvents"
    FOR EACH STATEMENT EXECUTE FUNCTION "AuditEvents_RejectChange"();

-- +goose Down
DROP TABLE "AuditEvents";
DROP FUNCTION "AuditEvents_RejectChange"();
