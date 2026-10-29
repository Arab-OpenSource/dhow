-- +goose Up
CREATE INDEX "IX_Sessions_User_LastSeen" ON "Sessions" ("UserId", "LastSeenAt" DESC) WHERE NOT "IsDeleted";

-- +goose Down
DROP INDEX "IX_Sessions_User_LastSeen";
