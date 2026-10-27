# ADR-0011: v1.0 deploy targets

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
The target users run VPS fleets, Docker hosts, Kubernetes, and static or shared hosting.

## Decision
v1.0 ships SSH/VPS (atomic symlink releases, health checks, auto-rollback), Docker/Compose hosts, Kubernetes (Helm/Kustomize/manifests with rollout status) and Static/CDN/SFTP. Host keys and TLS fingerprints are pinned per target.

## Alternatives considered
Ship SSH only first.

## Consequences
A larger v1.0 scope, but it matches the market. Every target needs E2E tests covering failure and rollback.
