# ADR-0015: RBAC: fixed roles + Teams; custom roles paid

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Deploy Center had inconsistent, scattered authorization checks and IDOR holes.

## Decision
Fixed roles Owner/Admin/Maintainer/Developer/Viewer at org, project and environment scope, grantable to users and Teams. A single `can()` policy, tenant-scoped loaders, and route-table tests per role. Custom roles are Business+; OPA policies are Enterprise.

## Alternatives considered
Fine-grained ReBAC (e.g. OpenFGA) from day one; fixed roles only.

## Consequences
A simple, auditable model. The policy engine interface allows a richer backend later.
