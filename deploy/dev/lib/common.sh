#!/usr/bin/env bash

DEV_HOME=${DHOW_DEV_HOME:-$HOME/.local/share/dhow-dev}
LOCAL_BIN=$HOME/.local/bin
assume_yes=${assume_yes:-false}
os=
distro=
arch=

fail() {
  printf 'dev-setup: %s\n' "$1" >&2
  exit 1
}

warn() {
  printf 'dev-setup: warning: %s\n' "$1" >&2
}

step() {
  printf '\n==> %s\n' "$1"
}

have() {
  command -v "$1" >/dev/null 2>&1
}

have_lib() {
  local ldconfig
  ldconfig=$(command -v ldconfig || printf '/sbin/ldconfig')
  "$ldconfig" -p 2>/dev/null | grep -qF "$1"
}

interactive() {
  [[ $assume_yes != true && -t 0 ]]
}

ask() {
  local name=$1 question=$2 default=${3-} reply=
  if interactive; then
    read -r -p "$question${default:+ [$default]}: " reply
  fi
  printf -v "$name" '%s' "${reply:-$default}"
}

ask_secret() {
  local name=$1 question=$2 reply=
  if interactive; then
    read -r -s -p "$question: " reply
    printf '\n'
  fi
  printf -v "$name" '%s' "$reply"
}

confirm() {
  local question=$1 default=${2:-y} reply=
  if interactive; then
    if [[ $default == y ]]; then
      read -r -p "$question [Y/n]: " reply
    else
      read -r -p "$question [y/N]: " reply
    fi
  fi
  reply=${reply:-$default}
  [[ $reply == y || $reply == Y || $reply == yes ]]
}

as_root() {
  if ((EUID == 0)); then
    "$@"
  else
    have sudo || fail "this step needs root; install sudo or run the script as root"
    sudo "$@"
  fi
}

random_hex() {
  od -An -tx1 -N"$1" /dev/urandom | tr -d ' \n'
}

version_ge() {
  [[ "$(printf '%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]
}

sha256_of() {
  if have sha256sum; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

fetch() {
  curl --proto '=https' --tlsv1.2 -fsSL "$@"
}

download() {
  local out=$1 url=$2
  printf 'Downloading %s\n' "$url"
  if [[ -t 2 ]]; then
    curl --proto '=https' --tlsv1.2 -fSL --progress-bar -o "$out" "$url"
  else
    fetch -o "$out" "$url"
  fi
}

on_wsl() {
  local release=
  [[ -n ${WSL_DISTRO_NAME:-} || -n ${WSL_INTEROP:-} ]] && return 0
  compgen -G '/proc/sys/fs/binfmt_misc/WSLInterop*' >/dev/null && return 0
  [[ -r /proc/sys/kernel/osrelease ]] && release=$(</proc/sys/kernel/osrelease)
  [[ ${release,,} == *microsoft* || ${release,,} == *wsl* ]]
}

drop_windows_paths() {
  local entry kept=() IFS=:
  for entry in $PATH; do
    [[ $entry == /mnt/[a-zA-Z]/* ]] || kept+=("$entry")
  done
  PATH="${kept[*]}"
  export PATH
}

detect_platform() {
  case $(uname -s) in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) fail "unsupported system $(uname -s); on Windows run deploy/dev/setup.ps1, which uses WSL2" ;;
  esac
  case $(uname -m) in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *) fail "unsupported architecture $(uname -m)" ;;
  esac
  if [[ $os == darwin ]]; then
    distro=macos
    return
  fi
  on_wsl && drop_windows_paths
  local id='' like=''
  if [[ -r /etc/os-release ]]; then
    id=$(. /etc/os-release && printf '%s' "${ID:-}")
    like=$(. /etc/os-release && printf '%s' "${ID_LIKE:-}")
  fi
  case " $id $like " in
    *" debian "* | *" ubuntu "*) distro=debian ;;
    *" fedora "*) distro=fedora ;;
    *" rhel "* | *" centos "* | *" rocky "* | *" almalinux "*) distro=rhel ;;
    *) distro=other ;;
  esac
  if [[ $distro == fedora && $id != fedora ]]; then
    distro=rhel
  fi
}
