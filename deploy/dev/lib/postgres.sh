#!/usr/bin/env bash

MIN_PG=180000
db_mode=
db_engine=
db_port=
db_password=
db_url=
db_test_url=
pg_psql=
pg_super=()

choose_db() {
  local engine='' default=native
  have docker && engine=docker
  [[ -z $engine ]] && have podman && engine=podman
  [[ -n $engine ]] && default=container
  if [[ -z $db_mode ]]; then
    printf 'PostgreSQL 18:\n  container  in %s, through make dev-db\n  native     installed on this machine\n  url        a database you already have\n' "${engine:-Docker or Podman (not found)}"
    ask db_mode "Which one" "$default"
  fi
  case $db_mode in
    container)
      if [[ -z $engine ]] && on_wsl; then
        fail "docker is not reachable in ${WSL_DISTRO_NAME:-this WSL distribution}; in Docker Desktop open Settings, Resources, WSL integration, turn it on for ${WSL_DISTRO_NAME:-this distribution}, then run this script again"
      fi
      [[ -n $engine ]] || fail "neither docker nor podman is installed; choose native or url"
      ;;
    native | url) ;;
    *) fail "unknown database choice: $db_mode (container, native or url)" ;;
  esac
  db_engine=$engine
}

setup_db() {
  case $db_mode in
    container) db_container ;;
    native) db_native ;;
    url) db_existing ;;
  esac
}

bootstrap_sql() {
  printf '%s\n' '\set ON_ERROR_STOP on' \
    "\\set pw '$db_password'" \
    "SELECT format('CREATE ROLE dhow LOGIN PASSWORD %L', :'pw') WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'dhow') \\gexec" \
    "SELECT format('ALTER ROLE dhow LOGIN PASSWORD %L', :'pw') \\gexec" \
    "SELECT 'CREATE DATABASE dhow OWNER dhow' WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'dhow') \\gexec" \
    "SELECT 'CREATE DATABASE dhow_test OWNER dhow' WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'dhow_test') \\gexec"
}

require_pg18() {
  local version
  version=$("$@" -XtAc 'SHOW server_version_num' | tr -d '[:space:]')
  [[ $version =~ ^[0-9]+$ ]] || fail "could not read the PostgreSQL version"
  ((version >= MIN_PG)) || fail "PostgreSQL $version is older than 18; Dhow's migrations need uuidv7() from PostgreSQL 18"
}

local_urls() {
  db_url="postgres://dhow:$db_password@127.0.0.1:$db_port/dhow?sslmode=disable"
  db_test_url="postgres://dhow:$db_password@127.0.0.1:$db_port/dhow_test?sslmode=disable"
}

db_container() {
  step "Starting PostgreSQL 18 in $db_engine"
  make -C "$root" --no-print-directory dev-db DEV_DB_ENGINE="$db_engine" DEV_DB_PORT="$db_port"
  local exec=("$db_engine" exec -i dhow-dev-postgres psql -U dhow -d dhow)
  require_pg18 "${exec[@]}"
  bootstrap_sql | "${exec[@]}" -q >/dev/null
  local_urls
}

db_native() {
  step "Installing PostgreSQL 18 on this machine"
  case $distro in
    debian) native_debian ;;
    fedora | rhel) native_rpm ;;
    macos) native_macos ;;
    *) fail "install PostgreSQL 18 yourself (docs/development.md), then run this script again and choose url" ;;
  esac
  require_pg18 "${pg_super[@]}"
  bootstrap_sql | "${pg_super[@]}" -q >/dev/null
  local_urls
}

as_postgres() {
  if ((EUID == 0)); then
    pg_super=(runuser -u postgres -- "$pg_psql" -p "$db_port")
  else
    pg_super=(sudo -u postgres -- "$pg_psql" -p "$db_port")
  fi
}

native_debian() {
  if [[ ! -x /usr/lib/postgresql/18/bin/postgres ]]; then
    confirm "Add the PostgreSQL project's apt repository and install postgresql-18 (needs sudo)?" || fail "PostgreSQL 18 is required"
    as_root apt-get update -q
    as_root apt-get install -y -q postgresql-common
    as_root /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y
    as_root apt-get install -y -q postgresql-18
  fi
  as_root pg_ctlcluster 18 main start 2>/dev/null || true
  db_port=$(pg_lsclusters -h | awk '$1 == "18" && $2 == "main" {print $3}')
  [[ -n $db_port ]] || fail "no PostgreSQL 18 cluster named main; create one with: sudo pg_createcluster 18 main --start"
  pg_psql=/usr/lib/postgresql/18/bin/psql
  as_postgres
}

native_rpm() {
  local data=/var/lib/pgsql/18/data hba
  if [[ ! -x /usr/pgsql-18/bin/postgres ]]; then
    confirm "Add the PostgreSQL project's dnf repository and install postgresql18-server (needs sudo)?" || fail "PostgreSQL 18 is required"
    local rel repo=pgdg-redhat-repo-latest.noarch.rpm
    rel=EL-$(rpm -E %rhel)
    if [[ $distro == fedora ]]; then
      rel=F-$(rpm -E %fedora)
      repo=pgdg-fedora-repo-latest.noarch.rpm
    fi
    as_root dnf install -y -q "https://download.postgresql.org/pub/repos/yum/reporpms/$rel-$(uname -m)/$repo"
    [[ $distro == rhel ]] && { as_root dnf -qy module disable postgresql || true; }
    as_root dnf install -y -q postgresql18-server
  fi
  as_root test -f "$data/PG_VERSION" || as_root /usr/pgsql-18/bin/postgresql-18-setup initdb
  hba="$data/pg_hba.conf"
  if ! as_root grep -q '^host dhow,dhow_test dhow 127.0.0.1/32 scram-sha-256' "$hba"; then
    as_root sed -i '1i host dhow,dhow_test dhow 127.0.0.1/32 scram-sha-256' "$hba"
  fi
  as_root systemctl enable --now postgresql-18
  as_root systemctl reload postgresql-18
  db_port=5432
  pg_psql=/usr/pgsql-18/bin/psql
  as_postgres
}

native_macos() {
  have brew || fail "install Homebrew (brew.sh) or Postgres.app with PostgreSQL 18, then run this script again"
  if ! brew list postgresql@18 >/dev/null 2>&1; then
    confirm "Install postgresql@18 with Homebrew?" || fail "PostgreSQL 18 is required"
    brew install postgresql@18
  fi
  brew services start postgresql@18 >/dev/null
  pg_psql="$(brew --prefix postgresql@18)/bin/psql"
  db_port=5432
  local _
  for _ in $(seq 1 30); do
    "$(brew --prefix postgresql@18)/bin/pg_isready" -q -h 127.0.0.1 -p "$db_port" && break
    sleep 1
  done
  pg_super=("$pg_psql" -h 127.0.0.1 -p "$db_port" -d postgres)
}

db_existing() {
  [[ -n $db_url ]] || ask db_url "DHOW_DATABASE_URL (postgres://user:password@host:port/db)" ""
  [[ -n $db_test_url ]] || ask db_test_url "DHOW_TEST_DATABASE_URL (a separate database for the tests)" ""
  [[ $db_url == postgres://* || $db_url == postgresql://* ]] || fail "DHOW_DATABASE_URL must be a postgres:// URL"
  [[ $db_test_url == postgres://* || $db_test_url == postgresql://* ]] || fail "DHOW_TEST_DATABASE_URL must be a postgres:// URL"
  if have psql; then
    require_pg18 psql "$db_url"
  else
    warn "psql is not installed, so the server version is checked at the first start instead"
  fi
}
