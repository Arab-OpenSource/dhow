# Dhow

> **Ship to your own servers — safely.**
> A secure, self-hosted continuous deployment platform. Arabic-first.

**Status:** 🧱 Foundations (milestone M1 of the [roadmap](docs/roadmap.md)). `docker compose up` boots an empty,
hardened instance with the web shell and health endpoints. Sign-in, projects, runners and deployments come in the next
milestones, so Dhow is not usable for real work yet. Feedback is welcome through issues and discussions.

---

## What Dhow is

Dhow is a **self-hosted Continuous Deployment control plane**. It takes your code or a built artifact and
delivers it to infrastructure you own (VPS, on-prem servers, Docker hosts, Kubernetes, static/CDN), with:

- **Isolated runners.** Every job runs in an ephemeral, sandboxed container (microVM optional). Nothing runs on the control plane host.
- **Pipeline-as-code.** A `.dhow.yml` file in your repo, plus a visual editor that writes the YAML and opens a PR.
- **Environments and approvals.** Protection rules, required reviewers and deployment windows.
- **Instant rollback.** Atomic releases with health checks and automatic rollback.
- **Scoped, encrypted secrets.** Secrets are injected only at runtime. Workload OIDC removes the need for long-lived cloud keys.
- **Audit by default.** An append-only, tamper-evident log of who deployed what, where, and when.

Dhow **integrates with** your CI. It does not try to replace it. Build inside Dhow, or let GitHub Actions /
GitLab CI build and hand Dhow the artifact through an official action.

## Who it is for

- Developers worldwide who want a secure, open-source way to deploy to their own infrastructure.
- Small and mid-size companies and agencies running VPS fleets or on-prem servers.
- The Arabic-speaking market. The UI is designed **Arabic-first** (RTL first, LTR second), not as a mirrored English UI.

## Editions

| Edition | License | Summary |
|---|---|---|
| **Community** | AGPL-3.0 | Free for any use, including commercial. No limits on users, projects or runners. Includes all security basics. |
| **Business** | Commercial (`ee/`) | Governance for many teams: SAML/SCIM, custom roles, multi-reviewer approvals, canary/blue-green, HA, long audit retention. Priced per active user. |
| **Enterprise** | Commercial (`ee/`) | Compliance and scale: Vault/KMS, microVM isolation, air-gapped install, SIEM export, OPA policies, SLA support. |

See [docs/editions-and-licensing.md](docs/editions-and-licensing.md).

## Install

Dhow installs with one Docker Compose file: `./init.sh` generates strong secrets, then `docker compose up -d` starts an empty, hardened instance with its own PostgreSQL. Images are signed with cosign and ship with an SBOM.
See [docs/install/](docs/install/README.md) to choose a method, then the [Compose guide](docs/install/compose.md) for the quick start, upgrades, backups and signature verification.
A host needs at least 1 CPU core, 2 GiB of memory and 5 GiB of free disk (2 cores, 4 GiB and 20 GiB recommended): see [Hardware requirements](docs/requirements.md).

## Documentation

| Doc | Contents |
|---|---|
| [Architecture](docs/architecture.md) | Components, data model, job lifecycle, storage, targets, install methods |
| [Security model](docs/security-model.md) | Threat model, secure defaults, authn/authz, isolation, lessons learned |
| [Pipeline spec](docs/pipeline-spec.md) | `.dhow.yml` schema v1 (draft) |
| [UI principles](docs/ui-principles.md) | Arabic-first design, navigation model, performance budgets |
| [Editions & licensing](docs/editions-and-licensing.md) | Open-core model, feature matrix, license enforcement |
| [Install](docs/install/README.md) | Choosing a method, Docker Compose, reverse proxies, local runs, configuration, troubleshooting |
| [Hardware requirements](docs/requirements.md) | Measured CPU, memory and disk for the server, PostgreSQL and runners |
| [Development](docs/development.md) | Running Dhow from source on Linux, macOS or Windows, with or without Docker, and the tests |
| [Sign in with OIDC](docs/guides/oidc.md) | Google, Keycloak, GitHub and any OIDC provider, account linking, allowed domains |
| [Roadmap](docs/roadmap.md) | Milestones toward v1.0 |
| [ADRs](Plan/ADR/) | Every architectural and product decision, with alternatives |

## Relationship to Deploy Center

Dhow is the from-scratch successor to **Deploy Center v3**. Deploy Center receives security fixes only,
and a migration tool will import its projects into Dhow. See [ADR-0002](Plan/ADR/0002-rewrite-with-v3-hotfix.md).

## License

Core: AGPL-3.0. Code under `ee/`: commercial license. See [LICENSE.md](LICENSE.md).
