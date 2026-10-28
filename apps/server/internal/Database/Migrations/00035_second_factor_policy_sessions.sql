-- +goose Up
ALTER TABLE "Organizations"
    ADD COLUMN "RequireSecondFactor" TEXT NOT NULL DEFAULT 'off'
        CHECK ("RequireSecondFactor" IN ('off', 'admins', 'everyone')),
    ADD COLUMN "RequireSecondFactorSince" timestamptz,
    ADD CONSTRAINT "CK_Organizations_SecondFactorSince"
        CHECK (("RequireSecondFactor" = 'off') = ("RequireSecondFactorSince" IS NULL));

-- +goose Down
ALTER TABLE "Organizations"
    DROP CONSTRAINT "CK_Organizations_SecondFactorSince",
    DROP COLUMN "RequireSecondFactorSince",
    DROP COLUMN "RequireSecondFactor";
