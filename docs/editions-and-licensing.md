# Editions and licensing

Status: **Draft**. Decisions: ADR-0017 to ADR-0020.

## 1. Principle

> **Community = one team can run Dhow in production, securely and without limits.**
> **Paid = an organization needs governance, compliance, scale or support across many teams.**

**Security basics are never paywalled.** Every edition includes 2FA, Passkeys, encryption, isolation, basic RBAC, the audit log, security fixes, backups and upgrades.

## 2. Licensing model (ADR-0017)

- **Core:** AGPL-3.0.
  - Companies may use it commercially, for free, without restriction.
  - Anyone who offers a modified Dhow as a network service must publish their changes.
- **`ee/` directory:** commercial license. Its source is visible in the same repository, but using it requires a valid license file.
- **Contributions:** made under the DCO (Developer Certificate of Origin). *Open question:* whether a CLA is needed to keep relicensing options. Decide this before accepting external contributions to `ee/`.

## 3. Feature matrix

| Capability | Community | Business | Enterprise |
|---|:-:|:-:|:-:|
| Users, projects, runners, deployments | Unlimited | Unlimited | Unlimited |
| Isolated runners (rootless containers, gVisor) | ✅ | ✅ | ✅ |
| Firecracker microVM isolation | — | — | ✅ |
| `.dhow.yml` + visual editor, matrix, artifacts, cache | ✅ | ✅ | ✅ |
| All targets (SSH, Docker, Kubernetes, Static/CDN/SFTP) | ✅ | ✅ | ✅ |
| Atomic releases, health checks, auto-rollback | ✅ | ✅ | ✅ |
| Password + TOTP + Passkeys + OIDC login | ✅ | ✅ | ✅ |
| SAML 2.0, SCIM, group→team sync | — | ✅ | ✅ |
| Fixed roles + Teams | ✅ | ✅ | ✅ |
| Custom roles | — | ✅ | ✅ |
| OPA policies (deploy policy-as-code) | — | — | ✅ |
| Environments + protection rules (branches, single reviewer) | ✅ | ✅ | ✅ |
| Multiple required reviewers, deployment windows, freezes | — | ✅ | ✅ |
| Canary / blue-green / rolling with analysis | Rolling | ✅ | ✅ |
| Built-in envelope-encrypted secrets, workload OIDC | ✅ | ✅ | ✅ |
| HashiCorp Vault / AWS, GCP, Azure KMS / HSM | — | — | ✅ |
| Audit log | 90 days | 1 year + export | Custom retention + SIEM streaming + tamper-evidence verification tooling |
| High availability (multiple control-plane replicas) | — | ✅ | ✅ |
| Air-gapped install bundle | — | — | ✅ |
| Notifications: Email, webhook, Slack, Teams, Telegram, Discord, WhatsApp | ✅ | ✅ | ✅ |
| Interactive approvals in Slack and Teams | — | ✅ | ✅ |
| Support | Community | Email, next business day | 24/7 SLA, onboarding, Arabic support |

The matrix is a draft. The rule for moving any line is the principle in §1.

## 4. Pricing (ADR-0018, ADR-0019)

- **Business:** priced per **active user** per month, billed monthly or yearly. An active user is anyone who signed in or triggered an action during the billing period. Bots and API tokens are free.
- **Enterprise:** custom contracts.
- **Community** has **no numeric limits** at all. The difference between editions is capabilities only.

## 5. License enforcement

- A **license file** is a JSON payload signed with Ed25519. It contains the org, edition, seat count, expiry and feature flags. The public key is embedded in the binary, so verification works **offline** and never needs to phone home. This matters for air-gapped customers.
- Code checks entitlements in **one place**: `entitlements.Has(feature.MultiReviewerApprovals)`. Feature code never parses license files.
- **Expiry and overage:**
  - The admin gets warnings 30 days before expiry, then a 14-day grace period.
  - After that, paid features **degrade to Community behaviour**. Existing configuration is kept, read-only.
  - Dhow **never stops deployments, never locks users out, and never deletes data** because of licensing.
- **Enforcement philosophy:** we accept that the check can be patched out. Dhow targets honest organizations. Real enforcement is the license terms plus the value of support and updates, not DRM.

## 6. Telemetry

- Telemetry is **opt-in**, anonymous and aggregate: version, edition, approximate counts and feature usage. It never includes code, names, hostnames or secrets.
- Exactly what is sent is documented, and admins can see the full payload in the UI.

## 7. Managed cloud (ADR-0020)

Dhow will later offer a **hosted control plane, with runners on the customer's infrastructure** (the Buildkite model). It will launch only after the self-hosted product is stable. The architecture already supports it:
- multi-org;
- outbound-only runners;
- secrets decrypted only for leased jobs.

## 8. Revenue lines (priority order)

1. Business and Enterprise licenses for self-hosted installs.
2. The managed control plane (later).
3. Services: onboarding, migration from Jenkins or custom scripts, and Arabic-language support.
