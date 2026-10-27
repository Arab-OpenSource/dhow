#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || -z "$1" ]]; then
  printf 'usage: %s IMAGE\n' "$0" >&2
  exit 2
fi

image=$1
port=${DHOW_SMOKE_PORT:-18080}
public_url=https://dhow.smoke.test
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
work=$(mktemp -d)
failures=0

export COMPOSE_PROJECT_NAME=dhow-smoke
export DHOW_IMAGE=$image
export DHOW_PUBLISH=127.0.0.1:$port

compose() { docker compose --project-directory "$work" -f "$work/compose.yaml" "$@"; }

cleanup() {
  local status=$?
  if ((status != 0 || failures != 0)); then
    compose logs --no-color --tail 60 2>&1 || true
  fi
  compose down --volumes --remove-orphans --timeout 30 >/dev/null 2>&1 || true
  rm -rf -- "$work"
  exit "$status"
}
trap cleanup EXIT

ok() { printf '  ok    %s\n' "$1"; }
bad() {
  printf '  FAIL  %s\n' "$1" >&2
  failures=$((failures + 1))
}
expect() {
  local name=$1
  shift
  if "$@"; then ok "$name"; else bad "$name"; fi
}

has_header() { grep -iq -- "^$2:.*$3" "$1"; }

http() {
  local prefix=$1 path=$2
  curl --silent --show-error --max-time 5 --output "$prefix.body" --dump-header "$prefix.head" \
    --write-out '%{http_code}' -- "http://127.0.0.1:$port$path"
}

printf 'Preparing the install directory\n'
cp -- "$root/deploy/compose/compose.yaml" "$root/deploy/compose/init.sh" "$root/deploy/compose/.env.example" "$work/"
"$work/init.sh" "$public_url" >/dev/null
expect ".env is mode 600" test "$(stat -c %a "$work/.env")" = 600
expect "init.sh refuses to overwrite .env" bash -c '! "$1" >/dev/null 2>&1' _ "$work/init.sh"

printf 'The image refuses to start without configuration\n'
set +e
noenv=$(docker run --rm "$image" 2>&1)
noenv_status=$?
set -e
expect "exits non-zero without config" test "$noenv_status" -ne 0
expect "names the missing secret key" grep -q DHOW_SECRET_KEY <<<"$noenv"

printf 'Starting the stack\n'
compose up --detach --wait --wait-timeout 120
dhow_id=$(compose ps --quiet dhow)
postgres_id=$(compose ps --quiet postgres)
expect "dhow is healthy" test "$(docker inspect --format '{{.State.Health.Status}}' "$dhow_id")" = healthy
expect "postgres is healthy" test "$(docker inspect --format '{{.State.Health.Status}}' "$postgres_id")" = healthy

printf 'HTTP behaviour\n'
status=$(http "$work/ready" /api/v1/health/ready)
expect "/api/v1/health/ready answers 200" test "$status" = 200
expect "/api/v1/health/ready reports ok" grep -q '"Status":"ok"' "$work/ready.body"

status=$(http "$work/meta" /api/v1/meta)
expect "/api/v1/meta answers 200" test "$status" = 200
expect "/api/v1/meta uses the envelope" grep -q '"Success":true' "$work/meta.body"
expect "/api/v1/meta reports an empty instance" grep -q '"SetupRequired":true' "$work/meta.body"

status=$(http "$work/missing" /api/v1/does-not-exist)
expect "unknown API paths answer 404" test "$status" = 404
expect "unknown API paths use the envelope" grep -q '"Success":false' "$work/missing.body"

status=$(http "$work/web" /)
expect "/ answers 200" test "$status" = 200
expect "/ is text/html" has_header "$work/web.head" Content-Type 'text/html'
expect "/ serves the web app" grep -q '<div id="root">' "$work/web.body"

printf 'Security headers\n'
for page in web ready; do
  expect "$page: Content-Security-Policy is 'self'" has_header "$work/$page.head" Content-Security-Policy "default-src 'self'"
  expect "$page: X-Content-Type-Options" has_header "$work/$page.head" X-Content-Type-Options nosniff
  expect "$page: X-Frame-Options" has_header "$work/$page.head" X-Frame-Options DENY
  expect "$page: Referrer-Policy" has_header "$work/$page.head" Referrer-Policy no-referrer
  expect "$page: Strict-Transport-Security" has_header "$work/$page.head" Strict-Transport-Security max-age
  expect "$page: Cross-Origin-Opener-Policy" has_header "$work/$page.head" Cross-Origin-Opener-Policy same-origin
done

printf 'Hardening\n'
expect "the container user is 65532" test "$(docker inspect --format '{{.Config.User}}' "$dhow_id")" = 65532:65532
expect "the root filesystem is read-only" test "$(docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' "$dhow_id")" = true
expect "all capabilities are dropped" test "$(docker inspect --format '{{join .HostConfig.CapDrop ","}}' "$dhow_id")" = ALL
expect "dhow is published on loopback only" test "$(compose port dhow 8080)" = "127.0.0.1:$port"
expect "postgres is not published" bash -c '! docker port "$1" 2>/dev/null | grep -q .' _ "$postgres_id"
set +e
shell=$(docker exec "$dhow_id" sh -c true 2>&1)
shell_status=$?
set -e
expect "docker exec sh fails" test "$shell_status" -ne 0
expect "docker exec sh fails because there is no shell" grep -q 'executable file not found' <<<"$shell"

if ((failures != 0)); then
  printf '\n%d check(s) failed\n' "$failures" >&2
  exit 1
fi
printf '\nAll smoke checks passed\n'
