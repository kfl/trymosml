#!/usr/bin/env bash
# Run from the unpacked bundle on a prepared Ubuntu host.
set -euo pipefail
if [[ $EUID -ne 0 ]]; then echo "Run as root" >&2; exit 1; fi
for tool in docker caddy python3 systemctl; do command -v "$tool" >/dev/null; done
image=trymosml/repl:legacy-20160924
docker image inspect "$image" >/dev/null
source_dir=$(cd "$(dirname "$0")/.." && pwd)
if [[ "$source_dir" == /opt/trymosml ]]; then
    echo "Unpack the bundle outside /opt/trymosml before installing" >&2
    exit 1
fi
if ! id trymosml >/dev/null 2>&1; then
    useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin trymosml
fi
systemctl stop trymosml.service 2>/dev/null || true
install -d -m 755 /opt/trymosml /etc/trymosml
cp -a "$source_dir/web" "$source_dir/webshell" "$source_dir/deploy" /opt/trymosml/
chown -R root:root /opt/trymosml
chmod 755 /opt/trymosml/deploy/{repl.py,install.sh} /opt/trymosml/webshell/webshell
install -m 644 "$source_dir/deploy/webshell.json" /etc/trymosml/webshell.json
install -m 644 "$source_dir/deploy/trymosml.service" /etc/systemd/system/trymosml.service
caddy validate --config "$source_dir/deploy/Caddyfile" --adapter caddyfile
install -m 644 "$source_dir/deploy/Caddyfile" /etc/caddy/Caddyfile
systemctl daemon-reload
systemctl enable --now docker trymosml
curl --fail --retry 10 --retry-connrefused --retry-delay 1 http://127.0.0.1:8080/healthz
systemctl enable caddy
if systemctl is-active --quiet caddy; then systemctl reload caddy; else systemctl start caddy; fi
