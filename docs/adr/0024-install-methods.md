# ADR-0024: Official install methods

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Users range from a single small VPS to Kubernetes clusters to air-gapped networks.

## Decision
Docker Compose one-liner (the default), Helm chart, single binary + systemd, and an air-gapped signed bundle (Enterprise). The installer generates strong secrets, and the server refuses placeholder ones.

## Alternatives considered
Docker only.

## Consequences
Four install paths to test in CI. Upgrade paths are documented for each.
