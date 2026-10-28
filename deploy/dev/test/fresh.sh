#!/usr/bin/env bash
set -euo pipefail

user=dev
windows='/mnt/c/Program Files/nodejs'
ca_opt=()

if [[ -n ${DHOW_EXTRA_CA:-} ]]; then
  ca_opt=(-o "Acquire::https::CAInfo=$DHOW_EXTRA_CA")
fi

if command -v apt-get >/dev/null; then
  if [[ -n ${DHOW_EXTRA_CA:-} && -f /etc/apt/sources.list.d/ubuntu.sources ]]; then
    sed -i 's#http://#https://#g' /etc/apt/sources.list.d/ubuntu.sources
  fi
  apt-get "${ca_opt[@]}" update -qq
  DEBIAN_FRONTEND=noninteractive apt-get "${ca_opt[@]}" install -y -qq sudo ca-certificates git >/dev/null
  if [[ -n ${DHOW_EXTRA_CA:-} ]]; then
    cp "$DHOW_EXTRA_CA" /usr/local/share/ca-certificates/extra.crt
    update-ca-certificates >/dev/null
  fi
else
  dnf install -y -q sudo ca-certificates git >/dev/null
  if [[ -n ${DHOW_EXTRA_CA:-} ]]; then
    cp "$DHOW_EXTRA_CA" /etc/pki/ca-trust/source/anchors/extra.crt
    update-ca-trust
  fi
fi

useradd --create-home --shell /bin/bash "$user"
printf '%s ALL=(ALL) NOPASSWD:ALL\nDefaults env_keep += "https_proxy HTTPS_PROXY"\n' "$user" >"/etc/sudoers.d/$user"

mkdir -p "$windows" /mnt/c/WINDOWS/system32
for tool in node npm npx corepack pnpm go; do
  printf '#!/bin/sh\r\nexit 0\r\n' >"$windows/$tool"
  chmod +x "$windows/$tool"
done

git config --global --add safe.directory '*'
git clone -q /repo "/home/$user/dhow"
chown -R "$user:$user" "/home/$user/dhow"

cd "/home/$user"
sudo -u "$user" env -i HOME="/home/$user" USER="$user" LANG=C.UTF-8 WSL_DISTRO_NAME=Ubuntu-24.04 \
  https_proxy="${https_proxy:-}" HTTPS_PROXY="${https_proxy:-}" NODE_EXTRA_CA_CERTS="${DHOW_EXTRA_CA:-}" \
  PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$windows:/mnt/c/WINDOWS/system32" \
  bash "/home/$user/dhow/deploy/dev/setup.sh" "$@"
