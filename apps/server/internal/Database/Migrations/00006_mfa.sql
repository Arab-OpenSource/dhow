-- +goose Up
ALTER TABLE "Users" ADD COLUMN "WebauthnHandle" bytea CHECK (octet_length("WebauthnHandle") = 32);

CREATE UNIQUE INDEX "UX_Users_WebauthnHandle" ON "Users" ("WebauthnHandle") WHERE "WebauthnHandle" IS NOT NULL;

CREATE TABLE "TotpFactors" (
    "Id"           uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"    timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"    timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"    boolean NOT NULL DEFAULT false,
    "UserId"       uuid NOT NULL REFERENCES "Users" ("Id"),
    "SessionId"    uuid REFERENCES "Sessions" ("Id"),
    "SecretSealed" bytea NOT NULL CHECK (octet_length("SecretSealed") BETWEEN 29 AND 128),
    "ExpiresAt"    timestamptz NOT NULL,
    "ConfirmedAt"  timestamptz,
    "LastUsedStep" bigint CHECK ("LastUsedStep" >= 0)
);

CREATE UNIQUE INDEX "UX_TotpFactors_User_Confirmed" ON "TotpFactors" ("UserId")
    WHERE "ConfirmedAt" IS NOT NULL AND NOT "IsDeleted";
CREATE INDEX "IX_TotpFactors_User" ON "TotpFactors" ("UserId") WHERE NOT "IsDeleted";

CREATE TABLE "RecoveryCodes" (
    "Id"        uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt" timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt" timestamptz NOT NULL DEFAULT now(),
    "IsDeleted" boolean NOT NULL DEFAULT false,
    "UserId"    uuid NOT NULL REFERENCES "Users" ("Id"),
    "CodeHash"  bytea NOT NULL CHECK (octet_length("CodeHash") = 32),
    "UsedAt"    timestamptz
);

CREATE UNIQUE INDEX "UX_RecoveryCodes_User_CodeHash" ON "RecoveryCodes" ("UserId", "CodeHash") WHERE NOT "IsDeleted";

CREATE TABLE "Passkeys" (
    "Id"                uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"         timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"         timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"         boolean NOT NULL DEFAULT false,
    "UserId"            uuid NOT NULL REFERENCES "Users" ("Id"),
    "Name"              TEXT NOT NULL CHECK (char_length("Name") BETWEEN 1 AND 64),
    "CredentialId"      bytea NOT NULL CHECK (octet_length("CredentialId") BETWEEN 16 AND 1023),
    "PublicKey"         bytea NOT NULL CHECK (octet_length("PublicKey") BETWEEN 1 AND 4096),
    "AttestationType"   TEXT NOT NULL DEFAULT '',
    "AttestationFormat" TEXT NOT NULL DEFAULT '',
    "Aaguid"            bytea NOT NULL DEFAULT ''::bytea,
    "SignCount"         bigint NOT NULL DEFAULT 0 CHECK ("SignCount" BETWEEN 0 AND 4294967295),
    "Transports"        TEXT[] NOT NULL DEFAULT '{}',
    "Attachment"        TEXT NOT NULL DEFAULT '',
    "UserPresent"       boolean NOT NULL DEFAULT false,
    "UserVerified"      boolean NOT NULL DEFAULT false,
    "BackupEligible"    boolean NOT NULL DEFAULT false,
    "BackupState"       boolean NOT NULL DEFAULT false,
    "LastUsedAt"        timestamptz
);

CREATE UNIQUE INDEX "UX_Passkeys_CredentialId" ON "Passkeys" ("CredentialId") WHERE NOT "IsDeleted";
CREATE INDEX "IX_Passkeys_User" ON "Passkeys" ("UserId", "CreatedAt" DESC) WHERE NOT "IsDeleted";

CREATE TABLE "AuthChallenges" (
    "Id"              uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"       timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"       timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"       boolean NOT NULL DEFAULT false,
    "Kind"            TEXT NOT NULL CHECK ("Kind" IN ('mfa_pending', 'passkey_login', 'passkey_register', 'passkey_step_up')),
    "TokenHash"       bytea CHECK (octet_length("TokenHash") = 32),
    "UserId"          uuid REFERENCES "Users" ("Id"),
    "SessionId"       uuid REFERENCES "Sessions" ("Id"),
    "DeviceHash"      bytea CHECK (octet_length("DeviceHash") = 32),
    "WebauthnSession" jsonb,
    "Attempts"        integer NOT NULL DEFAULT 0 CHECK ("Attempts" >= 0),
    "ExpiresAt"       timestamptz NOT NULL,
    "UsedAt"          timestamptz,
    CHECK ("Kind" <> 'mfa_pending' OR ("TokenHash" IS NOT NULL AND "UserId" IS NOT NULL AND "DeviceHash" IS NOT NULL)),
    CHECK ("Kind" NOT IN ('passkey_register', 'passkey_step_up') OR ("UserId" IS NOT NULL AND "SessionId" IS NOT NULL))
);

CREATE UNIQUE INDEX "UX_AuthChallenges_TokenHash" ON "AuthChallenges" ("TokenHash") WHERE "TokenHash" IS NOT NULL;

-- +goose Down
DROP TABLE "AuthChallenges";
DROP TABLE "Passkeys";
DROP TABLE "RecoveryCodes";
DROP TABLE "TotpFactors";
DROP INDEX "UX_Users_WebauthnHandle";
ALTER TABLE "Users" DROP COLUMN "WebauthnHandle";
