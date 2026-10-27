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

api() {
  local prefix=$1 method=$2 path=$3 body=$4
  shift 4
  curl --silent --show-error --max-time 10 --output "$prefix.body" --dump-header "$prefix.head" \
    --write-out '%{http_code}' --request "$method" --header 'Content-Type: application/json' "$@" \
    --data "$body" -- "http://127.0.0.1:$port$path"
}

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

printf 'First-owner setup, sign-in and sessions\n'
password='smoke-test passphrase 2026'
dhow_logs=$(compose logs --no-color dhow 2>&1)
setup_link=$(grep -o "$public_url/setup#[A-Za-z0-9_-]*" <<<"$dhow_logs" | tail -1)
expect "the setup link is logged exactly once" test "$(grep -c "$public_url/setup#" <<<"$dhow_logs")" = 1
expect "the setup link carries a 43-character token" test "${#setup_link}" -eq $((${#public_url} + 7 + 43))
setup_token=${setup_link#*#}
setup_body=$(printf '{"SetupToken":"%s","OwnerName":"Smoke Owner","Email":"Owner@Smoke.Test","Password":"%s","OrganizationName":"Smoke","OrganizationSlug":"smoke"}' "$setup_token" "$password")

status=$(api "$work/setup-no-origin" POST /api/v1/setup "$setup_body")
expect "setup without an Origin header answers 403" test "$status" = 403
expect "setup without an Origin header is CSRF_FAILED" grep -q '"Code":"CSRF_FAILED"' "$work/setup-no-origin.body"
status=$(api "$work/setup-bad" POST /api/v1/setup "${setup_body/$setup_token/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA}" --header "Origin: $public_url")
expect "setup with an unknown token answers 403" test "$status" = 403
status=$(api "$work/setup-weak" POST /api/v1/setup "${setup_body/$password/short}" --header "Origin: $public_url")
expect "setup with a short password answers 422" test "$status" = 422
status=$(api "$work/setup" POST /api/v1/setup "$setup_body" --header "Origin: $public_url")
expect "setup answers 200" test "$status" = 200
expect "setup stores the email lowercased" grep -q '"Email":"owner@smoke.test"' "$work/setup.body"
status=$(api "$work/setup-again" POST /api/v1/setup "$setup_body" --header "Origin: $public_url")
expect "a second setup answers 409" test "$status" = 409
status=$(http "$work/meta2" /api/v1/meta)
expect "/api/v1/meta reports that setup is done" grep -q '"SetupRequired":false' "$work/meta2.body"

login_body=$(printf '{"Email":"owner@smoke.test","Password":"%s"}' "$password")
status=$(api "$work/bad-login" POST /api/v1/sessions '{"Email":"owner@smoke.test","Password":"not the password"}' --header "Origin: $public_url")
expect "a wrong password answers 401" test "$status" = 401
expect "a wrong password is INVALID_CREDENTIALS" grep -q '"Code":"INVALID_CREDENTIALS"' "$work/bad-login.body"
expect "a wrong password sets no cookie" bash -c '! grep -qi "^set-cookie:" "$1"' _ "$work/bad-login.head"
status=$(api "$work/login" POST /api/v1/sessions "$login_body" --header "Origin: $public_url")
expect "sign-in answers 200" test "$status" = 200
for attribute in '__Host-dhow_session=' 'HttpOnly' 'Secure' 'SameSite=Strict' 'Path=/'; do
  expect "the session cookie has $attribute" has_header "$work/login.head" Set-Cookie "$attribute"
done
expect "the session cookie has no Domain" bash -c '! grep -i "^set-cookie:" "$1" | grep -qi "domain="' _ "$work/login.head"
cookie=$(sed -n 's/^[Ss]et-[Cc]ookie: \(__Host-dhow_session=[^;]*\);.*/\1/p' "$work/login.head" | tr -d '\r')
csrf=$(grep -o '"CsrfToken":"[^"]*"' "$work/login.body" | cut -d'"' -f4)
expect "sign-in returns a CSRF token" test -n "$csrf"

status=$(curl --silent --max-time 5 --output "$work/anon-me.body" --write-out '%{http_code}' -- "http://127.0.0.1:$port/api/v1/me")
expect "/api/v1/me without a session answers 401" test "$status" = 401
status=$(curl --silent --max-time 5 --output "$work/me.body" --write-out '%{http_code}' --header "Cookie: $cookie" -- "http://127.0.0.1:$port/api/v1/me")
expect "/api/v1/me with the session answers 200" test "$status" = 200
expect "/api/v1/me names the owner role" grep -q '"Role":"owner"' "$work/me.body"
status=$(api "$work/logout-no-csrf" DELETE /api/v1/sessions/current '' --header "Origin: $public_url" --header "Cookie: $cookie")
expect "sign-out without a CSRF token answers 403" test "$status" = 403
status=$(api "$work/logout-origin" DELETE /api/v1/sessions/current '' --header "Origin: https://evil.example" --header "Cookie: $cookie" --header "X-CSRF-Token: $csrf")
expect "sign-out from a foreign Origin answers 403" test "$status" = 403
status=$(api "$work/logout" DELETE /api/v1/sessions/current '' --header "Origin: $public_url" --header "Cookie: $cookie" --header "X-CSRF-Token: $csrf")
expect "sign-out with the CSRF token answers 200" test "$status" = 200
status=$(curl --silent --max-time 5 --output /dev/null --write-out '%{http_code}' --header "Cookie: $cookie" -- "http://127.0.0.1:$port/api/v1/me")
expect "the revoked session answers 401 at once" test "$status" = 401

dhow_logs=$(compose logs --no-color dhow 2>&1)
expect "the password never reaches the logs" bash -c '! grep -qF -- "$1" <<<"$2"' _ "$password" "$dhow_logs"
expect "the session cookie never reaches the logs" bash -c '! grep -qF -- "$1" <<<"$2"' _ "${cookie#*=}" "$dhow_logs"
expect "the setup token is logged only inside the setup link" test "$(grep -oF -- "$setup_token" <<<"$dhow_logs" | wc -l)" = 1

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
