# ADR-0021: Arabic-first UI

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
The Arabic-speaking market is a differentiator, and mirrored-English UIs feel foreign.

## Decision
Arabic (a friendly Egyptian-leaning register) is the primary UI language. Screens are designed RTL first, then LTR. CSS logical properties only, no hardcoded strings, proper bidi isolation for code and log content. English is the second first-class language; others come via the community.

## Alternatives considered
English first, Arabic later; mirroring only.

## Consequences
Design and QA effort per locale. Copywriting is part of the definition of done.
