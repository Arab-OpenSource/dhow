# ADR-0012: Git providers at launch

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
The users are split across providers, and self-hosted users often run Gitea or Forgejo.

## Decision
Support GitHub (App), GitLab (SaaS and self-managed), Bitbucket (Cloud and Data Center), and Gitea/Forgejo/generic Git. Webhooks use a per-repo HMAC secret plus delivery-id replay protection.

## Alternatives considered
Start with GitHub only.

## Consequences
Provider abstraction from day one. Status and check reporting is provider-specific.
