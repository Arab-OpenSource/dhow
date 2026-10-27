-- +goose Up
CREATE TABLE "Organizations" (
    "Id"        uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt" timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt" timestamptz NOT NULL DEFAULT now(),
    "IsDeleted" boolean NOT NULL DEFAULT false,
    "Name"      TEXT NOT NULL CHECK (char_length("Name") BETWEEN 1 AND 100),
    "Slug"      TEXT NOT NULL CHECK (char_length("Slug") BETWEEN 1 AND 63 AND "Slug" ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

CREATE UNIQUE INDEX "UX_Organizations_Slug" ON "Organizations" ("Slug") WHERE NOT "IsDeleted";

CREATE TABLE "Users" (
    "Id"                uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"         timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"         timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"         boolean NOT NULL DEFAULT false,
    "Email"             TEXT NOT NULL CHECK ("Email" = lower("Email") AND char_length("Email") BETWEEN 3 AND 254),
    "Name"              TEXT NOT NULL CHECK (char_length("Name") BETWEEN 1 AND 100),
    "PasswordHash"      TEXT NOT NULL,
    "PasswordChangedAt" timestamptz NOT NULL DEFAULT now(),
    "Locale"            TEXT NOT NULL DEFAULT 'ar' CHECK ("Locale" IN ('ar', 'en')),
    "Disabled"          boolean NOT NULL DEFAULT false
);

CREATE UNIQUE INDEX "UX_Users_Email" ON "Users" ("Email") WHERE NOT "IsDeleted";

CREATE TABLE "Memberships" (
    "Id"             uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"      timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"      timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"      boolean NOT NULL DEFAULT false,
    "UserId"         uuid NOT NULL REFERENCES "Users" ("Id"),
    "OrganizationId" uuid NOT NULL REFERENCES "Organizations" ("Id"),
    "Role"           TEXT NOT NULL CHECK ("Role" IN ('owner', 'admin', 'maintainer', 'developer', 'viewer'))
);

CREATE UNIQUE INDEX "UX_Memberships_User_Organization" ON "Memberships" ("UserId", "OrganizationId") WHERE NOT "IsDeleted";
CREATE INDEX "IX_Memberships_Organization" ON "Memberships" ("OrganizationId") WHERE NOT "IsDeleted";

-- +goose Down
DROP TABLE "Memberships";
DROP TABLE "Users";
DROP TABLE "Organizations";
