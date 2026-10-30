#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 4 || -z "$1" || ! -x "$2" || ! -x "$3" || -z "$4" ]]; then
  printf 'usage: %s BASE_IMAGE PATH_TO_DHOW_SERVER PATH_TO_DHOW_RUNNER DATABASE_URL\n' "$0" >&2
  printf '  Boots BASE_IMAGE with systemd on the host network and runs deploy/systemd/smoke-install.sh in it\n' >&2
  printf '  with the fake driver, so build the runner with -tags dhowtest.\n' >&2
  exit 2
fi

base=$1
server_bin=$2
runner_bin=$3
database_url=$4
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
slug=$(tr -c 'a-z0-9\n' '-' <<<"${base,,}")
name=${DHOW_E2E_NAME:-dhow-runner-install-$slug}
tag=dhow-runner-install:$slug
port=${DHOW_SMOKE_PORT:-18080}
runner_port=${DHOW_SMOKE_RUNNER_PORT:-18443}

in_box() { docker exec "$name" "$@"; }

cleanup() {
  local status=$?
  docker rm -f "$name" >/dev/null 2>&1 || true
  if [[ -z ${DHOW_E2E_KEEP_IMAGE:-} ]]; then
    docker rmi -f "$tag" >/dev/null 2>&1 || true
  fi
  exit "$status"
}
trap cleanup EXIT

printf 'Booting %s with systemd\n' "$base"
docker build --quiet --build-arg "BASE=$base" -t "$tag" -f "$here/Dockerfile.runner" "$here" >/dev/null
docker run -d --name "$name" --privileged --network host --cgroupns=private --tmpfs /run --tmpfs /run/lock "$tag" >/dev/null
state=
for _ in $(seq 60); do
  state=$(docker exec "$name" systemctl is-system-running 2>/dev/null || true)
  [[ $state == running || $state == degraded ]] && break
  sleep 1
done
[[ $state == running || $state == degraded ]] || {
  printf 'runner-install.sh: systemd did not come up in %s (%s)\n' "$base" "$state" >&2
  exit 1
}
docker cp "$server_bin" "$name:/root/dhow-server"
docker cp "$runner_bin" "$name:/root/dhow-runner"
docker cp "$here/../../systemd/smoke-install.sh" "$name:/root/smoke-install.sh"
in_box env DHOW_SMOKE_IDENTITY="${DHOW_SMOKE_IDENTITY:-auto}" DHOW_SMOKE_DRIVER=fake DHOW_SMOKE_PORT="$port" DHOW_SMOKE_RUNNER_PORT="$runner_port" \
  /root/smoke-install.sh /root/dhow-server /root/dhow-runner "$database_url"
