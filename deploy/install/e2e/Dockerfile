ARG BASE=debian:12
FROM ${BASE}

RUN if command -v apt-get >/dev/null 2>&1; then \
      apt-get update \
      && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends systemd systemd-sysv dbus \
      && rm -rf /var/lib/apt/lists/*; \
    else \
      dnf install -y systemd procps-ng util-linux shadow-utils \
      && dnf clean all; \
    fi \
    && systemctl mask getty.target console-getty.service systemd-logind.service systemd-firstboot.service

STOPSIGNAL SIGRTMIN+3
CMD ["/bin/sh", "-c", "mount --make-rshared / && exec /sbin/init"]
