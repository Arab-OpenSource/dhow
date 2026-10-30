-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE "Jobs" VALIDATE CONSTRAINT "CK_Jobs_OriginKind";
ALTER TABLE "Jobs" VALIDATE CONSTRAINT "CK_Jobs_RequiredFeatures";
ALTER TABLE "Jobs" VALIDATE CONSTRAINT "CK_Jobs_Origin";
ALTER TABLE "ApiTokens" VALIDATE CONSTRAINT "ApiTokens_Scopes_check";
CREATE INDEX CONCURRENTLY IF NOT EXISTS "IX_Jobs_Origin" ON "Jobs" ("OriginId", "CreatedAt" DESC)
    WHERE "OriginId" IS NOT NULL AND NOT "IsDeleted";

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS "IX_Jobs_Origin";
