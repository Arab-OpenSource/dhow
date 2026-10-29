-- +goose Up
CREATE TABLE "OidcProviders" (
    "Id"             uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"      timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"      timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"      boolean NOT NULL DEFAULT false,
    "ProviderId"     TEXT NOT NULL CHECK ("ProviderId" ~ '^[a-z][a-z0-9]{0,31}$'),
    "Type"           TEXT NOT NULL CHECK ("Type" IN ('oidc', 'github')),
    "Name"           TEXT NOT NULL CHECK (char_length("Name") BETWEEN 1 AND 40),
    "Issuer"         TEXT NOT NULL CHECK (char_length("Issuer") BETWEEN 1 AND 2048),
    "ClientId"       TEXT NOT NULL CHECK (octet_length("ClientId") BETWEEN 1 AND 512),
    "AllowedDomains" TEXT[] NOT NULL DEFAULT '{}' CHECK (cardinality("AllowedDomains") <= 50),
    "Secret"         bytea CHECK ("Secret" IS NULL OR octet_length("Secret") BETWEEN 29 AND 4096),
    "Version"        bigint NOT NULL DEFAULT 1 CHECK ("Version" >= 1),
    "CreatedBy"      uuid REFERENCES "Users" ("Id"),
    "UpdatedBy"      uuid REFERENCES "Users" ("Id")
);

CREATE UNIQUE INDEX "UX_OidcProviders_ProviderId" ON "OidcProviders" ("ProviderId") WHERE NOT "IsDeleted";
CREATE INDEX "IX_UserIdentities_Provider" ON "UserIdentities" ("Provider") WHERE NOT "IsDeleted";

-- +goose Down
DROP INDEX "IX_UserIdentities_Provider";
DROP TABLE "OidcProviders";
