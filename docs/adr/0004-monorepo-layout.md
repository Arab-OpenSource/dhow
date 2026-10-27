# ADR-0004: Single monorepo

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Deploy Center split server and client into separate repos. The v3 client source was never committed while its built bundle shipped.

## Decision
One repository: `apps/{server,runner,web,cli}`, `packages/{proto,spec,openapi,ui}`, `ee/`, `deploy/`, `docs/`. All artifacts are built in CI from source. Built assets are never committed.

## Alternatives considered
Separate repos per component.

## Consequences
Atomic cross-component changes and one CI. A polyglot toolchain (Go + pnpm) in one repo.
