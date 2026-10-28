#!/usr/bin/env bash

MIN_GO=1.21
MIN_NODE=22
added_paths=()

add_path() {
  case ":$PATH:" in
    *":$1:"*) ;;
    *) PATH="$1:$PATH"; export PATH; added_paths+=("$1") ;;
  esac
}

ensure_basics() {
  local missing=() tool
  for tool in git make curl python3 tar; do
    have "$tool" || missing+=("$tool")
  done
  if [[ $os == darwin ]]; then
    if ((${#missing[@]})) || ! xcode-select -p >/dev/null 2>&1; then
      xcode-select --install >/dev/null 2>&1 || true
      fail "install the Xcode Command Line Tools from the window that opened (they bring git, make and python3), then run this script again"
    fi
    return
  fi
  have xz || missing+=(xz)
  have_lib libatomic.so.1 || missing+=(libatomic)
  ((${#missing[@]})) || return 0
  confirm "Install ${missing[*]} with the system package manager?" || fail "these tools are required: ${missing[*]}"
  case $distro in
    debian)
      as_root apt-get update -q
      as_root apt-get install -y -q git make curl python3 tar xz-utils ca-certificates libatomic1
      ;;
    fedora | rhel) as_root dnf install -y -q git make curl python3 tar xz libatomic ;;
    *) fail "install ${missing[*]} with your package manager, then run this script again" ;;
  esac
}

ensure_go() {
  local current=
  if have go; then
    current=$(go env GOVERSION 2>/dev/null || true)
    current=${current#go}
    if [[ -n $current ]] && version_ge "$current" "$MIN_GO"; then
      printf 'Go %s found; it downloads the version pinned in go.mod by itself\n' "$current"
      return
    fi
  fi
  if [[ $os == darwin ]] && have brew && confirm "Install Go with Homebrew?"; then
    brew install go
    return
  fi
  confirm "Download the latest stable Go from go.dev into $DEV_HOME/go?" || fail "Go $MIN_GO or newer is required"
  local listing file sum tmp
  printf 'Looking up the latest Go release\n'
  listing=$(fetch 'https://go.dev/dl/?mode=json')
  read -r file sum < <(python3 -c '
import json, sys
for release in json.load(sys.stdin):
    if not release["stable"]:
        continue
    for f in release["files"]:
        if f["os"] == sys.argv[1] and f["arch"] == sys.argv[2] and f["kind"] == "archive":
            print(f["filename"], f["sha256"])
            sys.exit(0)
sys.exit(1)' "$os" "$arch" <<<"$listing") || fail "go.dev lists no Go archive for $os/$arch"
  tmp=$(mktemp -d)
  download "$tmp/$file" "https://go.dev/dl/$file"
  [[ $(sha256_of "$tmp/$file") == "$sum" ]] || fail "$file does not match the checksum go.dev publishes"
  printf 'Checksum verified; unpacking into %s/go\n' "$DEV_HOME"
  mkdir -p "$DEV_HOME"
  rm -rf "$DEV_HOME/go"
  tar -xzf "$tmp/$file" -C "$DEV_HOME"
  rm -rf "$tmp"
  add_path "$DEV_HOME/go/bin"
}

node_platform() {
  case $arch in
    amd64) printf '%s-x64' "$os" ;;
    arm64) printf '%s-arm64' "$os" ;;
  esac
}

ensure_node() {
  local current=
  if have node; then
    current=$(node --version)
    current=${current#v}
    if version_ge "$current" "$MIN_NODE"; then
      printf 'Node.js %s found\n' "$current"
      return
    fi
  fi
  if [[ $os == darwin ]] && have brew && confirm "Install Node.js with Homebrew?"; then
    brew install node
    return
  fi
  confirm "Download the latest Node.js LTS from nodejs.org into $DEV_HOME/node?" || fail "Node.js $MIN_NODE or newer is required"
  local version file tmp expected
  printf 'Looking up the latest Node.js LTS release\n'
  version=$(fetch 'https://nodejs.org/dist/index.json' | python3 -c '
import json, sys
for r in json.load(sys.stdin):
    if r["lts"] and int(r["version"].lstrip("v").split(".")[0]) >= int(sys.argv[1]):
        print(r["version"])
        sys.exit(0)
sys.exit(1)' "$MIN_NODE") || fail "nodejs.org lists no LTS release $MIN_NODE or newer"
  file="node-$version-$(node_platform).tar.gz"
  tmp=$(mktemp -d)
  download "$tmp/$file" "https://nodejs.org/dist/$version/$file"
  fetch -o "$tmp/SHASUMS256.txt" "https://nodejs.org/dist/$version/SHASUMS256.txt"
  expected=$(awk -v f="$file" '$2 == f {print $1}' "$tmp/SHASUMS256.txt")
  [[ -n $expected && $(sha256_of "$tmp/$file") == "$expected" ]] || fail "$file does not match SHASUMS256.txt from nodejs.org"
  printf 'Checksum verified; unpacking into %s/node\n' "$DEV_HOME"
  mkdir -p "$DEV_HOME"
  rm -rf "$DEV_HOME/node"
  tar -xzf "$tmp/$file" -C "$DEV_HOME"
  mv "$DEV_HOME/${file%.tar.gz}" "$DEV_HOME/node"
  rm -rf "$tmp"
  add_path "$DEV_HOME/node/bin"
}

ensure_pnpm() {
  local want
  want=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["packageManager"].split("@")[1].split("+")[0])' "$root/apps/web/package.json")
  if have pnpm && [[ $(pnpm --version 2>/dev/null) == "$want" ]]; then
    printf 'pnpm %s found\n' "$want"
    return
  fi
  printf 'Installing pnpm %s\n' "$want"
  mkdir -p "$LOCAL_BIN"
  add_path "$LOCAL_BIN"
  if have corepack; then
    corepack enable --install-directory "$LOCAL_BIN" pnpm
    COREPACK_ENABLE_DOWNLOAD_PROMPT=0 corepack prepare "pnpm@$want" --activate
  else
    npm install --global --prefix "$HOME/.local" "pnpm@$want"
  fi
  [[ $(pnpm --version 2>/dev/null) == "$want" ]] || fail "pnpm $want did not install; check that $LOCAL_BIN comes first in PATH"
}

persist_paths() {
  ((${#added_paths[@]})) || return 0
  local line rc
  line="export PATH=\"$(IFS=:; printf '%s' "${added_paths[*]}"):\$PATH\""
  rc=$HOME/.profile
  [[ $os == darwin ]] && rc=$HOME/.zprofile
  if grep -qsF "$line" "$rc"; then
    return
  fi
  if confirm "Add the new tools to PATH in $rc?"; then
    printf '\n%s\n' "$line" >>"$rc"
    printf 'Open a new terminal, or run: source %s\n' "$rc"
  else
    printf 'Add this to your shell profile:\n  %s\n' "$line"
  fi
}
