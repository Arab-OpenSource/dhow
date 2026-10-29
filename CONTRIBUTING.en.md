[العربية](CONTRIBUTING.md)

# Contributing to Dhow

Dhow is in the **design phase**, and design feedback is the most valuable contribution right now.

- **Discuss** a design document or ADR by opening an issue or discussion. Reference the doc and section.
- **Propose a decision change** by opening a PR that adds a new ADR in `Plan/ADR/` superseding the old one. Accepted ADRs are not edited in place.
- **Sign off your commits.** Contributions are accepted under the [Developer Certificate of Origin](https://developercertificate.org/); sign each commit with `git commit -s`.
- **Documentation language.** Public docs are Arabic first ([ADR-0064](Plan/ADR/0064-arabic-first-documentation.md)). The Arabic pages in `docs/` are the reference, and `docs/en/` mirrors them at the same paths in English. The same holds for `README.md`, `CONTRIBUTING.md` and `SECURITY.md` at the root, next to their `.en.md` versions. Internal plans and ADRs in `Plan/` are written in Arabic. User-facing copy and the Arabic docs are written in simple Arabic: Modern Standard grammar with short sentences, neither colloquial nor heavy classical (see `docs/en/ui-principles.md`).
  - **Change both versions in the same pull request**, with the same headings, tables and code blocks. Code blocks are byte-identical in both, and comments inside example code stay in English.
  - **If you cannot write Arabic,** open the pull request with the English change only and say so in its description. A maintainer adds the Arabic to the same pull request before it merges.
  - **`make docs-check` must pass.** It checks that both versions have the same files, headings, tables and code blocks, and that every link and anchor resolves. `make check` and CI run it too.
- **Security issues** must never go in public issues. See [SECURITY.md](SECURITY.en.md).

## Code contributions

- **Set up:** follow [Develop Dhow on your machine](docs/en/development.md). The quickest way needs only Git and Docker: `docker compose -f deploy/dev/compose.yaml up` runs the server and the web app with live reload. For the native tools you need Go (the version pinned in `go.mod`; `go` downloads it automatically), Node.js LTS and pnpm for `apps/web`, PostgreSQL 18 (Docker, Podman or a native install) and Python 3. `make dev-env`, `make dev-db` and `make dev` get a server running.
- **Before you push:** run `make dev-check`, which is `make check` (lint, tests with the race detector, vulnerability scan, hooks tests, docs check) with the test database set. A bare `make check` skips the database tests. For web changes also run `pnpm lint && pnpm typecheck && pnpm test && pnpm build && pnpm size` in `apps/web`.
- **Pull requests** are small and focused, follow the PR template, and must pass CI. `main` is protected; nothing merges without review.
- **Commits** follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `chore:` …), are written in English, and carry a DCO sign-off with your real name. Commit messages and pull requests must not contain AI attribution trailers or signatures.
- **Claude Code users:** the repository ships project hooks in `.claude/settings.json` that enforce these rules on every edit and git command. Do not disable them; `make hooks-test` covers them.
- **Code style:** no comments in code except `IMPORTANT:` notes and tool directives; files stay under 500 lines and functions under 100. JSON fields and database identifiers are PascalCase. See `CLAUDE.md` for the full list.
- **Tests:** new behavior comes with tests; security-relevant paths (authz, secrets, input parsing) also get negative tests.
