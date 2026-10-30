#!/usr/bin/env bash
set -euo pipefail

if [[ ${GET_AS:-root} == user ]]; then
  useradd --create-home installer
  printf 'installer ALL=(ALL) NOPASSWD: ALL\n' >/etc/sudoers.d/installer
  chmod 0440 /etc/sudoers.d/installer
  before=$(find /tmp -mindepth 1 -maxdepth 1 -user root | wc -l)
  sudo -u installer -- env CURL_CA_BUNDLE="$CURL_CA_BUNDLE" DHOW_DOWNLOAD_URL="$DHOW_DOWNLOAD_URL" NO_PROXY="$NO_PROXY" \
    bash /get.sh "$@"
  after=$(find /tmp -mindepth 1 -maxdepth 1 -user root | wc -l)
  if ((after == before)); then
    printf 'no staging directory left\n'
  fi
else
  bash /get.sh "$@"
fi
if [[ -e /usr/local/bin/dhow-runner ]]; then
  /usr/local/bin/dhow-runner version
  stat -c '%a %U %n' /usr/local/bin/dhow-runner /etc/systemd/system/dhow-runner.service
fi
if [[ -e /usr/local/bin/dhow ]]; then
  /usr/local/bin/dhow version
  stat -c '%a %U %n' /usr/local/bin/dhow
  printf 'version: 2\n' >/tmp/bad.dhow.yml
  code=0
  /usr/local/bin/dhow lint /tmp/bad.dhow.yml || code=$?
  printf 'dhow lint exit %s\n' "$code"
fi
