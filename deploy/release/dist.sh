#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf 'dist.sh: %s\n' "$1" >&2
  exit 1
}

if [[ $# -ne 2 || -z "$1" || -z "$2" ]]; then
  printf 'usage: %s VERSION COMMIT\n' "$0" >&2
  exit 2
fi

version=$1
commit=$2
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
out=${DIST_DIR:-$root/dist}
arches=${DIST_ARCHES:-amd64 arm64}
components=${DIST_COMPONENTS:-server runner}
module=github.com/Khaymah/dhow

[[ "$version" =~ ^[0-9A-Za-z][0-9A-Za-z.+-]*$ ]] || fail "VERSION must look like 1.2.3 or 1.2.3-rc.1, got: $version"
for component in $components; do
  case $component in
    server | runner) ;;
    *) fail "DIST_COMPONENTS lists server and runner only, got: $component" ;;
  esac
done
if [[ " $components " == *" server "* ]]; then
  [[ -f "$root/apps/server/internal/App/WebDist/index.html" ]] || fail "the web app is missing: run make web first"
fi
epoch=${SOURCE_DATE_EPOCH:-$(git -C "$root" log -1 --format=%ct)}

archive() {
  local component=$1 arch=$2 binary="dhow-$1" unit
  case $component in
    server) unit=dhow.service ;;
    runner) unit=dhow-runner.service ;;
  esac
  local name="${binary}_${version}_linux_${arch}"
  local stage="$out/$name"
  mkdir -p -- "$stage"
  (
    cd -- "$root"
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false \
      -ldflags "-s -w -buildid= -X $module/internal/buildinfo.version=$version -X $module/internal/buildinfo.commit=$commit" \
      -o "$stage/$binary" "./apps/$component/cmd/$binary"
  )
  cp -- "$root/deploy/systemd/$unit" "$root/LICENSE" "$root/README.md" "$stage/"
  chmod 0755 "$stage/$binary"
  chmod 0644 "$stage/$unit" "$stage/LICENSE" "$stage/README.md"
  tar --sort=name --mtime="@$epoch" --owner=0 --group=0 --numeric-owner --format=posix \
    --pax-option=exthdr.name=%d/PaxHeaders/%f,delete=atime,delete=ctime \
    -C "$stage" -cf - "$binary" "$unit" LICENSE README.md | gzip -9n >"$out/$name.tar.gz"
  rm -rf -- "$stage"
}

rm -rf -- "$out"
mkdir -p -- "$out"
for component in $components; do
  for arch in $arches; do
    archive "$component" "$arch"
  done
done
(cd -- "$out" && sha256sum -- *.tar.gz >SHA256SUMS)
printf 'Wrote %s\n' "$out"/*.tar.gz "$out/SHA256SUMS"
