-- +goose Up
CREATE INDEX "IX_Jobs_Active" ON "Jobs" ("OrganizationId", "State")
    WHERE "State" IN ('queued', 'leased', 'running') AND NOT "IsDeleted";
CREATE INDEX "IX_Jobs_Failed_Finished" ON "Jobs" ("OrganizationId", "FinishedAt")
    WHERE "State" IN ('failed', 'timed_out') AND NOT "IsDeleted";

-- +goose Down
DROP INDEX "IX_Jobs_Failed_Finished";
DROP INDEX "IX_Jobs_Active";
