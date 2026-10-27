# ADR-0014: Secrets: built-in envelope encryption; Vault/KMS in Enterprise

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Deploy Center stored some secrets in plaintext, returned them through the API, and derived the encryption key weakly.

## Decision
Envelope encryption with a DEK per scope (AES-256-GCM, 12-byte nonce, record-bound AAD) and a versioned KEK. Secrets are write-only through the API, decrypted only at job lease for that environment, and masked in the runner. Workload OIDC is free. Vault, cloud KMS and HSM are Enterprise.

## Alternatives considered
All integrations free; built-in only.

## Consequences
KEK backup becomes critical and is documented. Rotation re-wraps DEKs only.
