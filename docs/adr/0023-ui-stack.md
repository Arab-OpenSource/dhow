# ADR-0023: UI stack: React + Tailwind + shadcn/ui + Motion

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Deploy Center's MUI UI was heavy and generic-looking. Full design control, accessibility and small bundles are needed.

## Decision
React + Vite + TypeScript, Tailwind (logical utilities), shadcn/ui on Radix (components owned in `packages/ui`), Motion, TanStack Query, and an OpenAPI-generated client. Performance budgets are enforced in CI.

## Alternatives considered
MUI v7; Mantine; SolidJS/Svelte.

## Consequences
We own and maintain our components. Strong accessibility and RTL support.
