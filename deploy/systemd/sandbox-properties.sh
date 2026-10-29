#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" ]]; then
  printf 'usage: %s UNIT_FILE\n' "$0" >&2
  exit 2
fi

sed -n '/^\[Service\]/,/^\[/p' "$1" | while IFS= read -r line; do
  case $line in
    '' | '['* | Type=* | User=* | Group=* | ExecStart=* | EnvironmentFile=* | Restart* | TimeoutStopSec=* | StateDirectory* | WorkingDirectory=*) ;;
    *=*) printf -- '-p %q ' "$line" ;;
  esac
done
printf '\n'
