# CLAUDE.md — Dhow

Guidance for AI agents working in this repository. Humans: start at [README.md](README.md) and [CONTRIBUTING.md](CONTRIBUTING.md).

## What this is

Dhow (داو) is a secure, self-hosted continuous deployment platform: a Go control plane, Go runners that execute
jobs in isolated sandboxes, and an Arabic-first React UI. The design lives in `docs/`; every significant decision
is an ADR in `docs/adr/`. Read the relevant ADR before changing an area, and add a new ADR instead of editing an
accepted one.

## Layout (ADR-0004)

| Path | What |
|---|---|
| `apps/server` | Control plane (Go) |
| `apps/runner` | Runner (Go) |
| `apps/web` | Web UI (React + TypeScript + Tailwind, pnpm) |
| `apps/cli` | `dhow` CLI (Go) |
| `packages/openapi` | `openapi.yaml`, the public REST contract (spec-first) |
| `packages/proto` | Runner ↔ control plane protocol (Connect/gRPC) |
| `internal/` | Go packages shared by the Go apps |
| `ee/` | Commercial features (Dhow Enterprise License); everything else is AGPL-3.0 |
| `deploy/` | Dockerfiles, Compose, Helm |
| `docs/` | All documentation. Do not put Markdown docs anywhere else (README, CLAUDE, CONTRIBUTING, SECURITY, LICENSE excepted). |

## Commands

- `make check` runs Go lint, tests (with `-race`) and govulncheck. CI runs the same targets; run them before pushing.
- `make help` lists all targets. The Go toolchain is pinned in `go.mod`; the Makefile builds tools with it.
- Web (once `apps/web` exists): `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` inside `apps/web`.

## Rules that are never broken

- **Never execute user-controlled strings through a shell.** Build commands as argv slices (`exec.Command(name, args...)`), validate refs and paths, and pass `--` before user values.
- **Never evaluate user expressions with a general-purpose interpreter.** Pipeline conditions use CEL only.
- **Secrets are write-only.** No API response, log line, event or notification may contain a secret value; test it.
- **Fail closed.** Missing or default secrets, invalid config, or failed migrations stop the process at boot.
- **Every endpoint goes through the central `can()` policy.** Add a route-table test row for every new route and role.
- **SQL only through sqlc-generated queries** with parameters. No string-built SQL.
- **No `panic` for control flow, no ignored errors, no global mutable state** outside `main`.
- **Dependencies:** add one only when it removes real complexity; pin exact versions; CI scans every PR.

## Conventions

- Go: standard library first, `log/slog` JSON logs, `context.Context` first argument, errors wrapped with `%w`, table-driven tests. Lint config: `.golangci.yml`.
- API: change `packages/openapi/openapi.yaml` first, regenerate, then implement.
- UI: Arabic is the source language and RTL is the default direction. Use logical CSS properties only. Copy follows the design system's content rules (polite Egyptian Arabic, buttons name their action).
- Commits: Conventional Commits, signed off (`git commit -s`, DCO).
- Communicate with the maintainer in Arabic; write code, comments, commits and docs in English.
