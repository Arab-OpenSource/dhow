#!/usr/bin/env bash
set -euo pipefail
umask 077

repo=${DHOW_REPO:-Khaymah/dhow}
issuer=https://token.actions.githubusercontent.com
identity="^https://github\.com/${repo}/\.github/workflows/release\.yml@refs/tags/v"
version=
args=()

fail() {
  printf 'get.sh: %s\n' "$1" >&2
  exit 1
}

while (($#)); do
  case $1 in
    --version) [[ $# -ge 2 ]] || fail "--version needs a value"; version=${2#v}; shift 2 ;;
    --version=*) version=${1#*=}; version=${version#v}; shift ;;
    *) args+=("$1"); shift ;;
  esac
done

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
archive="dhow-server_${version}_linux_${arch}.tar.gz"
for file in "$archive" SHA256SUMS SHA256SUMS.sigstore.json; do
  curl --proto '=https' --tlsv1.2 -fsSL -o "$tmp/$file" "https://github.com/$repo/releases/download/v$version/$file"
done

if command -v cosign >/dev/null 2>&1; then
  cosign verify-blob --bundle "$tmp/SHA256SUMS.sigstore.json" --certificate-oidc-issuer "$issuer" \
    --certificate-identity-regexp "$identity" "$tmp/SHA256SUMS" >/dev/null 2>&1 || fail "the signature on SHA256SUMS does not verify"
  printf 'get.sh: SHA256SUMS is signed by the dhow release workflow\n' >&2
else
  printf 'get.sh: warning: cosign is not installed, so only the checksum is checked. To check the signature yourself:\n' >&2
  printf '  cosign verify-blob --bundle SHA256SUMS.sigstore.json --certificate-oidc-issuer %s --certificate-identity-regexp %q SHA256SUMS\n' "$issuer" "$identity" >&2
fi
(cd -- "$tmp" && grep -F -- "  $archive" SHA256SUMS | sha256sum --check --strict --quiet -) || fail "$archive does not match SHA256SUMS"

tar -xzf "$tmp/$archive" -C "$tmp" dhow-server
runner=()
if ((EUID != 0)); then
  command -v sudo >/dev/null 2>&1 || fail "run this as root, or install sudo"
  runner=(sudo --)
fi
"${runner[@]}" "$tmp/dhow-server" install "${args[@]}"
