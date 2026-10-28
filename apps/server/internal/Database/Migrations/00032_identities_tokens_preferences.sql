-- +goose Up
CREATE TABLE "UserIdentities" (
    "Id"         uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"  timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"  timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"  boolean NOT NULL DEFAULT false,
    "UserId"     uuid NOT NULL REFERENCES "Users" ("Id"),
    "Provider"   TEXT NOT NULL CHECK ("Provider" ~ '^[a-z][a-z0-9]{0,31}$'),
    "Issuer"     TEXT NOT NULL CHECK (char_length("Issuer") BETWEEN 1 AND 2048),
    "Subject"    TEXT NOT NULL CHECK (char_length("Subject") BETWEEN 1 AND 255),
    "Email"      TEXT NOT NULL DEFAULT '' CHECK (char_length("Email") <= 254),
    "LastUsedAt" timestamptz
);

CREATE UNIQUE INDEX "UX_UserIdentities_Issuer_Subject" ON "UserIdentities" ("Issuer", "Subject") WHERE NOT "IsDeleted";
CREATE UNIQUE INDEX "UX_UserIdentities_User_Provider" ON "UserIdentities" ("UserId", "Provider") WHERE NOT "IsDeleted";

CREATE TABLE "OidcFlows" (
    "Id"         uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"  timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"  timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"  boolean NOT NULL DEFAULT false,
    "Purpose"    TEXT NOT NULL CHECK ("Purpose" IN ('login', 'link')),
    "Provider"   TEXT NOT NULL CHECK ("Provider" ~ '^[a-z][a-z0-9]{0,31}$'),
    "TokenHash"  bytea NOT NULL CHECK (octet_length("TokenHash") = 32),
    "StateHash"  bytea NOT NULL CHECK (octet_length("StateHash") = 32),
    "DeviceHash" bytea NOT NULL CHECK (octet_length("DeviceHash") = 32),
    "UserId"     uuid REFERENCES "Users" ("Id"),
    "SessionId"  uuid REFERENCES "Sessions" ("Id"),
    "ExpiresAt"  timestamptz NOT NULL,
    "UsedAt"     timestamptz,
    CHECK ("Purpose" <> 'link' OR ("UserId" IS NOT NULL AND "SessionId" IS NOT NULL))
);

CREATE UNIQUE INDEX "UX_OidcFlows_TokenHash" ON "OidcFlows" ("TokenHash");
CREATE INDEX "IX_OidcFlows_Live" ON "OidcFlows" ("ExpiresAt") WHERE NOT "IsDeleted";

CREATE TABLE "ApiTokens" (
    "Id"              uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"       timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"       timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"       boolean NOT NULL DEFAULT false,
    "UserId"          uuid NOT NULL REFERENCES "Users" ("Id"),
    "OrganizationId"  uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "MembershipId"    uuid NOT NULL REFERENCES "Memberships" ("Id"),
    "Name"            TEXT NOT NULL CHECK (char_length("Name") BETWEEN 1 AND 100),
    "TokenHash"       bytea NOT NULL CHECK (octet_length("TokenHash") = 32),
    "Scopes"          TEXT[] NOT NULL CHECK (
        cardinality("Scopes") BETWEEN 1 AND 16
        AND "Scopes" <@ ARRAY['org.read', 'org.update', 'member.read', 'member.manage', 'audit.read', 'runner.read', 'runner.manage', 'job.read', 'job.cancel']::text[]
    ),
    "ExpiresAt"       timestamptz NOT NULL,
    "RevokedAt"       timestamptz,
    "RevokedByUserId" uuid REFERENCES "Users" ("Id"),
    "LastUsedAt"      timestamptz,
    "LastUsedIp"      TEXT NOT NULL DEFAULT '' CHECK (char_length("LastUsedIp") <= 64),
    CHECK ("ExpiresAt" > "CreatedAt"),
    CHECK (("RevokedAt" IS NULL) = ("RevokedByUserId" IS NULL))
);

CREATE UNIQUE INDEX "UX_ApiTokens_TokenHash" ON "ApiTokens" ("TokenHash");
CREATE INDEX "IX_ApiTokens_User" ON "ApiTokens" ("UserId", "CreatedAt" DESC) WHERE NOT "IsDeleted";
CREATE INDEX "IX_ApiTokens_Organization" ON "ApiTokens" ("OrganizationId", "CreatedAt" DESC) WHERE NOT "IsDeleted";

CREATE TABLE "UserPreferences" (
    "Id"             uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"      timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"      timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"      boolean NOT NULL DEFAULT false,
    "UserId"         uuid NOT NULL REFERENCES "Users" ("Id"),
    "ThemeId"        TEXT NOT NULL CHECK ("ThemeId" ~ '^[a-z0-9-]{1,40}$'),
    "CustomThemes"   jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (
        jsonb_typeof("CustomThemes") = 'array' AND octet_length("CustomThemes"::text) <= 65536
    ),
    "OrganizationId" uuid REFERENCES "Organizations" ("Id")
);

CREATE UNIQUE INDEX "UX_UserPreferences_User" ON "UserPreferences" ("UserId") WHERE NOT "IsDeleted";

-- +goose Down
DROP TABLE "UserPreferences";
DROP TABLE "ApiTokens";
DROP TABLE "OidcFlows";
DROP TABLE "UserIdentities";
