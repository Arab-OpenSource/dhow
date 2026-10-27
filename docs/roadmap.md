# Roadmap to v1.0

Status: **Draft**. Team: Sabry + Claude.

This plan builds the **complete v1.0 product** described in these docs, not a cut-down MVP. Milestones are
ordered, and each one ends in a working, tested, demoable state. There are no calendar dates. Effort is given in
rough T-shirt sizes (S < M < L < XL).

| # | Milestone | Scope | Exit criteria | Effort |
|---|---|---|---|---|
| M0 | **Design sign-off** | Design system (brand color, Arabic and Latin type, tokens, core components, key screens in RTL and LTR, dark and light); architecture, security model, pipeline spec, editions; ADRs accepted | Sabry approves the design system and ADRs | M |
| M1 | **Foundations** | Monorepo, required CI (tests, lint, vuln scanning, CodeQL, SBOM, signing), Go server skeleton, Postgres migrations with locking, config validation (fail closed), OpenAPI pipeline, web app shell (workspace, panels, ⌘K, i18n ar/en) | `docker compose up` boots an empty, secure instance | L |
| M2 | **Identity & tenancy** | Owner setup flow, invitations, password + TOTP + Passkeys + OIDC, sessions with rotation and revocation, orgs, teams, fixed roles, `can()` policy + route-table tests, API tokens, audit log | All authz tests green; external review of the auth flows | L |
| M3 | **Runner & isolation** | Runner binary, registration and identity, leasing over Connect, rootless container sandbox with limits and network policy, gVisor option, log streaming (seq + SSE), cancellation, reconciliation, Helm chart for the server and runners | A job runs isolated; killing the runner mid-job reconciles correctly | XL |
| M4 | **Pipelines** | `.dhow.yml` schema + validator + CEL, DAG/matrix, artifacts, cache, container-image plugins, `dhow lint` CLI, Git providers (GitHub App, GitLab, Bitbucket, Gitea/Forgejo/generic), fork-PR policy | Real repos on all four providers run multi-job pipelines | XL |
| M5 | **Secrets & environments** | Envelope encryption, KEK rotation, scoped secrets, runner-side masking, workload OIDC issuer, environments + protection rules + approvals | Secrets never appear in API, logs or notifications (tested); approval flow works end to end | L |
| M6 | **Deploy targets** | SSH/VPS atomic releases + health checks + auto-rollback, Docker/Compose, Kubernetes, Static/CDN/SFTP, external-CI artifact ingestion (official GitHub Action and GitLab component) | Each target deploys, fails a health check, and auto-rolls back in an E2E test | XL |
| M7 | **Product UI** | Full workspace: overview, deployment panels, virtualized log viewer, visual pipeline editor (writes YAML, opens a PR), approvals, settings, notifications (Email, webhook, Slack, Teams, Telegram, Discord, WhatsApp) | Performance budgets met; WCAG 2.2 AA audit passed; Arabic copy reviewed | XL |
| M8 | **Editions** | License file + entitlements, `ee/` features (SAML/SCIM, custom roles, multi-reviewer, windows, HA, canary, Vault/KMS, microVM, SIEM, air-gapped bundle) | The paid features work behind entitlements; expiry degrades gracefully | XL |
| M9 | **Hardening** | Threat-model review, fuzzing (YAML, CEL, webhooks), load tests, chaos tests (runner and DB loss), backup and restore drill, external penetration test, fix all findings | No open critical or high findings | L |
| M10 | **Public beta** | Install methods hardened for beta (the Compose stack, the binary with the interactive installer, and Helm ship earlier), docs site (ar/en), Deploy Center v3 importer, community channels | First external teams in production | M |
| M11 | **v1.0 GA** | Stabilization, upgrade guarantees, signed releases, security advisory process | GA release | M |

**In parallel, before M1:** the Deploy Center **v3.0.1 security hotfix** for existing users (ADR-0002).

**After GA:** the managed control plane (ADR-0020), a runner autoscaler, a plugin registry with verified publishers, a Terraform provider, and more languages.
