# ADR-0020: Managed control plane after self-hosted stabilizes

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
A hosted control plane with customer-run runners is a strong revenue line, but it is costly to operate safely.

## Decision
The architecture is cloud-ready from day one (multi-org, outbound runners, scoped secrets). The service launches after self-hosted v1.0 is stable.

## Alternatives considered
Launch the cloud at v1.0; never offer a cloud.

## Consequences
There is no cloud revenue early on, but the team stays focused.
