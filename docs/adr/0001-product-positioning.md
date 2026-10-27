# ADR-0001: Product positioning: self-hosted CD that integrates with CI

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
General-purpose CI is dominated by GitHub Actions and GitLab CI. Self-hosted *deployment* to owned infrastructure (VPS, on-prem, closed networks) with governance is underserved, especially for SMEs, agencies and the Arabic-speaking market.

## Decision
Dhow is a self-hosted Continuous Deployment control plane. It can build inside its own runners, but its core value is safe delivery: environments, approvals, atomic releases, rollback, audit. It integrates with external CI instead of competing head-on.

## Alternatives considered
Full CI/CD competitor to Actions (huge scope, crowded market); PaaS like Coolify/Vercel (simpler, but weak on governance and compliance).

## Consequences
Scope stays focused on deployment excellence. Build features (matrix, cache) exist but are not the differentiator. An official GitHub Action and a GitLab component are first-class deliverables.
