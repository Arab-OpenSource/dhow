-- +goose Up
CREATE TABLE "SetupTokens" (
    "Id"        uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt" timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt" timestamptz NOT NULL DEFAULT now(),
    "IsDeleted" boolean NOT NULL DEFAULT false,
    "TokenHash" bytea NOT NULL CHECK (octet_length("TokenHash") = 32),
    "ExpiresAt" timestamptz NOT NULL,
    "UsedAt"    timestamptz
);

CREATE UNIQUE INDEX "UX_SetupTokens_TokenHash" ON "SetupTokens" ("TokenHash");

-- +goose Down
DROP TABLE "SetupTokens";
