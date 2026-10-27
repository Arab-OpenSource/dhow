# Security model

Status: **Draft v0.1**. Security is the product: every design choice below is a default, not an option.

## 1. Assets

The assets to protect are:
- Deployment targets: production servers and clusters.
- Secrets: environment secrets, target credentials and the KEK.
- Source code and artifacts.
- The audit trail.
- User accounts and sessions.

## 2. Threat model

| Actor | Capability | Primary controls |
|---|---|---|
| **Malicious or compromised developer** | Can push code and edit `.dhow.yml` in repos they can write to | Sandboxed runners (no host access); environment protection rules and approvals; secrets scoped per environment; branch restrictions per environment; audit |
| **External contributor (PR from a fork)** | Can open PRs, so attacker-controlled code reaches CI | Fork PR jobs run with **no secrets** and are restricted to non-deploy environments. Running them needs maintainer approval, and the pipeline definition always comes from the base branch for privileged events |
| **Compromised runner host** | Full control of one runner | A runner only receives secrets for jobs leased to it. A runner identity can be revoked instantly. Runners can be pinned to environments through labels and pools. Runners never get control-plane credentials or database access |
| **Network attacker** | MITM between components or against targets | TLS everywhere. Runner identity keys are pinned. SSH host keys and TLS fingerprints are pinned per target. Webhooks are verified with HMAC and replay protection |
| **Stolen session or token** | Holds a user's cookie or API token | 15-minute access tokens. Refresh tokens rotate, and reuse is detected. Sessions are revocable server-side. High-risk actions require re-authentication. API tokens are scoped and expire |
| **Unauthenticated internet attacker** | Can reach the public endpoints | Registration is closed. Every route requires authentication by default. Rate limits apply per IP *and* per account. There is no unauthenticated realtime channel |
| **Malicious plugin author** | Publishes a step image | Plugins run inside the job sandbox with the job's privileges only. Plugins are referenced by digest. The registry marks verified publishers and requires signatures (cosign) |

## 3. Secure defaults

- **Registration.** Public registration is **closed**. The first owner account is created through a one-time setup URL. After that, users join by invitation or via SSO/OIDC (with optional domain allowlists).
- **Fail closed on weak configuration.** The server **refuses to boot** with missing, placeholder, short or reused secrets. Real environment variables take precedence over config files.
- **TLS** is required outside localhost. HSTS is on. Cookies are `Secure`, `HttpOnly` and `SameSite=Lax/Strict`.
- **Strict CSP:** `default-src 'self'`, no inline script, `frame-ancestors 'none'`, Trusted Types.
- **No secret values in any API response, log line, error message or notification.**

## 4. Authentication (ADR-0013)

| Method | Edition |
|---|---|
| Password with Argon2id, breached-password check (k-anonymity, optional offline list) and max length | Community |
| TOTP. Enrolling or changing it requires the current factor. Replay protection stores the last accepted step | Community |
| **Passkeys (WebAuthn)**, the phishing-resistant method and the recommended default | Community |
| OIDC login (GitHub, Google, Keycloak, any OIDC IdP) | Community |
| SAML 2.0, SCIM provisioning, group→team sync | Business+ |

**Login flow.** The password step only yields a short-lived, single-use `mfa_pending` challenge that is bound to the user and the device. The second factor must present that challenge. A user id taken from the client is never trusted.

**Lockout.** Lockout counters are per account and per IP, with exponential backoff. Error messages are generic and responses take constant time.

## 5. Sessions and tokens

- The **access token** is short-lived (15 minutes) and carries a session id (`sid`). The server checks the session state on every request, using a cached lookup.
- The **refresh token** is opaque, stored hashed, **rotated on every use**, and **reuse detection** revokes the whole session family.
- **Global revocation.** Logout, password change, MFA change, role change and an admin "sign out everywhere" all revoke sessions server-side.
- **API tokens** are prefixed (`dhow_pat_…`), so secret scanners can detect them. They are stored as HMAC-SHA256 with a server pepper, scoped to an org or project with explicit permissions, and must expire. A token can never exceed its creator's permissions.

## 6. Authorization (ADR-0015)

- A **single policy function**, `can(subject, action, resource)`, is the only path to authorization.
  - Handlers declare the action and resource, and middleware enforces them.
  - Resource loaders are tenant-scoped, so a missing check results in "not found", not a data leak.
- **Fixed roles** apply per scope (Organization → Project → Environment):

| Role | Summary |
|---|---|
| Owner | Billing/license, org deletion, everything |
| Admin | Manage org, members, runners, all projects |
| Maintainer | Manage project settings, environments, protection rules, secrets |
| Developer | Trigger deployments to environments their rules allow; read logs |
| Viewer | Read-only on assigned projects |

- **Teams** group users, and roles can be granted to teams.
- **Custom roles and OPA policies** come from `ee/`.
- **Testing:** a test walks the **entire route table** for every role and scope and asserts the expected allow or deny result. A route without a declared policy fails the build.

## 7. Execution safety

- **Isolation (ADR-0007).** Jobs only run inside a rootless container or microVM, with limits, a read-only root filesystem, network egress denied by default, and no host mounts. The control plane never executes user code.
- **No shell strings in platform code.**
  - Every process the platform starts (git, ssh, rsync, docker, kubectl) uses argv arrays.
  - User-controlled values are validated against strict grammars: refs go through `git check-ref-format`, SHAs must match `^[0-9a-f]{40}$`, and URLs go through a parser with a scheme allowlist.
  - Those values are passed after `--`.
- **Expressions use CEL.** CEL is non-Turing-complete, evaluates against typed variables, has cost limits, and never performs string-concatenation evaluation.
- **No regex "WAF" as a security control.** Correctness comes from parameterized queries, typed validation and output encoding.

## 8. Data protection

- Secrets use envelope encryption (see [architecture §6](architecture.md#6-secrets)): AES-256-GCM with a random 12-byte nonce, AAD bound to the record, and versioned keys with rotation.
- Webhook secrets, target credentials and API-token peppers are all treated as secrets.
- Masking happens in the runner, on the stream, before any output leaves the host.
- Retention policies apply to logs, artifacts and audit events. Export and deletion tooling supports GDPR-style requests.

## 9. Audit

- The audit log is append-only, and each event includes `prev_hash`, which makes the log **tamper-evident**.
- Every security-relevant action is recorded, including auth events, role changes, secret writes, approvals, deployments and runner registration.
- **Retention:** Community keeps 90 days. Business and Enterprise get longer retention, export, and SIEM streaming.

## 10. Supply chain

- Releases are reproducible builds, **signed with cosign** (keyless), and published with an **SBOM** (SPDX) and SLSA provenance.
- Container images are distroless and non-root, and are referenced by digest in the Helm and Compose manifests.
- **CI:**
  - It is **required** for merge: tests, lint with zero warnings allowed, type checks, `govulncheck`/`npm audit` blocking on high and critical, CodeQL, secret scanning and license checks.
  - Dependabot updates are merged only on green CI.
- An external penetration test is required before GA, and again yearly.
- The disclosure policy is in [`SECURITY.md`](../SECURITY.md).

## 11. Lessons from the Deploy Center v3 audit

Each class of vulnerability found in Deploy Center v3, and the design rule in Dhow that removes it:

| v3 issue class | Dhow design rule |
|---|---|
| Public registration accepted a client-supplied role (anyone could become admin) | Registration is closed by default. Request bodies are decoded into typed DTOs with **no role field**. Roles change only through an authorized, audited endpoint |
| The realtime socket had no authentication and broadcast full records containing secrets | Realtime runs over **SSE behind the same session auth**. Every stream is authorized per resource. Payloads are explicit DTOs and never ORM objects. There are no global broadcasts |
| The 2FA step trusted a client-supplied user id, so the password step could be skipped | Server-held `mfa_pending` challenge, bound to the password step; lockout per account |
| Pipelines executed directly on the server host | The control plane never executes user code. Jobs run only in sandboxed runners |
| Commands were built by string interpolation and run through a shell | argv-only process execution, with strict validation of refs, SHAs, URLs and options |
| Conditions were evaluated with dynamic code construction | CEL, with typed inputs and cost limits |
| The API returned webhook secrets and credentials in plaintext | Secrets are write-only. Response DTOs contain no secret fields. Everything is encrypted at rest |
| Routes lacked object-level checks (IDOR) | A central `can()` policy, tenant-scoped loaders, non-enumerable UUIDv7 ids, and route-table authorization tests |
| Hardcoded fallback secrets were accepted in production | Boot fails on missing or placeholder secrets. The installer generates strong ones |
| Stateless long-lived JWTs could not be revoked | 15-minute access tokens checked against server-side sessions; refresh rotation with reuse detection |
| In-place `rsync --delete` deploys with fragile backups | Atomic releases (symlink switch), health checks, automatic rollback to the previous release |
| Queue retries re-ran whole, non-idempotent deployments | No automatic retry after the first mutating step. Leases and reconciliation handle crashed jobs, and an environment lock prevents concurrent deployments |
| Notification URLs allowed SSRF and blocked the single worker | A separate notification queue with timeouts, plus an SSRF guard that checks the resolved IP |
| An idempotency cache was not bound to the user and replayed cookies | Idempotency keys are scoped to the authenticated principal. Auth endpoints and `Set-Cookie` are never cached |
| Coverage gates excluded files that were never loaded, and integration tests bypassed the real middleware stack | Coverage is measured over all packages. E2E tests run against the real server binary, and CI fails if a required service is missing |
