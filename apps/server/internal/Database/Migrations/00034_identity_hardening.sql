-- +goose Up
CREATE TABLE "RateLimits" (
    "Id"          uuid PRIMARY KEY DEFAULT uuidv7(),
    "CreatedAt"   timestamptz NOT NULL DEFAULT now(),
    "UpdatedAt"   timestamptz NOT NULL DEFAULT now(),
    "IsDeleted"   boolean NOT NULL DEFAULT false,
    "Key"         TEXT NOT NULL CHECK (char_length("Key") BETWEEN 1 AND 200),
    "WindowStart" timestamptz NOT NULL,
    "Hits"        integer NOT NULL CHECK ("Hits" >= 1),
    "ExpiresAt"   timestamptz NOT NULL CHECK ("ExpiresAt" > "WindowStart")
);

CREATE UNIQUE INDEX "UX_RateLimits_Key" ON "RateLimits" ("Key");
CREATE INDEX "IX_RateLimits_ExpiresAt" ON "RateLimits" ("ExpiresAt");

-- +goose Down
DROP TABLE "RateLimits";
