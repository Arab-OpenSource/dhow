-- name: GetUserForSession :one
SELECT "Id", "Email", "Name", "Locale", "Disabled", "WebauthnHandle"
FROM "Users"
WHERE "Id" = @id AND NOT "IsDeleted";

-- name: GetMfaFactors :one
SELECT
    EXISTS (SELECT 1 FROM "TotpFactors" t
            WHERE t."UserId" = @user_id AND t."ConfirmedAt" IS NOT NULL AND NOT t."IsDeleted") AS "HasTotp",
    (SELECT count(*) FROM "Passkeys" p WHERE p."UserId" = @user_id AND NOT p."IsDeleted")::bigint AS "PasskeyCount",
    (SELECT count(*) FROM "RecoveryCodes" r
     WHERE r."UserId" = @user_id AND r."UsedAt" IS NULL AND NOT r."IsDeleted")::bigint AS "RecoveryCodesRemaining";

-- name: DiscardPendingTotp :exec
UPDATE "TotpFactors"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "UserId" = @user_id AND "ConfirmedAt" IS NULL AND NOT "IsDeleted";

-- name: InsertTotpEnrollment :one
INSERT INTO "TotpFactors" ("UserId", "SessionId", "SecretSealed", "ExpiresAt", "CreatedAt", "UpdatedAt")
VALUES (@user_id, @session_id, @secret_sealed, @expires_at, @now, @now)
RETURNING "Id";

-- name: LockPendingTotp :one
SELECT "Id", "SecretSealed"
FROM "TotpFactors"
WHERE "UserId" = @user_id AND "SessionId" = @session_id AND "ConfirmedAt" IS NULL AND NOT "IsDeleted"
  AND "ExpiresAt" > @now
ORDER BY "CreatedAt" DESC, "Id" DESC
LIMIT 1
FOR UPDATE;

-- name: GetConfirmedTotp :one
SELECT "Id", "SecretSealed", "LastUsedStep", "ConfirmedAt"
FROM "TotpFactors"
WHERE "UserId" = @user_id AND "ConfirmedAt" IS NOT NULL AND NOT "IsDeleted";

-- name: AdvanceTotpStep :execrows
UPDATE "TotpFactors"
SET "LastUsedStep" = @step, "UpdatedAt" = @now
WHERE "Id" = @id AND "ConfirmedAt" IS NOT NULL AND NOT "IsDeleted"
  AND ("LastUsedStep" IS NULL OR "LastUsedStep" < @step);

-- name: RetireConfirmedTotp :execrows
UPDATE "TotpFactors"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "UserId" = @user_id AND "ConfirmedAt" IS NOT NULL AND NOT "IsDeleted";

-- name: ConfirmTotpFactor :exec
UPDATE "TotpFactors"
SET "ConfirmedAt" = @now::timestamptz, "LastUsedStep" = @step, "SessionId" = NULL, "UpdatedAt" = @now
WHERE "Id" = @id AND "ConfirmedAt" IS NULL AND NOT "IsDeleted";

-- name: RetireRecoveryCodes :execrows
UPDATE "RecoveryCodes"
SET "IsDeleted" = true, "UpdatedAt" = @now
WHERE "UserId" = @user_id AND NOT "IsDeleted";

-- name: InsertRecoveryCode :exec
INSERT INTO "RecoveryCodes" ("UserId", "CodeHash", "CreatedAt", "UpdatedAt")
VALUES (@user_id, @code_hash, @now, @now);

-- name: ConsumeRecoveryCode :execrows
UPDATE "RecoveryCodes"
SET "UsedAt" = @now::timestamptz, "UpdatedAt" = @now
WHERE "UserId" = @user_id AND "CodeHash" = @code_hash AND "UsedAt" IS NULL AND NOT "IsDeleted";

-- name: InsertAuthChallenge :one
INSERT INTO "AuthChallenges" ("Kind", "TokenHash", "UserId", "SessionId", "DeviceHash", "WebauthnSession", "ExpiresAt",
                              "CreatedAt", "UpdatedAt")
VALUES (@kind, sqlc.narg(token_hash), sqlc.narg(user_id), sqlc.narg(session_id), sqlc.narg(device_hash),
        sqlc.narg(webauthn_session), @expires_at, @now, @now)
RETURNING "Id";

-- name: FindMfaChallenge :one
SELECT "Id", "UserId", "DeviceHash", "ExpiresAt"
FROM "AuthChallenges"
WHERE "TokenHash" = @token_hash AND "Kind" = 'mfa_pending' AND NOT "IsDeleted" AND "UsedAt" IS NULL
  AND "ExpiresAt" > @now;

-- name: LockMfaChallenge :one
SELECT "Id", "UserId", "DeviceHash", "WebauthnSession", "Attempts"
FROM "AuthChallenges"
WHERE "Id" = @id AND "Kind" = 'mfa_pending' AND NOT "IsDeleted" AND "UsedAt" IS NULL AND "ExpiresAt" > @now
FOR UPDATE;

-- name: SetMfaChallengeWebauthn :execrows
UPDATE "AuthChallenges"
SET "WebauthnSession" = @webauthn_session, "UpdatedAt" = @now
WHERE "Id" = @id AND "Kind" = 'mfa_pending' AND NOT "IsDeleted" AND "UsedAt" IS NULL AND "ExpiresAt" > @now;

-- name: FailMfaChallenge :exec
UPDATE "AuthChallenges"
SET "Attempts" = "Attempts" + 1,
    "UsedAt" = CASE WHEN "Attempts" + 1 >= @max_attempts::integer THEN @now::timestamptz ELSE "UsedAt" END,
    "UpdatedAt" = @now
WHERE "Id" = @id;

-- name: UseMfaChallenge :execrows
UPDATE "AuthChallenges"
SET "UsedAt" = @now::timestamptz, "UpdatedAt" = @now
WHERE "Id" = @id AND "UsedAt" IS NULL;

-- name: TakeWebauthnChallenge :one
UPDATE "AuthChallenges"
SET "UsedAt" = @now::timestamptz, "UpdatedAt" = @now
WHERE "Id" = @id AND "Kind" = @kind AND NOT "IsDeleted" AND "UsedAt" IS NULL AND "ExpiresAt" > @now
  AND "UserId" IS NOT DISTINCT FROM sqlc.narg(user_id)
  AND "SessionId" IS NOT DISTINCT FROM sqlc.narg(session_id)
RETURNING "WebauthnSession";

-- name: PeekPendingTotp :one
SELECT "Id", "SecretSealed"
FROM "TotpFactors"
WHERE "UserId" = @user_id AND "SessionId" = @session_id AND "ConfirmedAt" IS NULL AND NOT "IsDeleted"
  AND "ExpiresAt" > @now
ORDER BY "CreatedAt" DESC, "Id" DESC
LIMIT 1;

-- name: ClearMfaReset :exec
UPDATE "Users"
SET "MfaResetAt" = NULL, "UpdatedAt" = @now
WHERE "Id" = @id AND "MfaResetAt" IS NOT NULL;
