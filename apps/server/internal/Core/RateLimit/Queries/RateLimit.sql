-- name: TakeRateLimit :one
INSERT INTO "RateLimits" ("Key", "WindowStart", "Hits", "ExpiresAt", "CreatedAt", "UpdatedAt")
VALUES (@key, @window_start, 1, @expires_at, @now, @now)
ON CONFLICT ("Key") DO UPDATE
SET "Hits" = CASE
        WHEN EXCLUDED."WindowStart" > "RateLimits"."WindowStart" THEN 1
        ELSE LEAST("RateLimits"."Hits" + 1, @ceiling::integer)
    END,
    "WindowStart" = GREATEST("RateLimits"."WindowStart", EXCLUDED."WindowStart"),
    "ExpiresAt" = GREATEST("RateLimits"."ExpiresAt", EXCLUDED."ExpiresAt"),
    "UpdatedAt" = @now
RETURNING "Hits", "ExpiresAt";
