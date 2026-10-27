# Pipeline spec — `.dhow.yml` v1 (draft)

Status: **Draft**. The machine-readable JSON Schema will live in `packages/spec/` and is shared by the server, the
runner, the CLI (`dhow lint`) and the visual editor.

## 1. Principles

- **One source of truth.** The pipeline lives in the repository at the commit being deployed (ADR-0008).
  - For privileged events (deploys and PRs from forks), the definition is read from the **base/default branch**, so a PR cannot rewrite its own permissions.
- **Steps are sandboxed.** A step is either a `run:` (arguments for a shell *inside the job sandbox*) or a `uses:` (a container-image plugin).
- **Expressions are CEL** inside `${{ … }}`. They are typed, non-Turing-complete and have cost limits.
  - Values from Git (commit message, author, branch) are **data** and are never re-parsed as expressions.
- **Secrets are referenced, not embedded.** `secrets.NAME` resolves only within the job's environment scope.

## 2. Full example

```yaml
version: 1
name: web-app

on:
  push:
    branches: [main]
  tag:
    patterns: ["v*"]
  pull_request: {}            # fork PRs: no secrets, no deploy environments
  artifact:                   # external CI hands over a built artifact
    from: github-actions

defaults:
  runner:
    labels: [linux, amd64]
    isolation: container        # container | gvisor | microvm
  timeout: 20m

concurrency:
  group: ${{ project.name + "-" + git.ref }}
  cancel_in_progress: true

jobs:
  test:
    image: node:22-bookworm@sha256:<digest>
    strategy:
      matrix:
        node: ["20", "22"]
    steps:
      - name: Install
        run: npm ci
      - name: Test
        run: npm test -- --ci
    cache:
      key: ${{ "npm-" + hash_files("package-lock.json") }}
      paths: [~/.npm]

  build:
    needs: [test]
    image: node:22-bookworm@sha256:<digest>
    network:
      egress: [registry.npmjs.org]      # default is deny-all
    steps:
      - run: npm ci && npm run build
    artifacts:
      - name: web
        paths: [dist/, package.json, package-lock.json]

  deploy-staging:
    needs: [build]
    if: ${{ git.branch == "main" }}
    environment: staging
    steps:
      - uses: dhow/deploy-ssh@sha256:<digest>
        with:
          artifact: web
          target: staging-vps          # defined on the environment, credentials are env secrets
          strategy: atomic-symlink
          keep_releases: 5
          reload: systemctl reload web-app
          health_check:
            url: https://staging.example.com/healthz
            expect_status: 200
            timeout: 60s

  deploy-production:
    needs: [deploy-staging]
    if: ${{ git.is_tag && git.tag.startsWith("v") }}
    environment: production            # protection rules enforced server-side (see §4)
    steps:
      - uses: dhow/deploy-ssh@sha256:<digest>
        with:
          artifact: web
          target: prod-fleet
          strategy: atomic-symlink
          rolling: { batch: 2, pause: 30s }
          health_check:
            url: https://example.com/healthz
      - uses: dhow/notify@sha256:<digest>
        with:
          channel: release-room
          message: ${{ "Released " + git.tag + " to production" }}

  migrate-db:
    needs: [deploy-production]
    environment: production
    steps:
      - run: ./bin/migrate up
        env:
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
        id_token:
          audience: sts.amazonaws.com    # workload OIDC token instead of static cloud keys
```

## 3. Reference (v1)

| Key | Type | Notes |
|---|---|---|
| `version` | int | Must be `1` |
| `on` | map | `push`, `tag`, `pull_request`, `manual` (with typed `inputs`), `schedule` (cron), `artifact`, `api` |
| `defaults.runner` | map | `labels`, `isolation` (the minimum isolation level; the environment policy may raise it) |
| `concurrency` | map | `group` (CEL), `cancel_in_progress` |
| `jobs.<id>.needs` | list | DAG edges; cycles are rejected at validation time |
| `jobs.<id>.strategy.matrix` | map | Cartesian product; `include` and `exclude` are supported; `max_parallel` |
| `jobs.<id>.image` | string | Must be digest-pinned for production environments (enforced by policy) |
| `jobs.<id>.environment` | string | Binds protection rules, secrets and targets |
| `jobs.<id>.network.egress` | list | Host allowlist (`host`, `host:port` or `*.domain`; no IP literals; at most 64); empty means deny-all. Traffic goes through the runner's proxy (`HTTPS_PROXY` / `HTTP_PROXY`), so clients must be proxy-aware; SSH can use `ProxyCommand=/run/dhow/bin/connect %h %p`. Private, loopback, link-local and metadata addresses are always denied (ADR-0044) |
| `jobs.<id>.if` / `steps[].if` | CEL bool | — |
| `steps[].run` | string | Executed by `/bin/sh -e` **inside the sandbox** (or by `shell:`); the script is passed on stdin, and the image must provide the shell |
| `steps[].uses` | image ref | Plugin; must be digest-pinned; `with:` holds typed inputs declared by the plugin manifest |
| `steps[].env` | map | Values may reference `secrets.*` and `vars.*` |
| `artifacts` | list | Content-addressed upload; `retention` overridable down to the org policy minimum |
| `cache` | map | `key` (CEL), `paths`; scoped per project and branch, read-only for fork PRs |
| `timeout` | duration | Per job or step; hard-killed by the runner |

**CEL context.** These variables are available:
- `git.{sha, ref, branch, tag, is_tag, message, author, pr}`
- `project.*` and `env.*`
- `vars.*` (non-secret)
- `secrets.*` (only inside `env`/`with`; it can never appear in an `if`)
- `matrix.*` and `needs.<job>.outputs.*`

These functions are available: `hash_files()`, `startsWith()`, `matches()` (RE2 syntax), and `contains()`.

## 4. Environment protection (server-side, not in YAML)

Protection rules are configured on the environment in the UI or API, so a commit **cannot weaken them**. A rule can set:
- which branches or tags are allowed;
- the required reviewers (a count and which teams), with optional self-approval prevention;
- a wait timer and deployment windows, including freeze periods;
- the required isolation level, and a requirement for digest-pinned images and verified plugins;
- an admin bypass, which is recorded in the audit log.

## 5. Visual editor

The UI offers a visual pipeline editor (a jobs graph plus step forms), intended for newcomers. It works like this:
- It reads the current `.dhow.yml` at the default branch.
- It edits a typed model, validated live against the same JSON Schema.
- It **writes YAML and opens a pull request** through the Git provider integration. Dhow's copy never silently diverges from the repo.

## 6. Validation

`dhow lint` (CLI), the editor and the server all use the same Go validator. They check the schema, the DAG, CEL type errors, unknown or unpinned plugins, and policy violations.
