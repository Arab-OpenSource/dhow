-- +goose Up
ALTER TABLE "OidcFlows" ADD COLUMN "Issuer" TEXT NOT NULL DEFAULT '' CHECK (char_length("Issuer") <= 2048);

-- +goose Down
ALTER TABLE "OidcFlows" DROP COLUMN "Issuer";
