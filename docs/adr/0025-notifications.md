# ADR-0025: Notification channels at launch

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Teams communicate over different tools, and WhatsApp is dominant in Egypt and the Gulf.

## Decision
Email, a generic signed webhook, Slack, Microsoft Teams, Telegram, Discord and WhatsApp Business (Cloud API). Delivery uses a dedicated queue with timeouts and an SSRF guard. Interactive approvals in Slack and Teams are Business+.

## Alternatives considered
Email + webhook only.

## Consequences
More adapters to maintain. WhatsApp requires the customer's Meta Business account.
