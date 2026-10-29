-- +goose Up
ALTER TABLE "Memberships" ADD COLUMN "LastSeenAt" timestamptz;

-- +goose Down
ALTER TABLE "Memberships" DROP COLUMN "LastSeenAt";
