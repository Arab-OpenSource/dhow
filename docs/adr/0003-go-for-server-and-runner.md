# ADR-0003: Go for the control plane and the runner

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
The product is self-hosted infrastructure software. It needs single-binary distribution, low memory use, strong concurrency, and an attractive language for DevOps contributors. The runner and server must share the YAML parser, the CEL evaluator and the protocol code.

## Decision
Write both `apps/server` and `apps/runner` (and the CLI) in Go. The web UI stays TypeScript, with types generated from OpenAPI.

## Alternatives considered
TypeScript + Hono on Node (shared types with the UI, but two backend languages and a heavier runtime); Hono on Bun (fast, but less mature for long-running production infra); NestJS (heavier, more boilerplate).

## Consequences
Static binaries for all platforms and a shared `packages/spec`. The team must be fluent in Go. No direct type sharing with the UI (codegen instead).
