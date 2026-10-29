-- +goose Up
CREATE TABLE "InstanceSettings" (
    "Id"        uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt" timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt" timestamptz NOT NULL DEFAULT now(),
    "IsDeleted" boolean NOT NULL DEFAULT false,
    "Group"     TEXT NOT NULL CHECK ("Group" ~ '^[a-z][a-z-]{0,31}$'),
    "Value"     jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof("Value") = 'object'),
    "Secrets"   bytea CHECK ("Secrets" IS NULL OR octet_length("Secrets") BETWEEN 29 AND 65536),
    "Version"   bigint NOT NULL DEFAULT 1 CHECK ("Version" >= 1),
    "UpdatedBy" uuid REFERENCES "Users" ("Id")
);

CREATE UNIQUE INDEX "UX_InstanceSettings_Group" ON "InstanceSettings" ("Group") WHERE NOT "IsDeleted";

ALTER TABLE "JobAttempts" ADD COLUMN "LogMaxBytes" bigint CHECK ("LogMaxBytes" IS NULL OR "LogMaxBytes" > 0);

-- +goose Down
ALTER TABLE "JobAttempts" DROP COLUMN "LogMaxBytes";
DROP TABLE "InstanceSettings";
