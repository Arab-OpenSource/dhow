-- name: TakeRateLimit :one
INSERT INTO "RateLimits" ("Key", "WindowStart", "Hits", "ExpiresAt", "CreatedAt", "UpdatedAt")
VALUES (
    sqlc.arg(key),
    date_bin(sqlc.arg(window_seconds)::float8 * interval '1 second', COALESCE(sqlc.narg(now)::timestamptz, now()), timestamptz '2000-01-01 00:00:00+00'),
    1,
    date_bin(sqlc.arg(window_seconds)::float8 * interval '1 second', COALESCE(sqlc.narg(now)::timestamptz, now()), timestamptz '2000-01-01 00:00:00+00')
        + sqlc.arg(window_seconds)::float8 * interval '1 second',
    COALESCE(sqlc.narg(now)::timestamptz, now()),
    COALESCE(sqlc.narg(now)::timestamptz, now())
)
ON CONFLICT ("Key") DO UPDATE
SET "Hits" = CASE
        WHEN EXCLUDED."WindowStart" > "RateLimits"."WindowStart" THEN 1
        ELSE LEAST("RateLimits"."Hits" + 1, sqlc.arg(ceiling)::integer)
    END,
    "WindowStart" = GREATEST("RateLimits"."WindowStart", EXCLUDED."WindowStart"),
    "ExpiresAt" = GREATEST("RateLimits"."ExpiresAt", EXCLUDED."ExpiresAt"),
    "UpdatedAt" = EXCLUDED."UpdatedAt"
RETURNING "Hits", "ExpiresAt", "UpdatedAt" AS "Now";
