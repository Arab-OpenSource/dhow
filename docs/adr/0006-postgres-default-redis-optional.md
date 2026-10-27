# ADR-0006: PostgreSQL by default; Redis optional

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Every extra stateful component makes self-hosting harder (operations, backups). Deploy Center required MariaDB plus Redis.

## Decision
PostgreSQL is the only required database. It also serves as the job queue (`SKIP LOCKED`), the lock service (advisory locks) and the event bus (`LISTEN/NOTIFY`). Redis can optionally be enabled for pub/sub and rate limits at high scale. Object storage (S3/MinIO, or local disk) holds logs and artifacts.

## Alternatives considered
Postgres + required Redis; keep MariaDB.

## Consequences
Simpler installs and backups. Both the Redis and non-Redis paths must be tested.
