#!/usr/bin/env bash
set -euo pipefail
umask 077

dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
env_file="$dir/.env"
public_url="${1:-${DHOW_PUBLIC_URL:-https://dhow.localhost}}"

fail() {
  printf 'init.sh: %s\n' "$1" >&2
  exit 1
}

if [[ -e "$env_file" || -L "$env_file" ]]; then
  fail "$env_file already exists and will not be overwritten. Move it away first if you really want new secrets: they cannot decrypt data stored with the old ones."
fi
command -v openssl >/dev/null 2>&1 || fail "openssl is required to generate secrets"
if [[ ! "$public_url" =~ ^https://[^[:space:]/?#@]+(/[^[:space:]?#]*)?$ ]]; then
  fail "the public URL must be an absolute https URL without credentials, a query or a fragment, got: $public_url"
fi

secret_key=$(openssl rand -hex 32)
postgres_password=$(openssl rand -hex 24)

(
  set -o noclobber
  {
    printf 'POSTGRES_PASSWORD=%s\n' "$postgres_password"
    printf 'DHOW_SECRET_KEY=%s\n' "$secret_key"
    printf 'DHOW_PUBLIC_URL=%s\n' "$public_url"
  } >"$env_file"
)
chmod 600 "$env_file"

printf 'Wrote %s (mode 600).\n' "$env_file"
printf 'Public URL: %s\n' "$public_url"
printf 'Back up DHOW_SECRET_KEY: without it the stored secrets cannot be decrypted.\n'
printf 'Next: docker compose up -d\n'
