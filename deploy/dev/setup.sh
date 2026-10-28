#!/usr/bin/env bash
# shellcheck source-path=SCRIPTDIR
set -euo pipefail
umask 077

here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
root=$(cd -- "$here/../.." && pwd)
assume_yes=false
skip_web=false
check_only=false

# shellcheck source=lib/common.sh
. "$here/lib/common.sh"
# shellcheck source=lib/tools.sh
. "$here/lib/tools.sh"
# shellcheck source=lib/postgres.sh
. "$here/lib/postgres.sh"
# shellcheck source=lib/env.sh
. "$here/lib/env.sh"

usage() {
  cat <<'USAGE'
Usage: deploy/dev/setup.sh [options]

Sets up a development machine for Dhow: Git, Make, Go, Node.js and pnpm,
PostgreSQL 18, and a private .env.dev with generated secrets.

  --yes               accept every default without asking (for scripts and CI)
  --db=MODE           container (Docker or Podman), native, or url
  --db-url=URL        with --db=url: the database for the server
  --test-db-url=URL   with --db=url: a separate database for the tests
  --port=PORT         the server port on localhost (default 8080)
  --db-port=PORT      the PostgreSQL port on localhost (default 5432)
  --env-file=PATH     where to write the settings (default .env.dev)
  --skip-web          do not build the web app
  --check             only check the tools; install and change nothing
  -h, --help          show this help
USAGE
}

parse_args() {
  while (($#)); do
    case $1 in
      --yes | -y) assume_yes=true ;;
      --db=*) db_mode=${1#*=} ;;
      --db-url=*) db_url=${1#*=} ;;
      --test-db-url=*) db_test_url=${1#*=} ;;
      --port=*) http_port=${1#*=} ;;
      --db-port=*) db_port=${1#*=} ;;
      --env-file=*) env_file=${1#*=} ;;
      --skip-web) skip_web=true ;;
      --check) check_only=true ;;
      -h | --help) usage; exit 0 ;;
      *) usage >&2; fail "unknown option: $1" ;;
    esac
    shift
  done
}

check_tools() {
  local tool status=0
  for tool in git make curl python3 go node pnpm psql docker podman; do
    if have "$tool"; then
      printf '  %-8s %s\n' "$tool" "$(command -v "$tool")"
    else
      printf '  %-8s missing\n' "$tool"
      case $tool in psql | docker | podman) ;; *) status=1 ;; esac
    fi
  done
  return "$status"
}

summary() {
  step "Ready"
  printf 'Start the server:      make dev\n'
  printf 'Web app, live reload:  make dev-vite, then pnpm -C apps/web dev (http://localhost:5173)\n'
  printf 'Every check and test:  make dev-check\n'
  printf 'Then open the setup_url link the server logs, on http://localhost:%s\n' "$http_port"
  if [[ $http_port != 8080 ]]; then
    warn "the Vite dev server forwards /api to port 8080, so live reload needs the server on 8080"
  fi
}

main() {
  parse_args "$@"
  env_file=${env_file:-$root/.env.dev}
  detect_platform
  printf 'Dhow development setup on %s (%s, %s)\n' "$distro" "$os" "$arch"
  if [[ $check_only == true ]]; then
    check_tools
    exit $?
  fi
  step "Tools"
  ensure_basics
  ensure_go
  ensure_node
  ensure_pnpm
  choose_db
  load_existing
  collect_settings
  setup_db
  write_env
  if [[ $skip_web == false ]]; then
    step "Building the web app"
    make -C "$root" --no-print-directory web
  fi
  persist_paths
  summary
}

main "$@"
