# ADR-0017: AGPL-3.0 core + commercial `ee/`

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Deploy Center's personal/commercial license blocked adoption at work, which is exactly where paying customers come from.

## Decision
The core is AGPL-3.0, free for any use including commercial. The `ee/` directory is under a commercial license, gated by an offline-verified license file. Contributions are under the DCO. Whether a CLA is needed is an open question.

## Alternatives considered
FSL (source-available, converts to Apache after 2 years); Apache-2.0 + ee; keep the current license.

## Consequences
Genuinely open source. SaaS free-riding is deterred by the AGPL. Clear separation of paid code.
