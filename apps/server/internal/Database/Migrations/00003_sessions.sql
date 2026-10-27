-- +goose Up
CREATE TABLE "Sessions" (
    "Id"            uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"     timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"     timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"     boolean NOT NULL DEFAULT false,
    "UserId"        uuid NOT NULL REFERENCES "Users" ("Id"),
    "TokenHash"     bytea NOT NULL CHECK (octet_length("TokenHash") = 32),
    "CsrfHash"      bytea NOT NULL CHECK (octet_length("CsrfHash") = 32),
    "LastSeenAt"    timestamptz NOT NULL DEFAULT now(),
    "ExpiresAt"     timestamptz NOT NULL,
    "IdleExpiresAt" timestamptz NOT NULL,
    "RevokedAt"     timestamptz,
    "UserAgent"     TEXT NOT NULL DEFAULT '',
    "IpAddress"     TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX "UX_Sessions_TokenHash" ON "Sessions" ("TokenHash");
CREATE INDEX "IX_Sessions_User_Active" ON "Sessions" ("UserId", "CreatedAt" DESC) WHERE "RevokedAt" IS NULL AND NOT "IsDeleted";

CREATE TABLE "LoginThrottles" (
    "Id"            uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"     timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"     timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"     boolean NOT NULL DEFAULT false,
    "Key"           TEXT NOT NULL CHECK (char_length("Key") BETWEEN 1 AND 128),
    "Failures"      integer NOT NULL DEFAULT 0 CHECK ("Failures" >= 0),
    "LastFailureAt" timestamptz NOT NULL DEFAULT now(),
    "LockedUntil"   timestamptz
);

CREATE UNIQUE INDEX "UX_LoginThrottles_Key" ON "LoginThrottles" ("Key");

-- +goose Down
DROP TABLE "LoginThrottles";
DROP TABLE "Sessions";
