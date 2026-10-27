# Security model

Status: **Draft v0.2**. Security is the product: every design choice below is a default, not an option.

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
| **Network attacker** | MITM between components or against targets | TLS everywhere. Runner identity keys are pinned, and the control plane signs every lease grant with a key the runner pinned at registration, so a compromised proxy cannot inject jobs. Secrets are sealed end to end to the runner's key (HPKE). SSH host keys and TLS fingerprints are pinned per target. Webhooks are verified with HMAC and replay protection |
| **Stolen session or token** | Holds a user's cookie or API token | Server-side sessions with an idle and an absolute timeout. A session is revoked at once by sign-out, password change, MFA change, role change or an admin action. Cookies are `HttpOnly`, `Secure` and `SameSite=Strict`, and every state-changing request needs a CSRF token. High-risk actions require re-authentication. API tokens are scoped and expire |
| **Unauthenticated internet attacker** | Can reach the public endpoints | Registration is closed. Every route requires authentication by default. Rate limits apply per IP *and* per account. There is no unauthenticated realtime channel |
| **Malicious plugin author** | Publishes a step image | Plugins run inside the job sandbox with the job's privileges only. Plugins are referenced by digest. The registry marks verified publishers and requires signatures (cosign) |

## 3. Secure defaults

- **Registration.** Public registration is **closed**. The first owner account is created through a one-time setup URL. After that, users join by invitation or via SSO/OIDC (with optional domain allowlists).
- **Fail closed on weak configuration.** The server **refuses to boot** with missing, placeholder, short or reused secrets. Real environment variables take precedence over config files.
- **TLS** is required outside localhost. HSTS is on. Cookies are `Secure`, `HttpOnly` and `SameSite=Strict`.
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

**Lockout.** Lockout counters are per account and per IP, with exponential backoff (details in [section 5](#5-sessions-and-tokens-adr-0033)). Error messages are generic and responses take constant time.

## 5. Sessions and tokens (ADR-0033)

The web UI and the API are served from the same origin, so browsers use **server-side sessions** with a cookie. There is no JWT for browsers: a JWT would still need a server-side lookup on every request to be revocable, and it would live in JavaScript.

- **Cookie.** `__Host-dhow_session` with `HttpOnly`, `Secure`, `SameSite=Strict` and `Path=/`, and no `Domain`. The `__Host-` prefix makes browsers refuse a cookie set by a sub-domain or over plain HTTP. `Secure` is always set, because the public URL is HTTPS by design.
- **Session id.** 32 random bytes. Only its SHA-256 hash is stored in `"Sessions"`, so a database leak does not yield usable cookies.
- **Lifetime.** A session ends after **12 hours without activity** (idle timeout) or **14 days** after sign-in (absolute timeout), whichever comes first. `LastSeenAt` is written at most once a minute, so reads do not turn into writes.
- **Rotation.** Signing in always issues a new session id and revokes the session the request carried. The id is also replaced whenever a user's privileges rise (role changes arrive with M2.3).
- **Revocation is immediate.** Every request checks the session row, with no cache. Sign-out, password change, MFA change, role change, an admin "sign out everywhere" and "sign out of every other device" all revoke sessions server-side. A disabled or deleted user loses every session at once.
- **CSRF.**
  - Every request that is not `GET`, `HEAD` or `OPTIONS` needs an `X-CSRF-Token` header and an `Origin` header equal to `DHOW_PUBLIC_URL`. Otherwise the answer is `403 CSRF_FAILED`.
  - The token is an HMAC of the session id under a key derived from `DHOW_SECRET_KEY`. It is bound to one session, `GET /api/v1/me` returns it, and the server stores only its hash.
  - Sign-in and setup have no session yet, so they skip the token but still check `Origin`.
  - Responses of the API are never cacheable (`Cache-Control: no-store`).
- **Passwords.** Argon2id (m=19 MiB, t=2, p=1, 16-byte salt, 32-byte key), stored as a PHC string and compared in constant time. A hash made with older parameters is replaced after the next successful sign-in. The length is 12 to 128 characters.
  - **Breached passwords.** When a password is set or changed, the server asks the Have I Been Pwned range API. Only the first 5 hex characters of the password's SHA-1 leave the server (k-anonymity), padding is requested, and the call times out after 3 seconds. If the service cannot be reached, the change goes through and a warning is logged. `DHOW_BREACHED_PASSWORD_CHECK=false` turns the check off for isolated networks.
- **Sign-in throttling.** Failures are counted per account and per client address. After 5 failures for an account, or 20 for an address, the key is locked for 30 seconds, and every further failure doubles the lock up to 15 minutes. A key forgets its failures after an hour without one, and a successful sign-in resets the account. Attempts are counted before the password is checked, in one transaction that locks both counters, so a burst of parallel guesses gets no more attempts than a slow one. Attempts made during a lock are refused without being counted, so nobody can extend a lock. A locked key answers with the same generic `INVALID_CREDENTIALS` error plus `Retry-After`. Unknown accounts are throttled exactly like real ones, and the unknown-account path still runs a full Argon2id verification, so neither the answer nor its timing shows whether an account exists.
- **CLI, CI and integrations** use **API tokens**, never a session. They are prefixed (`dhow_pat_…`), so secret scanners can detect them. They are stored as HMAC-SHA256 with a server pepper, scoped to an org or project with explicit permissions, and must expire. A token can never exceed its creator's permissions.

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

- **Isolation (ADR-0007, ADR-0043).** Jobs only run inside a rootless container or microVM, with limits, a read-only root filesystem, network egress denied by default, and no host mounts. The control plane never executes user code.
  - The default sandbox is rootless Podman with all capabilities dropped, `no-new-privileges`, a seccomp profile that also blocks nested user namespaces, `io_uring`, `bpf`, `ptrace` and `mount`, and a separate user-namespace range per job.
  - A runner refuses to start if cgroup v2 delegation for CPU, memory and PIDs is missing, and it fails the job if the created container does not match the policy.
  - An escape test suite (host files, runner credentials, other jobs, metadata endpoints, container sockets, privilege escalation, network) is a required CI check.
- **Network egress (ADR-0044).** A sandbox has no network interface besides loopback. Allowed hosts are reached only through a per-job proxy in the runner. The proxy resolves names itself, connects to the address it resolved, and always denies private, loopback, link-local, CGNAT and cloud metadata ranges. Only the runner operator can open a private range.
- **Runner plane (ADR-0041, ADR-0042).**
  - Runners authenticate with pinned Ed25519 keys and 15-minute access tokens.
  - Starting a job needs a fresh signature.
  - Every mutating call carries the attempt's fencing token.
  - Session cookies are rejected on the runner plane, and runner tokens are rejected on the public API.
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

- The audit log is append-only, and each event includes `PrevHash`, which makes the log **tamper-evident**.
  - Every event stores `Hash = SHA-256(PrevHash ‖ canonical JSON of the event)`, and events form one chain per organization (plus one for instance-level events). Writers take a per-chain advisory lock, so the chain never forks.
  - A database trigger rejects `UPDATE`, `DELETE` and `TRUNCATE` on the table. `GET /api/v1/organizations/{id}/audit-events/verify` walks a chain and reports the first event whose hash or link is wrong.
  - Event data never carries a secret: passwords, tokens and cookies are not recorded, and keys that look sensitive are dropped before the write.
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
| Stateless long-lived JWTs could not be revoked | Server-side sessions stored as hashes, checked on every request, revoked at once, with idle and absolute timeouts and CSRF tokens |
| In-place `rsync --delete` deploys with fragile backups | Atomic releases (symlink switch), health checks, automatic rollback to the previous release |
| Queue retries re-ran whole, non-idempotent deployments | No automatic retry after the first mutating step. Leases and reconciliation handle crashed jobs, and an environment lock prevents concurrent deployments |
| Notification URLs allowed SSRF and blocked the single worker | A separate notification queue with timeouts, plus an SSRF guard that checks the resolved IP |
| An idempotency cache was not bound to the user and replayed cookies | Idempotency keys are scoped to the authenticated principal. Auth endpoints and `Set-Cookie` are never cached |
| Coverage gates excluded files that were never loaded, and integration tests bypassed the real middleware stack | Coverage is measured over all packages. E2E tests run against the real server binary, and CI fails if a required service is missing |
