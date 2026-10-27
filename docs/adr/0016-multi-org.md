# ADR-0016: Multiple organizations per installation from day one

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Agencies manage many clients, and a future managed cloud needs tenancy.

## Decision
Every resource is scoped to an Organization. Tenant isolation is enforced in repository functions and tested.

## Alternatives considered
A single org per install.

## Consequences
Slightly more complex queries and UI. No costly retrofit later.
