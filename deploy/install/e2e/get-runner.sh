#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || ! -f "$1/SHA256SUMS" ]]; then
  printf 'usage: %s DIST_DIR [IMAGE]\n  DIST_DIR holds dhow-runner_<version>_linux_<arch>.tar.gz and SHA256SUMS from deploy/release/dist.sh\n' "$0" >&2
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
  cp -- "$dist/$archive" "$dist/SHA256SUMS" "$work/$name/v$version/"
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
printf 'x' >>"$work/tampered/v$version/$archive"
release unlisted
grep -vF -- "$archive" "$dist/SHA256SUMS" >"$work/unlisted/v$version/SHA256SUMS" || true

openssl req -x509 -newkey ed25519 -nodes -days 1 -subj /CN=127.0.0.1 -addext subjectAltName=IP:127.0.0.1 \
  -keyout "$work/key.pem" -out "$work/ca.pem" 2>/dev/null
(cd -- "$work" && exec openssl s_server -quiet -accept "127.0.0.1:$port" -cert ca.pem -key key.pem -WWW) >/dev/null 2>&1 &
server_pid=$!
for _ in $(seq 50); do
  curl -sf --cacert "$work/ca.pem" -o /dev/null "https://127.0.0.1:$port/good/v$version/SHA256SUMS" && break
  sleep 0.2
done

get() {
  local name=$1 log=$2
  shift 2
  docker run --rm --network host --platform "linux/$arch" -v "$here/../get.sh:/get.sh:ro" -v "$work/ca.pem:/ca.pem:ro" \
    -e CURL_CA_BUNDLE=/ca.pem -e DHOW_DOWNLOAD_URL="https://127.0.0.1:$port/$name" -e NO_PROXY='*' \
    "$image" bash -c 'bash /get.sh "$@" && if [[ -e /usr/local/bin/dhow-runner ]]; then /usr/local/bin/dhow-runner version && stat -c "%a %U %n" /usr/local/bin/dhow-runner /etc/systemd/system/dhow-runner.service; fi' \
    get.sh "$@" >"$log" 2>&1
}

printf 'get.sh --component runner against a local %s release\n' "$version"
expect "a good release installs" get good "$work/good.log" --component runner --version "$version"
cat -- "$work/good.log"
expect "it warns that only the checksum was checked without cosign" grep -q 'cosign is not installed' "$work/good.log"
expect "it installs the binary 0755 root" grep -q '^755 root /usr/local/bin/dhow-runner$' "$work/good.log"
expect "it installs the unit 0644 root" grep -q '^644 root /etc/systemd/system/dhow-runner.service$' "$work/good.log"
expect "the installed runner runs" grep -q "^dhow-runner $version " "$work/good.log"
expect "it says it does not run dhow-runner install yet" grep -q "does not run 'dhow-runner install' yet" "$work/good.log"
expect "it prints the register step" grep -q 'dhow-runner register --url' "$work/good.log"

expect "a tampered archive is refused" fails get tampered "$work/tampered.log" --component runner --version "$version"
expect "the refusal names the checksum" grep -q 'does not match SHA256SUMS' "$work/tampered.log"
expect "nothing is installed from a tampered archive" lacks 'installed dhow-runner' "$work/tampered.log"
expect "an archive missing from SHA256SUMS is refused" fails get unlisted "$work/unlisted.log" --component runner --version "$version"
expect "the refusal says the line is missing" grep -q 'SHA256SUMS has no line for' "$work/unlisted.log"
expect "the server stays the default component" get good "$work/server.log" --version "$version" --yes
expect "the server path runs dhow-server install with the options" grep -q '^dhow-server install --yes$' "$work/server.log"
expect "an unknown component is refused" fails get good "$work/component.log" --component agent --version "$version"
expect "installer options are refused for the runner" fails get good "$work/options.log" --component runner --version "$version" --yes

if ((failures != 0)); then
  printf '\n%d get.sh check(s) failed\n' "$failures" >&2
  exit 1
fi
printf '\nAll get.sh runner checks passed\n'
