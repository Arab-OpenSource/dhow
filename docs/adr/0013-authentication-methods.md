# ADR-0013: Authentication methods

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Strong authentication must be free. SSO for enterprises is a common paid driver.

## Decision
Community: password (Argon2id), TOTP, Passkeys/WebAuthn and OIDC login. Business+: SAML 2.0, SCIM and group sync. MFA is bound to a server-side challenge from the password step.

## Alternatives considered
Everything free (removes the main paid driver); password + TOTP only in Community.

## Consequences
Phishing-resistant login is available to everyone.
