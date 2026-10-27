# ADR-0018: Paid editions priced per active user

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
The pricing model must be simple, familiar, and must not discourage adding runners or targets.

## Decision
Business is priced per active user per month or year. Bots and API tokens are free. Enterprise is custom.

## Alternatives considered
Per runner or target (penalizes infrastructure growth); flat per install (undercharges large customers).

## Consequences
Requires an active-user count reported in the license and admin UI.
