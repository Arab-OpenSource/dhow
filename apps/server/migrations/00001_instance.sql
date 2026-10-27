-- +goose Up
CREATE TABLE "Instance" (
    "Id"               smallint    PRIMARY KEY DEFAULT 1 CHECK ("Id" = 1),
    "CreatedAt"        timestamptz NOT NULL DEFAULT now(),
    "SetupCompletedAt" timestamptz
);

INSERT INTO "Instance" ("Id") VALUES (1);

-- +goose Down
DROP TABLE "Instance";
