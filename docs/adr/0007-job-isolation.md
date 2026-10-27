# ADR-0007: Job isolation: rootless containers by default, microVM optional

- **Status:** Accepted (design phase)
- **Date:** 2026-10-27

## Context
Deploy Center ran pipelines on the server host with the server's privileges, so any developer effectively had full platform control.

## Decision
The control plane never executes user code. Runners execute every job in a fresh rootless Docker/Podman container: resource limits, read-only root filesystem, network egress denied by default, no host mounts. gVisor is available in all editions. Firecracker microVM as the default isolation is Enterprise.

## Alternatives considered
MicroVM always (needs KVM, excludes many VPS and non-Linux hosts); containers only (weaker boundary against hostile code).

## Consequences
A container runtime is required on runner hosts. Environments can enforce a minimum isolation level.
