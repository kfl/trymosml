#!/usr/bin/env bash
# Prepare a freshly rebuilt Ubuntu 26.04 amd64 Droplet.
set -euo pipefail
[[ $EUID -eq 0 ]] || { echo "Run as root" >&2; exit 1; }
. /etc/os-release
[[ "$ID" == ubuntu && "$VERSION_ID" == 26.04 ]] || { echo "Expected Ubuntu 26.04" >&2; exit 1; }
[[ $(uname -m) == x86_64 ]] || { echo "Expected amd64" >&2; exit 1; }

# Give the existing 512 MiB Droplet headroom during package installation.
# REPL containers have swap disabled independently.
if [[ $(awk '/MemTotal/ {print $2}' /proc/meminfo) -lt 1048576 ]] && ! swapon --show --noheadings | grep -q .; then
    if [[ ! -e /swapfile ]]; then
        fallocate -l 1G /swapfile
        chmod 600 /swapfile
        mkswap /swapfile
    fi
    swapon /swapfile
    grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get -y -o Dpkg::Options::=--force-confold upgrade
apt-get install -y ca-certificates curl gnupg python3 docker.io
install -d -m 755 /etc/apt/keyrings
curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/gpg.key |
    gpg --dearmor --yes -o /etc/apt/keyrings/caddy-stable.gpg
curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt |
    sed 's@/usr/share/keyrings/caddy-stable-archive-keyring.gpg@/etc/apt/keyrings/caddy-stable.gpg@g' > /etc/apt/sources.list.d/caddy-stable.list
chmod 644 /etc/apt/keyrings/caddy-stable.gpg
apt-get update
apt-get install -y caddy
systemctl enable --now docker
# Bound persistent journals on this small disk.
install -d /etc/systemd/journald.conf.d
printf '[Journal]\nSystemMaxUse=100M\n' > /etc/systemd/journald.conf.d/trymosml.conf
systemctl restart systemd-journald
