# ADR-0008: Pipeline-as-code (`.dhow.yml`) with a visual editor

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Deploy Center stored pipelines as JSON edited through forms. That gave no review, no history and no diff.

## Decision
The pipeline is a `.dhow.yml` in the repo. The visual editor reads it, edits a typed model, and writes YAML through a pull request. Privileged events read the definition from the base branch. Environment protection lives server-side.

## Alternatives considered
YAML only (harder for beginners); UI-stored and YAML side by side (two sources of truth).

## Consequences
One source of truth with an approachable UX. The editor depends on the Git provider write integration.
