#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'USAGE'
usage: deploy/test-vm/run.sh DISTRO PATH_TO_DHOW_SERVER PATH_TO_DHOW_RUNNER

Boots a cloud image of DISTRO (debian-13, fedora or rocky-10) under QEMU/KVM. cloud-init starts
PostgreSQL 18 in a container, and the guest runs deploy/systemd/smoke-install.sh with real Podman:
dhow-runner install --answers, doctor, a job, rotate-keys, a second install and uninstall.

Environment:
  VM_CACHE        where downloaded images are kept (default: ~/.cache/dhow-vm)
  VM_ACCEL        kvm (default) or tcg, which works without /dev/kvm but is very slow
  VM_MEMORY       guest memory in MiB (default: 4096)
  VM_CPUS         guest CPUs (default: 2)
  VM_TIMEOUT      seconds to wait for the result (default: 2700)
  FEDORA_VERSION  the Fedora release to test (default: 43)
USAGE
  exit 2
}

[[ $# -eq 3 && -x "$2" && -x "$3" ]] || usage
distro=$1
server_bin=$2
runner_bin=$3
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
cache=${VM_CACHE:-$HOME/.cache/dhow-vm}
accel=${VM_ACCEL:-kvm}
memory=${VM_MEMORY:-4096}
cpus=${VM_CPUS:-2}
timeout=${VM_TIMEOUT:-2700}
marker='DHOW-VM-TEST:'
work=$(mktemp -d)
qemu_pid=

fail() {
  printf 'vm-test: %s\n' "$1" >&2
  exit 1
}

cleanup() {
  local status=$?
  if [[ -n $qemu_pid ]]; then kill "$qemu_pid" 2>/dev/null || true; fi
  if ((status != 0)) && [[ -s $work/console.log ]]; then
    printf '\n--- last lines of the guest console ---\n' >&2
    tail -n 80 "$work/console.log" >&2 || true
  fi
  rm -rf -- "$work"
  exit "$status"
}
trap cleanup EXIT

for tool in qemu-system-x86_64 qemu-img curl gpg sha256sum sha512sum; do
  command -v "$tool" >/dev/null 2>&1 || fail "$tool is not installed"
done
iso_tool=
for candidate in genisoimage mkisofs xorrisofs; do
  if command -v "$candidate" >/dev/null 2>&1; then
    iso_tool=$candidate
    break
  fi
done
[[ -n $iso_tool ]] || fail "genisoimage, mkisofs or xorrisofs is needed to build the cloud-init seed"
if [[ $accel == kvm && ! -w /dev/kvm ]]; then
  fail "/dev/kvm is missing or not writable; enable KVM, or set VM_ACCEL=tcg to emulate (slow)"
fi

fedora_version=${FEDORA_VERSION:-43}
case $distro in
  debian-13)
    dir=https://cloud.debian.org/images/cloud/trixie/latest
    image_re='debian-13-genericcloud-amd64\.qcow2'
    sums_re='SHA512SUMS'
    signature=none
    ;;
  fedora)
    dir=https://download.fedoraproject.org/pub/fedora/linux/releases/$fedora_version/Cloud/x86_64/images
    image_re='Fedora-Cloud-Base-Generic-[0-9.-]+\.x86_64\.qcow2'
    sums_re='Fedora-Cloud-[0-9.-]+-x86_64-CHECKSUM'
    signature=clearsigned
    key_url=https://fedoraproject.org/fedora.gpg
    key_fpr=${FEDORA_KEY_FINGERPRINT:-}
    if [[ -z $key_fpr && $fedora_version == 43 ]]; then key_fpr=C6E7F081CF80E13146676E88829B606631645531; fi
    [[ -n $key_fpr ]] || fail "set FEDORA_KEY_FINGERPRINT to the primary key of Fedora $fedora_version (https://fedoraproject.org/security/)"
    ;;
  rocky-10)
    dir=https://dl.rockylinux.org/pub/rocky/10/images/x86_64
    image_re='Rocky-10-GenericCloud-Base\.latest\.x86_64\.qcow2'
    sums_re='CHECKSUM'
    signature=detached
    key_url=https://dl.rockylinux.org/pub/rocky/RPM-GPG-KEY-Rocky-10
    key_fpr=FC226859C0860BF0DDB95B085B106C736FEDFC85
    ;;
  *) fail "DISTRO must be debian-13, fedora or rocky-10, got: $distro" ;;
esac

fetch() { curl -fsSL --retry 3 --proto '=https' --proto-redir '=https' "$@"; }

verify_sums() {
  [[ $signature == none ]] && return 0
  export GNUPGHOME=$work/gnupg
  install -d -m 0700 "$GNUPGHOME"
  fetch -o "$work/key.asc" "$key_url"
  gpg --batch --quiet --import "$work/key.asc" 2>/dev/null
  if [[ $signature == detached ]]; then
    fetch -o "$work/sums.asc" "$dir/$sums.asc"
    gpg --batch --status-fd 3 --verify "$work/sums.asc" "$work/sums" 3>"$work/gpg.status" 2>/dev/null || return 1
  else
    gpg --batch --status-fd 3 --output "$work/sums.signed" --decrypt "$work/sums" 3>"$work/gpg.status" 2>/dev/null || return 1
    mv -- "$work/sums.signed" "$work/sums"
  fi
  grep -qE "^\[GNUPG:\] VALIDSIG .* $key_fpr\$" "$work/gpg.status"
}

newest() { grep -oE "href=\"(\./)?$1\"" | cut -d'"' -f2 | sed 's|^\./||' | sort -uV | tail -n 1; }

expected_sum() {
  local file=$1 sums=$2 line
  line=$(grep -E "(^|[ (*])${file//./\\.}( |\)|$)" "$sums" | grep -vE '^#' | head -n 1 || true)
  [[ -n $line ]] || return 1
  if [[ $line == *' = '* ]]; then
    printf '%s\n' "${line##* = }"
  else
    printf '%s\n' "${line%% *}"
  fi
}

printf 'Finding the %s cloud image\n' "$distro"
fetch -o "$work/listing.html" "$dir/"
image=$(newest "$image_re" <"$work/listing.html")
sums=$(newest "$sums_re" <"$work/listing.html")
[[ -n $image && -n $sums ]] || fail "no image or checksum file matching $image_re and $sums_re in $dir/"
fetch -o "$work/sums" "$dir/$sums"
verify_sums || fail "$sums is not signed by the pinned $distro key $key_fpr"
want=$(expected_sum "$image" "$work/sums") || fail "$sums has no line for $image"
case ${#want} in
  64) hasher=sha256sum ;;
  128) hasher=sha512sum ;;
  *) fail "unknown checksum length in $sums" ;;
esac

mkdir -p "$cache"
cached="$cache/$distro-$image"
if [[ ! -s $cached ]] || [[ $($hasher "$cached" | cut -d' ' -f1) != "$want" ]]; then
  printf 'Downloading %s\n' "$image"
  curl -fL --retry 3 --proto '=https' --proto-redir '=https' -o "$cached.part" "$dir/$image"
  mv -- "$cached.part" "$cached"
fi
[[ $($hasher "$cached" | cut -d' ' -f1) == "$want" ]] || fail "$image does not match $sums"
if [[ $signature == none ]]; then
  printf 'Checked %s against %s (over https only: %s publishes no signature for it)\n' "$image" "$sums" "$distro"
else
  printf 'Checked %s against %s, signed by %s\n' "$image" "$sums" "$key_fpr"
fi

qemu-img create -q -f qcow2 -F qcow2 -b "$cached" "$work/disk.qcow2" 20G

seed=$work/seed
mkdir -p "$seed"
install -m 0755 "$server_bin" "$seed/dhow-server"
install -m 0755 "$runner_bin" "$seed/dhow-runner"
install -m 0755 "$here/../systemd/smoke-install.sh" "$seed/smoke-install.sh"
install -m 0755 "$here/guest.sh" "$seed/guest.sh"
printf 'instance-id: dhow-vm-%s\nlocal-hostname: dhow-vm\n' "$distro" >"$seed/meta-data"
cp -- "$here/user-data.yaml" "$seed/user-data"
if [[ $iso_tool == xorrisofs ]]; then
  xorrisofs -quiet -V cidata -J -R -o "$work/seed.iso" "$seed"
else
  "$iso_tool" -quiet -V cidata -J -R -o "$work/seed.iso" "$seed"
fi

printf 'Booting %s (%s, %s MiB, %s CPUs)\n' "$distro" "$accel" "$memory" "$cpus"
cpu=host
[[ $accel == kvm ]] || cpu=max
qemu-system-x86_64 -machine "q35,accel=$accel" -cpu "$cpu" -smp "$cpus" -m "$memory" \
  -drive "if=virtio,file=$work/disk.qcow2,format=qcow2" \
  -drive "if=virtio,file=$work/seed.iso,format=raw,media=cdrom,readonly=on" \
  -netdev user,id=net0 -device virtio-net-pci,netdev=net0 \
  -display none -monitor none -serial "file:$work/console.log" -no-reboot &
qemu_pid=$!

deadline=$((SECONDS + timeout))
result=
while ((SECONDS < deadline)); do
  if ! kill -0 "$qemu_pid" 2>/dev/null; then
    result=$(grep -ao "$marker [A-Z]*" "$work/console.log" | tail -n 1 || true)
    break
  fi
  result=$(grep -ao "$marker [A-Z]*" "$work/console.log" | tail -n 1 || true)
  [[ -n $result ]] && break
  sleep 5
done
[[ -n $result ]] || fail "no result from the guest within ${timeout}s"

grep -aE '^\s+(ok|FAIL)\s' "$work/console.log" | tr -d '\r' || true
if [[ $result != "$marker PASS" ]]; then
  fail "the checks failed on $distro"
fi
printf '\nAll runner install checks passed on %s\n' "$distro"
