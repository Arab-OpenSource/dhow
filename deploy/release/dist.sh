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
module=github.com/Khaymah/dhow

[[ "$version" =~ ^[0-9A-Za-z][0-9A-Za-z.+-]*$ ]] || fail "VERSION must look like 1.2.3 or 1.2.3-rc.1, got: $version"
[[ -f "$root/apps/server/internal/App/WebDist/index.html" ]] || fail "the web app is missing: run make web first"
epoch=${SOURCE_DATE_EPOCH:-$(git -C "$root" log -1 --format=%ct)}

rm -rf -- "$out"
mkdir -p -- "$out"
for arch in $arches; do
  name="dhow-server_${version}_linux_${arch}"
  stage="$out/$name"
  mkdir -p -- "$stage"
  (
    cd -- "$root"
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false \
      -ldflags "-s -w -buildid= -X $module/internal/buildinfo.version=$version -X $module/internal/buildinfo.commit=$commit" \
      -o "$stage/dhow-server" ./apps/server/cmd/dhow-server
  )
  cp -- "$root/deploy/systemd/dhow.service" "$root/LICENSE" "$root/README.md" "$stage/"
  chmod 0755 "$stage/dhow-server"
  chmod 0644 "$stage/dhow.service" "$stage/LICENSE" "$stage/README.md"
  tar --sort=name --mtime="@$epoch" --owner=0 --group=0 --numeric-owner --format=posix \
    --pax-option=exthdr.name=%d/PaxHeaders/%f,delete=atime,delete=ctime \
    -C "$stage" -cf - dhow-server dhow.service LICENSE README.md | gzip -9n >"$out/$name.tar.gz"
  rm -rf -- "$stage"
done
(cd -- "$out" && sha256sum -- *.tar.gz >SHA256SUMS)
printf 'Wrote %s\n' "$out"/*.tar.gz "$out/SHA256SUMS"
