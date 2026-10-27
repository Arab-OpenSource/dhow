# ADR-0009: Plugins are container-image steps

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
An ecosystem of reusable steps is essential. Executing third-party code must stay inside the sandbox.

## Decision
A reusable step is an OCI image, referenced by digest, with a manifest declaring typed inputs and outputs. A community registry will have verified publishers and cosign signatures.

## Alternatives considered
Partial GitHub Actions compatibility (large surface, ongoing compatibility burden); WebAssembly plugins (immature ecosystem).

## Consequences
Plugins can be written in any language, with isolation inherited from the job. Actions compatibility may be revisited after v1.0.
