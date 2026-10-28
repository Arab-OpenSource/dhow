#!/usr/bin/env bash
# shellcheck source-path=SCRIPTDIR/..
set -euo pipefail

here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=lib/common.sh
. "$here/../lib/common.sh"
# shellcheck source=lib/env.sh
. "$here/../lib/env.sh"

failures=0

expect() {
  local want=$1 check=$2 value=$3 got=no
  "$check" "$value" && got=yes
  if [[ $got != "$want" ]]; then
    printf 'FAIL %s %q: got %s, want %s\n' "$check" "$value" "$got" "$want"
    failures=$((failures + 1))
  fi
}

for v in 1 587 65535; do expect yes is_port "$v"; done
for v in 0 65536 Y '' 5x -1; do expect no is_port "$v"; done
for v in smtp.example.com localhost mail-1.example.org; do expect yes is_host "$v"; done
for v in '' 'smtp example.com' -smtp.example.com 'smtp.example.com:587' 'a/b'; do expect no is_host "$v"; done
for v in starttls tls none; do expect yes is_tls "$v"; done
for v in '' Y STARTTLS ssl; do expect no is_tls "$v"; done
for v in dhow@example.com dhow@localhost 'Dhow <dhow@example.com>' '<dhow@example.com>'; do expect yes is_mailbox "$v"; done
for v in '' Y dhow 'Dhow dhow@example.com' 'Dhow <dhow>' 'a@b@c' 'Dhow <a@b' 'a@b, c@d'; do expect no is_mailbox "$v"; done

smtp_host=Y smtp_port=Y smtp_tls=Y smtp_from=Y
if smtp_valid; then
  printf 'FAIL smtp_valid accepted Y for every answer\n'
  failures=$((failures + 1))
fi
smtp_host=smtp.example.com smtp_port=587 smtp_tls=starttls smtp_from='Dhow <dhow@example.com>'
smtp_valid || { printf 'FAIL smtp_valid rejected good settings\n'; failures=$((failures + 1)); }

# shellcheck disable=SC2317
interactive() { true; }

answer=''
ask_valid answer "Port" 8080 is_port "Type a number." 2>/dev/null <<<$'Y\n70000\n9090'
[[ $answer == 9090 ]] || { printf 'FAIL ask_valid kept %q\n' "$answer"; failures=$((failures + 1)); }

if confirm "Go on?" n 2>/dev/null <<<$'maybe\nYES'; then :; else
  printf 'FAIL confirm did not take YES after a wrong answer\n'
  failures=$((failures + 1))
fi
if confirm "Go on?" y 2>/dev/null <<<'No'; then
  printf 'FAIL confirm took No as yes\n'
  failures=$((failures + 1))
fi

# shellcheck disable=SC2317
interactive() { false; }
if (ask_valid answer "Port" Y is_port "Type a number." 2>/dev/null); then
  printf 'FAIL ask_valid accepted a bad default without a terminal\n'
  failures=$((failures + 1))
fi

detect_platform
on_wsl || true
drop_windows_paths

((failures == 0)) || exit 1
printf 'settings checks passed\n'
