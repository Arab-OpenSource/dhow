#!/usr/bin/env bash
set -euo pipefail
umask 077

default_repo=Khaymah/dhow
repo=${DHOW_REPO:-$default_repo}
downloads=${DHOW_DOWNLOAD_URL:-https://github.com/$repo/releases/download}
custom_source=false
if [[ -n ${DHOW_DOWNLOAD_URL:-} || $repo != "$default_repo" ]]; then
  custom_source=true
fi
skip_signature=false
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
    --insecure-skip-signature) skip_signature=true; shift ;;
    *) args+=("$1"); shift ;;
  esac
done

case $component in
  server) ;;
  runner) ((${#args[@]} == 0)) || fail "--component runner takes no installer options yet: ${args[*]}" ;;
  cli) ((${#args[@]} == 0)) || fail "--component cli takes no other options: ${args[*]}" ;;
  *) fail "--component must be server, runner or cli, got: $component" ;;
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

as_root=()
if ((EUID != 0)); then
  command -v sudo >/dev/null 2>&1 || fail "run this as root, or install sudo"
  as_root=(sudo --)
fi
tmp=$(mktemp -d)
stage=
cleanup() {
  rm -rf -- "$tmp"
  if [[ -n $stage && $stage != "$tmp" ]]; then
    "${as_root[@]}" rm -rf -- "$stage"
  fi
}
trap cleanup EXIT
archive="dhow-${component}_${version}_linux_${arch}.tar.gz"
if [[ $component == cli ]]; then
  archive="dhow_${version}_linux_${arch}.tar.gz"
fi
for file in "$archive" SHA256SUMS SHA256SUMS.sigstore.json; do
  curl --proto '=https' --tlsv1.2 -fsSL -o "$tmp/$file" "$downloads/v$version/$file"
done

stage=$tmp
if ((EUID != 0)); then
  stage=$("${as_root[@]}" mktemp -d)
  "${as_root[@]}" chmod 0755 "$stage"
  for file in "$archive" SHA256SUMS SHA256SUMS.sigstore.json; do
    "${as_root[@]}" install -m 0644 -- "$tmp/$file" "$stage/$file"
  done
fi

if [[ $skip_signature == true ]]; then
  printf 'get.sh: WARNING: --insecure-skip-signature: the signature on SHA256SUMS is NOT checked.\n' >&2
  printf 'get.sh: WARNING: whoever serves these files decides what runs as root on this machine.\n' >&2
elif command -v cosign >/dev/null 2>&1; then
  cosign verify-blob --bundle "$stage/SHA256SUMS.sigstore.json" --certificate-oidc-issuer "$issuer" \
    --certificate-identity-regexp "$identity" "$stage/SHA256SUMS" >/dev/null 2>&1 || fail "the signature on SHA256SUMS does not verify"
  printf 'get.sh: SHA256SUMS is signed by the dhow release workflow\n' >&2
elif [[ $custom_source == true ]]; then
  fail "cosign is required when DHOW_DOWNLOAD_URL or DHOW_REPO is set, because the checksum alone cannot tell a mirror's files from the release; install cosign, or pass --insecure-skip-signature if you trust this source"
else
  printf 'get.sh: warning: cosign is not installed, so only the checksum is checked. To check the signature yourself:\n' >&2
  printf '  cosign verify-blob --bundle SHA256SUMS.sigstore.json --certificate-oidc-issuer %s --certificate-identity-regexp %q SHA256SUMS\n' "$issuer" "$identity" >&2
fi
line=$(grep -E -- "^[0-9a-f]{64}  ${archive//./\\.}\$" "$stage/SHA256SUMS") || fail "SHA256SUMS has no line for $archive"
printf '%s  %s\n' "${line%% *}" "$stage/$archive" | "${as_root[@]}" sha256sum --check --strict --quiet - ||
  fail "$archive does not match SHA256SUMS"

if [[ $component == server ]]; then
  "${as_root[@]}" tar -xzf "$stage/$archive" -C "$stage" dhow-server
  "${as_root[@]}" "$stage/dhow-server" install "${args[@]}"
  exit 0
fi

if [[ $component == cli ]]; then
  "${as_root[@]}" tar -xzf "$stage/$archive" -C "$stage" dhow
  "${as_root[@]}" install -D -m 0755 -- "$stage/dhow" /usr/local/bin/dhow
  cat <<EOF
get.sh: installed dhow $version:
  /usr/local/bin/dhow

Check a pipeline file, without a network connection:

  dhow lint .dhow.yml

See https://github.com/$repo/blob/main/docs/en/cli.md
EOF
  exit 0
fi

"${as_root[@]}" tar -xzf "$stage/$archive" -C "$stage" dhow-runner dhow-runner.service
"${as_root[@]}" install -D -m 0755 -- "$stage/dhow-runner" /usr/local/bin/dhow-runner
"${as_root[@]}" install -D -m 0644 -- "$stage/dhow-runner.service" /etc/systemd/system/dhow-runner.service
if [[ -d /run/systemd/system ]]; then
  "${as_root[@]}" systemctl daemon-reload
fi
cat <<EOF
get.sh: installed dhow-runner $version:
  /usr/local/bin/dhow-runner
  /etc/systemd/system/dhow-runner.service (not enabled yet)

Next, install and register it. Have a registration token from the Runners page of your
Dhow server ready, then run:

  sudo dhow-runner install

It asks for the server address and the token (hidden while you type), checks Podman,
cgroup v2 and user namespaces, creates the dhow-runner user, its sub-ids and directories,
keeps the identity encrypted with systemd-creds when the host can, and starts the runner.
For an unattended install, use: sudo dhow-runner install --answers <file>
See https://github.com/$repo/blob/main/docs/en/runner/install.md
EOF
