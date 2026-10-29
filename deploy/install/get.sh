#!/usr/bin/env bash
set -euo pipefail
umask 077

repo=${DHOW_REPO:-Khaymah/dhow}
downloads=${DHOW_DOWNLOAD_URL:-https://github.com/$repo/releases/download}
issuer=https://token.actions.githubusercontent.com
identity="^https://github\.com/${repo}/\.github/workflows/release\.yml@refs/tags/v"
version=
component=server
args=()

fail() {
  printf 'get.sh: %s\n' "$1" >&2
  exit 1
}

while (($#)); do
  case $1 in
    --version) [[ $# -ge 2 ]] || fail "--version needs a value"; version=${2#v}; shift 2 ;;
    --version=*) version=${1#*=}; version=${version#v}; shift ;;
    --component) [[ $# -ge 2 ]] || fail "--component needs a value"; component=$2; shift 2 ;;
    --component=*) component=${1#*=}; shift ;;
    *) args+=("$1"); shift ;;
  esac
done

case $component in
  server) ;;
  runner) ((${#args[@]} == 0)) || fail "--component runner takes no installer options yet: ${args[*]}" ;;
  *) fail "--component must be server or runner, got: $component" ;;
esac
[[ $downloads == https://* ]] || fail "DHOW_DOWNLOAD_URL must be an https:// address"
[[ $(uname -s) == Linux ]] || fail "only Linux is supported"
case $(uname -m) in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported architecture $(uname -m); only amd64 and arm64 are released" ;;
esac
for tool in curl tar sha256sum; do
  command -v "$tool" >/dev/null 2>&1 || fail "$tool is required"
done

if [[ -z $version ]]; then
  latest=$(curl --proto '=https' --tlsv1.2 -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")
  version=${latest##*/v}
fi
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || fail "not a release version: $version"

tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
archive="dhow-${component}_${version}_linux_${arch}.tar.gz"
for file in "$archive" SHA256SUMS SHA256SUMS.sigstore.json; do
  curl --proto '=https' --tlsv1.2 -fsSL -o "$tmp/$file" "$downloads/v$version/$file"
done

if command -v cosign >/dev/null 2>&1; then
  cosign verify-blob --bundle "$tmp/SHA256SUMS.sigstore.json" --certificate-oidc-issuer "$issuer" \
    --certificate-identity-regexp "$identity" "$tmp/SHA256SUMS" >/dev/null 2>&1 || fail "the signature on SHA256SUMS does not verify"
  printf 'get.sh: SHA256SUMS is signed by the dhow release workflow\n' >&2
else
  printf 'get.sh: warning: cosign is not installed, so only the checksum is checked. To check the signature yourself:\n' >&2
  printf '  cosign verify-blob --bundle SHA256SUMS.sigstore.json --certificate-oidc-issuer %s --certificate-identity-regexp %q SHA256SUMS\n' "$issuer" "$identity" >&2
fi
line=$(grep -E -- "^[0-9a-f]{64}  ${archive//./\\.}\$" "$tmp/SHA256SUMS") || fail "SHA256SUMS has no line for $archive"
(cd -- "$tmp" && sha256sum --check --strict --quiet - <<<"$line") || fail "$archive does not match SHA256SUMS"

as_root=()
if ((EUID != 0)); then
  command -v sudo >/dev/null 2>&1 || fail "run this as root, or install sudo"
  as_root=(sudo --)
fi

if [[ $component == server ]]; then
  tar -xzf "$tmp/$archive" -C "$tmp" dhow-server
  "${as_root[@]}" "$tmp/dhow-server" install "${args[@]}"
  exit 0
fi

tar -xzf "$tmp/$archive" -C "$tmp" dhow-runner dhow-runner.service
"${as_root[@]}" install -D -m 0755 -- "$tmp/dhow-runner" /usr/local/bin/dhow-runner
"${as_root[@]}" install -D -m 0644 -- "$tmp/dhow-runner.service" /etc/systemd/system/dhow-runner.service
if [[ -d /run/systemd/system ]]; then
  "${as_root[@]}" systemctl daemon-reload
fi
cat <<EOF
get.sh: installed dhow-runner $version:
  /usr/local/bin/dhow-runner
  /etc/systemd/system/dhow-runner.service (not enabled yet)

This script does not run 'dhow-runner install' yet; that comes in a later release.
Finish the setup as in https://github.com/$repo/blob/main/docs/runner/install.md:

  1. Create the user, its sub-ids and directories:
       sudo useradd --system --home-dir /var/lib/dhow-runner --shell /usr/sbin/nologin dhow-runner
       sudo usermod --add-subuids 1000000-2048575 --add-subgids 1000000-2048575 dhow-runner
       sudo install -d -m 0700 -o dhow-runner -g dhow-runner /var/lib/dhow-runner
       sudo install -d -m 0750 -o root -g dhow-runner /etc/dhow-runner
       sudo install -m 0640 -o root -g dhow-runner /dev/null /etc/dhow-runner/runner.env
  2. Register it with a token from the Runners page of your Dhow server:
       sudo -u dhow-runner dhow-runner register --url https://dhow.example.com --token-file -
  3. Start it:
       sudo systemctl enable --now dhow-runner
EOF
