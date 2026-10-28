-- +goose Up
CREATE TABLE "RunnerRegistrationTokens" (
    "Id"              uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"       timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"       timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"       boolean NOT NULL DEFAULT false,
    "OrganizationId"  uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "CreatedByUserId" uuid NOT NULL REFERENCES "Users" ("Id"),
    "Description"     TEXT NOT NULL DEFAULT '' CHECK (char_length("Description") <= 200),
    "TokenHash"       bytea NOT NULL CHECK (octet_length("TokenHash") = 32),
    "Labels"          TEXT[] NOT NULL DEFAULT '{}' CHECK (cardinality("Labels") <= 32),
    "MaxUses"         integer NOT NULL DEFAULT 1 CHECK ("MaxUses" BETWEEN 1 AND 50),
    "Uses"            integer NOT NULL DEFAULT 0 CHECK ("Uses" >= 0 AND "Uses" <= "MaxUses"),
    "ExpiresAt"       timestamptz NOT NULL CHECK ("ExpiresAt" > "CreatedAt" AND "ExpiresAt" <= "CreatedAt" + interval '24 hours'),
    "LastUsedAt"      timestamptz,
    "RevokedAt"       timestamptz,
    "RevokedByUserId" uuid REFERENCES "Users" ("Id"),
    CHECK (("RevokedAt" IS NULL) = ("RevokedByUserId" IS NULL))
);

CREATE UNIQUE INDEX "UX_RunnerRegistrationTokens_TokenHash" ON "RunnerRegistrationTokens" ("TokenHash");
CREATE INDEX "IX_RunnerRegistrationTokens_Organization" ON "RunnerRegistrationTokens" ("OrganizationId", "CreatedAt" DESC) WHERE NOT "IsDeleted";

CREATE TABLE "Runners" (
    "Id"                  uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"           timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"           timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"           boolean NOT NULL DEFAULT false,
    "OrganizationId"      uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "RegistrationTokenId" uuid NOT NULL REFERENCES "RunnerRegistrationTokens" ("Id"),
    "Name"                TEXT NOT NULL CHECK (char_length("Name") BETWEEN 1 AND 100),
    "Labels"              TEXT[] NOT NULL DEFAULT '{}' CHECK (cardinality("Labels") <= 32),
    "CapabilityLabels"    TEXT[] NOT NULL DEFAULT '{}' CHECK (cardinality("CapabilityLabels") <= 8),
    "Os"                  TEXT NOT NULL CHECK ("Os" IN ('linux')),
    "Arch"                TEXT NOT NULL CHECK ("Arch" IN ('amd64', 'arm64')),
    "Version"             TEXT NOT NULL CHECK (char_length("Version") BETWEEN 1 AND 64),
    "IsolationLevels"     TEXT[] NOT NULL CHECK (cardinality("IsolationLevels") >= 1 AND "IsolationLevels" <@ ARRAY['container', 'gvisor']::text[]),
    "Capacity"            integer NOT NULL CHECK ("Capacity" BETWEEN 1 AND 16),
    "SigningPublicKey"    bytea NOT NULL CHECK (octet_length("SigningPublicKey") = 32),
    "EncryptionPublicKey" bytea NOT NULL CHECK (octet_length("EncryptionPublicKey") = 1216),
    "KeyVersion"          integer NOT NULL DEFAULT 1 CHECK ("KeyVersion" >= 1),
    "KeysRotatedAt"       timestamptz NOT NULL DEFAULT now(),
    "State"               TEXT NOT NULL DEFAULT 'active' CHECK ("State" IN ('active', 'disabled', 'revoked')),
    "DisabledReason"      TEXT NOT NULL DEFAULT '' CHECK ("DisabledReason" IN ('', 'manual', 'clone_suspected')),
    "LastSeenAt"          timestamptz,
    "LastIpAddress"       TEXT NOT NULL DEFAULT '',
    "RevokedAt"           timestamptz,
    CHECK (("State" = 'revoked') = ("RevokedAt" IS NOT NULL)),
    CHECK (("State" = 'disabled') = ("DisabledReason" <> ''))
);

CREATE UNIQUE INDEX "UX_Runners_SigningPublicKey" ON "Runners" ("SigningPublicKey");
CREATE INDEX "IX_Runners_Organization" ON "Runners" ("OrganizationId", "CreatedAt" DESC) WHERE NOT "IsDeleted";

CREATE TABLE "RunnerSessions" (
    "Id"                uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"         timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"         timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"         boolean NOT NULL DEFAULT false,
    "RunnerId"          uuid NOT NULL REFERENCES "Runners" ("Id"),
    "TokenHash"         bytea NOT NULL CHECK (octet_length("TokenHash") = 32),
    "ChallengeNonce"    bytea NOT NULL CHECK (octet_length("ChallengeNonce") = 16),
    "KeyVersion"        integer NOT NULL CHECK ("KeyVersion" >= 1),
    "ExpiresAt"         timestamptz NOT NULL CHECK ("ExpiresAt" > "CreatedAt"),
    "RevokedAt"         timestamptz,
    "ReplacedSessionId" uuid REFERENCES "RunnerSessions" ("Id"),
    "IpAddress"         TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX "UX_RunnerSessions_TokenHash" ON "RunnerSessions" ("TokenHash");
CREATE UNIQUE INDEX "UX_RunnerSessions_ChallengeNonce" ON "RunnerSessions" ("ChallengeNonce");
CREATE UNIQUE INDEX "UX_RunnerSessions_Runner_Active" ON "RunnerSessions" ("RunnerId") WHERE "RevokedAt" IS NULL AND NOT "IsDeleted";
CREATE INDEX "IX_RunnerSessions_Runner_Created" ON "RunnerSessions" ("RunnerId", "CreatedAt" DESC);

CREATE TABLE "RunnerThrottles" (
    "Id"            uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"     timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"     timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"     boolean NOT NULL DEFAULT false,
    "Key"           TEXT NOT NULL CHECK (char_length("Key") BETWEEN 1 AND 128),
    "Failures"      integer NOT NULL DEFAULT 0 CHECK ("Failures" >= 0),
    "LastFailureAt" timestamptz NOT NULL DEFAULT now(),
    "LockedUntil"   timestamptz
);

CREATE UNIQUE INDEX "UX_RunnerThrottles_Key" ON "RunnerThrottles" ("Key");

-- +goose Down
DROP TABLE "RunnerThrottles";
DROP TABLE "RunnerSessions";
DROP TABLE "Runners";
DROP TABLE "RunnerRegistrationTokens";
