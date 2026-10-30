#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || ! -f "$1/SHA256SUMS" ]]; then
  printf 'usage: %s DIST_DIR [IMAGE]\n  DIST_DIR holds dhow-runner_<version>_linux_<arch>.tar.gz, optionally dhow_<version>_linux_<arch>.tar.gz, and SHA256SUMS from deploy/release/dist.sh\n' "$0" >&2
  exit 2
fi

dist=$(cd -- "$1" && pwd)
image=${2:-quay.io/fedora/fedora:latest}
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
port=${GET_SH_TEST_PORT:-18444}
archive=$(cd -- "$dist" && find . -maxdepth 1 -name 'dhow-runner_*_linux_*.tar.gz' -printf '%f\n' | head -1)
[[ -n $archive ]] || {
  printf 'get-runner.sh: no dhow-runner archive in %s\n' "$dist" >&2
  exit 2
}
version=${archive#dhow-runner_}
version=${version%_linux_*}
arch=${archive##*_}
arch=${arch%.tar.gz}
cli_archive="dhow_${version}_linux_${arch}.tar.gz"
archives=("$archive")
if [[ -f "$dist/$cli_archive" ]]; then
  archives+=("$cli_archive")
else
  cli_archive=
fi
work=$(mktemp -d)
server_pid=
failures=0

cleanup() {
  local status=$?
  if [[ -n $server_pid ]]; then kill "$server_pid" 2>/dev/null || true; fi
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
  local what=$1
  shift
  if "$@"; then ok "$what"; else bad "$what"; fi
}
fails() { ! "$@"; }
lacks() { ! grep -q -- "$1" "$2"; }

release() {
  local name=$1
  mkdir -p "$work/$name/v$version"
  local a
  for a in "${archives[@]}"; do
    cp -- "$dist/$a" "$work/$name/v$version/"
  done
  cp -- "$dist/SHA256SUMS" "$work/$name/v$version/"
  printf '{}\n' >"$work/$name/v$version/SHA256SUMS.sigstore.json"
}
release good
mkdir -p "$work/server"
printf '#!/bin/sh\nprintf "dhow-server %%s\\n" "$*"\n' >"$work/server/dhow-server"
chmod 0755 "$work/server/dhow-server"
server_archive="dhow-server_${version}_linux_${arch}.tar.gz"
tar -C "$work/server" -czf "$work/good/v$version/$server_archive" dhow-server
(cd -- "$work/good/v$version" && sha256sum -- "$server_archive" >>SHA256SUMS)
release tampered
for a in "${archives[@]}"; do
  printf 'x' >>"$work/tampered/v$version/$a"
done
release unlisted
grep -vF -e "$archive" ${cli_archive:+-e "$cli_archive"} "$dist/SHA256SUMS" >"$work/unlisted/v$version/SHA256SUMS" || true

openssl req -x509 -newkey ed25519 -nodes -days 1 -subj /CN=127.0.0.1 -addext subjectAltName=IP:127.0.0.1 \
  -keyout "$work/key.pem" -out "$work/ca.pem" 2>/dev/null
(cd -- "$work" && exec openssl s_server -quiet -accept "127.0.0.1:$port" -cert ca.pem -key key.pem -WWW) >/dev/null 2>&1 &
server_pid=$!
for _ in $(seq 50); do
  curl -sf --cacert "$work/ca.pem" -o /dev/null "https://127.0.0.1:$port/good/v$version/SHA256SUMS" && break
  sleep 0.2
done

run_get() {
  local user=$1 name=$2 log=$3
  shift 3
  docker run --rm --network host --platform "linux/$arch" -v "$here/../get.sh:/get.sh:ro" -v "$work/ca.pem:/ca.pem:ro" \
    -v "$here/get-runner-box.sh:/box.sh:ro" -e CURL_CA_BUNDLE=/ca.pem -e DHOW_DOWNLOAD_URL="https://127.0.0.1:$port/$name" \
    -e NO_PROXY='*' -e GET_AS="$user" "$image" bash /box.sh "$@" >"$log" 2>&1
}
get() { run_get root "$@"; }
skip=--insecure-skip-signature

printf 'get.sh --component runner against a local %s release\n' "$version"
expect "a mirror without cosign is refused" fails get good "$work/nocosign.log" --component runner --version "$version"
expect "the refusal asks for cosign" grep -q 'cosign is required when DHOW_DOWNLOAD_URL or DHOW_REPO is set' "$work/nocosign.log"
expect "nothing is installed without the signature" lacks 'installed dhow-runner' "$work/nocosign.log"
expect "a good release installs with --insecure-skip-signature" get good "$work/good.log" --component runner --version "$version" "$skip"
cat -- "$work/good.log"
expect "the opt-out prints a loud warning" grep -q 'WARNING: --insecure-skip-signature' "$work/good.log"
expect "it installs the binary 0755 root" grep -q '^755 root /usr/local/bin/dhow-runner$' "$work/good.log"
expect "it installs the unit 0644 root" grep -q '^644 root /etc/systemd/system/dhow-runner.service$' "$work/good.log"
expect "the installed runner runs" grep -q "^dhow-runner $version " "$work/good.log"
expect "it points to dhow-runner install" grep -q '^  sudo dhow-runner install$' "$work/good.log"
expect "it no longer lists the manual steps" lacks 'useradd' "$work/good.log"

expect "a user with sudo installs from a root-owned copy" run_get user good "$work/user.log" --component runner --version "$version" "$skip"
expect "the user install puts root-owned files in place" grep -q '^755 root /usr/local/bin/dhow-runner$' "$work/user.log"
expect "the root-owned staging directory is removed" grep -q '^no staging directory left$' "$work/user.log"

expect "a tampered archive is refused" fails get tampered "$work/tampered.log" --component runner --version "$version" "$skip"
expect "the refusal names the checksum" grep -q 'does not match SHA256SUMS' "$work/tampered.log"
expect "nothing is installed from a tampered archive" lacks 'installed dhow-runner' "$work/tampered.log"
expect "an archive missing from SHA256SUMS is refused" fails get unlisted "$work/unlisted.log" --component runner --version "$version" "$skip"
expect "the refusal says the line is missing" grep -q 'SHA256SUMS has no line for' "$work/unlisted.log"
expect "the server stays the default component" get good "$work/server.log" --version "$version" --yes "$skip"
expect "the server path runs dhow-server install with the options" grep -q '^dhow-server install --yes$' "$work/server.log"
expect "an unknown component is refused" fails get good "$work/component.log" --component agent --version "$version" "$skip"
expect "installer options are refused for the runner" fails get good "$work/options.log" --component runner --version "$version" --yes "$skip"

if [[ -n $cli_archive ]]; then
  printf '\nget.sh --component cli against a local %s release\n' "$version"
  expect "a mirror without cosign is refused for the CLI" fails get good "$work/cli-nocosign.log" --component cli --version "$version"
  expect "the CLI refusal asks for cosign" grep -q 'cosign is required when DHOW_DOWNLOAD_URL or DHOW_REPO is set' "$work/cli-nocosign.log"
  expect "the CLI is not installed without the signature" lacks 'installed dhow ' "$work/cli-nocosign.log"
  expect "a good CLI release installs with --insecure-skip-signature" get good "$work/cli.log" --component cli --version "$version" "$skip"
  cat -- "$work/cli.log"
  expect "the CLI opt-out prints a loud warning" grep -q 'WARNING: --insecure-skip-signature' "$work/cli.log"
  expect "it installs dhow 0755 root" grep -q '^755 root /usr/local/bin/dhow$' "$work/cli.log"
  expect "the installed CLI runs" grep -q "^dhow $version " "$work/cli.log"
  expect "the installed CLI lints a bad file with exit code 1" grep -q '^dhow lint exit 1$' "$work/cli.log"
  expect "the CLI path installs no runner" lacks 'dhow-runner' "$work/cli.log"
  expect "a user with sudo installs the CLI from a root-owned copy" run_get user good "$work/cli-user.log" --component cli --version "$version" "$skip"
  expect "the user install puts a root-owned dhow in place" grep -q '^755 root /usr/local/bin/dhow$' "$work/cli-user.log"
  expect "the CLI staging directory is removed" grep -q '^no staging directory left$' "$work/cli-user.log"
  expect "a tampered CLI archive is refused" fails get tampered "$work/cli-tampered.log" --component cli --version "$version" "$skip"
  expect "the CLI refusal names the checksum" grep -q 'does not match SHA256SUMS' "$work/cli-tampered.log"
  expect "nothing is installed from a tampered CLI archive" lacks 'installed dhow ' "$work/cli-tampered.log"
  expect "a CLI archive missing from SHA256SUMS is refused" fails get unlisted "$work/cli-unlisted.log" --component cli --version "$version" "$skip"
  expect "the CLI refusal says the line is missing" grep -q 'SHA256SUMS has no line for' "$work/cli-unlisted.log"
  expect "other options are refused for the CLI" fails get good "$work/cli-options.log" --component cli --version "$version" --yes "$skip"
fi

if ((failures != 0)); then
  printf '\n%d get.sh check(s) failed\n' "$failures" >&2
  exit 1
fi
printf '\nAll get.sh checks passed\n'
