-- name: EnsureWebauthnHandle :one
UPDATE "Users"
SET "WebauthnHandle" = COALESCE("WebauthnHandle", @handle), "UpdatedAt" = @now
WHERE "Id" = @id AND NOT "IsDeleted"
RETURNING "WebauthnHandle";

-- name: GetUserByWebauthnHandle :one
SELECT "Id", "Email", "Name", "Locale", "Disabled", "WebauthnHandle"
FROM "Users"
WHERE "WebauthnHandle" = @handle AND NOT "IsDeleted";

-- name: ListUserPasskeys :many
SELECT "Id", "CreatedAt", "Name", "CredentialId", "PublicKey", "AttestationType", "AttestationFormat", "Aaguid",
       "SignCount", "Transports", "Attachment", "UserPresent", "UserVerified", "BackupEligible", "BackupState",
       "LastUsedAt"
FROM "Passkeys"
WHERE "UserId" = @user_id AND NOT "IsDeleted"
ORDER BY "CreatedAt" DESC, "Id" DESC;

-- name: InsertPasskey :one
INSERT INTO "Passkeys" ("UserId", "Name", "CredentialId", "PublicKey", "AttestationType", "AttestationFormat", "Aaguid",
                        "SignCount", "Transports", "Attachment", "UserPresent", "UserVerified", "BackupEligible",
                        "BackupState", "CreatedAt", "UpdatedAt")
VALUES (@user_id, @name, @credential_id, @public_key, @attestation_type, @attestation_format, @aaguid, @sign_count,
        @transports, @attachment, @user_present, @user_verified, @backup_eligible, @backup_state, @now, @now)
RETURNING "Id", "CreatedAt";

-- name: RecordPasskeyUse :execrows
UPDATE "Passkeys"
SET "SignCount" = @sign_count, "UserVerified" = @user_verified, "BackupState" = @backup_state, "LastUsedAt" = @now::timestamptz,
    "UpdatedAt" = @now
WHERE "Id" = @id AND NOT "IsDeleted" AND "SignCount" = @previous_sign_count;

-- name: RenamePasskey :one
UPDATE "Passkeys"
SET "Name" = @name, "UpdatedAt" = @now
WHERE "Id" = @id AND "UserId" = @user_id AND NOT "IsDeleted"
RETURNING "Id", "CreatedAt", "Name", "Transports", "BackupEligible", "LastUsedAt";

-- name: RemovePasskey :execrows
UPDATE "Passkeys"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "Id" = @id AND "UserId" = @user_id AND NOT "IsDeleted";
