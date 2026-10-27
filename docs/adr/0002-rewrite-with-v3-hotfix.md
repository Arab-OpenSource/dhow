# ADR-0002: Rewrite from scratch, with a v3 security hotfix in parallel

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
The Deploy Center v3 audit found architectural issues (host execution, shell-string commands, non-revocable sessions, unauthenticated realtime) that cannot be fixed incrementally without rewriting most of the system.

## Decision
Build Dhow from scratch. Ship a Deploy Center v3.0.1 hotfix that closes the critical issues, then freeze v3 to security fixes only. Provide a v3 → Dhow importer.

## Alternatives considered
Evolve v3 incrementally (keeps legacy constraints); rewrite and abandon v3 (leaves current users exposed).

## Consequences
Two codebases for a while. Knowledge transfers, code does not. Existing users remain protected.
