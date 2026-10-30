[العربية](SECURITY.md)

# Security Policy

## Reporting a vulnerability

**Do not open public issues for security problems.**

Report privately through **GitHub Security Advisories** for this repository (Security tab → "Report a vulnerability").

When reporting:
- Include the affected version or commit, the impact, and the steps to reproduce.
- Expect an acknowledgement within 3 business days.
- We aim to agree on a fix and disclosure timeline with you, typically within 90 days. Reporters are credited unless they prefer otherwise.

## Scope

The scope is the Dhow control plane, runner, CLI, official plugins, install artifacts and the docs site.

## Host kernel advisories

Some kernel flaws reach Dhow through the host it runs on. [Kernel security advisories](docs/en/security/kernel-advisories.md) lists them, what Dhow changed, and how to check and protect the host. The first one is CVE-2026-98432 (multipath TCP).

## Supported versions

There are no released versions yet; Dhow is in the design phase. A support table will be added at the first release.
