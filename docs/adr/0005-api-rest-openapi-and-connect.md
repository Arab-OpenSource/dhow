# ADR-0005: Public REST + OpenAPI; Connect/gRPC for runners; SSE for live UI

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Users, the CLI, Terraform and integrations need an approachable API. Runners need efficient streaming and typed contracts.

## Decision
Public API: REST with an OpenAPI 3.1 spec as the source of truth. Runner ↔ control plane: Connect/gRPC over HTTP/2. UI live updates: SSE authenticated by the session.

## Alternatives considered
REST only (weaker streaming for runners); Connect everywhere (harder for curl users).

## Consequences
Two contract formats to maintain (OpenAPI + protobuf). Clean separation of the public and internal surfaces.
