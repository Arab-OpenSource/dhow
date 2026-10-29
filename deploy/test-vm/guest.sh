#!/usr/bin/env bash
set -uo pipefail

src=/run/dhow-vm
console=/dev/ttyS0
work=/root/dhow-vm
log=/var/log/dhow-vm-test.log
pg_password=vm-only-password
pg_image=docker.io/library/postgres:18

exec > >(tee -a "$log" "$console") 2>&1

finish() {
  printf '\nDHOW-VM-TEST: %s\n' "$1"
  sync
  systemctl poweroff
  exit 0
}

mkdir -p "$work"
for f in dhow-server dhow-runner smoke-install.sh; do
  install -m 0755 "$src/$f" "$work/$f" || finish FAIL
done
printf 'Guest: %s\n' "$(grep -m1 '^PRETTY_NAME=' /etc/os-release | cut -d= -f2-)"
for tool in curl openssl; do
  command -v "$tool" >/dev/null 2>&1 && continue
  if command -v apt-get >/dev/null 2>&1; then
    DEBIAN_FRONTEND=noninteractive apt-get install -y "$tool" || finish FAIL
  else
    dnf install -y "$tool" || finish FAIL
  fi
done
podman --version || finish FAIL

podman run -d --name dhow-pg --network host -e POSTGRES_USER=dhow -e POSTGRES_PASSWORD="$pg_password" \
  -e POSTGRES_DB=dhow "$pg_image" -c listen_addresses=127.0.0.1 || finish FAIL
for _ in $(seq 90); do
  podman exec dhow-pg pg_isready -h 127.0.0.1 -U dhow >/dev/null 2>&1 && break
  sleep 2
done
podman exec dhow-pg pg_isready -h 127.0.0.1 -U dhow || finish FAIL

if "$work/smoke-install.sh" "$work/dhow-server" "$work/dhow-runner" \
  "postgres://dhow:$pg_password@127.0.0.1:5432/dhow?sslmode=disable"; then
  finish PASS
fi
finish FAIL
