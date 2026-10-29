# Contributing to Dhow

Dhow is in the **design phase**, and design feedback is the most valuable contribution right now.

- **Discuss** a design document or ADR by opening an issue or discussion. Reference the doc and section.
- **Propose a decision change** by opening a PR that adds a new ADR in `Plan/ADR/` superseding the old one. Accepted ADRs are not edited in place.
- **Sign off your commits.** Contributions are accepted under the [Developer Certificate of Origin](https://developercertificate.org/); sign each commit with `git commit -s`.
- **Documentation language.** Public docs in `docs/` are written in English. Internal plans and ADRs in `Plan/` are written in Arabic. User-facing copy is written Arabic-first, in simple Arabic: Modern Standard grammar with short sentences, neither colloquial nor heavy classical (see `docs/ui-principles.md`).
- **Security issues** must never go in public issues. See [SECURITY.md](SECURITY.md).

## Code contributions

- **Set up:** follow [Develop Dhow on your machine](docs/development.md). The quickest way needs only Git and Docker: `docker compose -f deploy/dev/compose.yaml up` runs the server and the web app with live reload. For the native tools you need Go (the version pinned in `go.mod`; `go` downloads it automatically), Node.js LTS and pnpm for `apps/web`, PostgreSQL 18 (Docker, Podman or a native install) and Python 3. `make dev-env`, `make dev-db` and `make dev` get a server running.
- **Before you push:** run `make dev-check`, which is `make check` (lint, tests with the race detector, vulnerability scan, hooks tests) with the test database set. A bare `make check` skips the database tests. For web changes also run `pnpm lint && pnpm typecheck && pnpm test && pnpm build && pnpm size` in `apps/web`.
- **Pull requests** are small and focused, follow the PR template, and must pass CI. `main` is protected; nothing merges without review.
- **Commits** follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `chore:` …), are written in English, and carry a DCO sign-off with your real name. Commit messages and pull requests must not contain AI attribution trailers or signatures.
- **Claude Code users:** the repository ships project hooks in `.claude/settings.json` that enforce these rules on every edit and git command. Do not disable them; `make hooks-test` covers them.
- **Code style:** no comments in code except `IMPORTANT:` notes and tool directives; files stay under 500 lines and functions under 100. JSON fields and database identifiers are PascalCase. See `CLAUDE.md` for the full list.
- **Tests:** new behavior comes with tests; security-relevant paths (authz, secrets, input parsing) also get negative tests.
