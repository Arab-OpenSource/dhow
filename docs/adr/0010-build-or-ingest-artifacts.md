# ADR-0010: Build inside Dhow or ingest artifacts from external CI

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Some teams have no CI. Others already build in GitHub Actions or GitLab CI.

## Decision
Both paths produce a content-addressed artifact that deployments reference by digest. Official integrations (a GitHub Action and a GitLab CI component) upload artifacts and trigger deployments.

## Alternatives considered
Build only; ingest only.

## Consequences
Rollback redeploys an existing artifact and never rebuilds. Two ingestion paths need testing.
