# Architecture Decision Records

| ADR | Title |
|---|---|
| [0001](0001-product-positioning.md) | Product positioning: self-hosted CD that integrates with CI |
| [0002](0002-rewrite-with-v3-hotfix.md) | Rewrite from scratch, with a v3 security hotfix in parallel |
| [0003](0003-go-for-server-and-runner.md) | Go for the control plane and the runner |
| [0004](0004-monorepo-layout.md) | Single monorepo |
| [0005](0005-api-rest-openapi-and-connect.md) | Public REST + OpenAPI; Connect/gRPC for runners; SSE for live UI |
| [0006](0006-postgres-default-redis-optional.md) | PostgreSQL by default; Redis optional |
| [0007](0007-job-isolation.md) | Job isolation: rootless containers by default, microVM optional |
| [0008](0008-pipeline-as-code-with-visual-editor.md) | Pipeline-as-code (`.dhow.yml`) with a visual editor |
| [0009](0009-container-image-plugins.md) | Plugins are container-image steps |
| [0010](0010-build-or-ingest-artifacts.md) | Build inside Dhow or ingest artifacts from external CI |
| [0011](0011-deploy-targets.md) | v1.0 deploy targets |
| [0012](0012-git-providers.md) | Git providers at launch |
| [0013](0013-authentication-methods.md) | Authentication methods |
| [0014](0014-secrets-management.md) | Secrets: built-in envelope encryption; Vault/KMS in Enterprise |
| [0015](0015-rbac-model.md) | RBAC: fixed roles + Teams; custom roles paid |
| [0016](0016-multi-org.md) | Multiple organizations per installation from day one |
| [0017](0017-licensing-agpl-and-ee.md) | AGPL-3.0 core + commercial `ee/` |
| [0018](0018-pricing-per-active-user.md) | Paid editions priced per active user |
| [0019](0019-no-community-limits.md) | No numeric limits in Community |
| [0020](0020-managed-cloud-later.md) | Managed control plane after self-hosted stabilizes |
| [0021](0021-arabic-first-ui.md) | Arabic-first UI |
| [0022](0022-navigation-model.md) | Single workspace + slide-over panels + command palette |
| [0023](0023-ui-stack.md) | UI stack: React + Tailwind + shadcn/ui + Motion |
| [0024](0024-install-methods.md) | Official install methods |
| [0025](0025-notifications.md) | Notification channels at launch |
| [0026](0026-product-name.md) | Product name: Dhow |
| [0027](0027-public-repo-from-day-one.md) | Public repository from day one |
