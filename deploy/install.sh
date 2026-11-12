#!/usr/bin/env bash
# Install an unpacked release, or switch back to the previous release.
set -euo pipefail
[[ $EUID -eq 0 ]] || { echo 'Run as root' >&2; exit 1; }
[[ $# -eq 0 || ( $# -eq 1 && $1 == --rollback ) ]] || {
    echo 'Usage: install.sh [--rollback]' >&2; exit 1;
}
for tool in docker caddy python3 systemctl systemd-run curl flock runuser; do command -v "$tool" >/dev/null; done
exec 9>/run/lock/trymosml-deploy.lock
flock -n 9 || { echo 'Another deployment is running' >&2; exit 1; }

current=/opt/trymosml
previous=/opt/trymosml-previous
releases=/opt/trymosml-releases
config=/etc/trymosml/webshell.json
caddy_config=/etc/caddy/Caddyfile
unit=/etc/systemd/system/trymosml.service
source_dir=$(cd "$(dirname "$0")/.." && pwd -P)
old=''
legacy=false
stopped=false
switched=false
unit_created=false
caddy_created=false
config_created=false

# Rename a symlink on the same filesystem so readers see either whole release.
point_to() {
    local link=$1 target=$2 temporary="${1}.new.$$"
    ln -s "$target" "$temporary"
    mv -Tf "$temporary" "$link"
}

recover() {
    local status=$?
    trap - EXIT
    if [[ $status -ne 0 && $stopped == true ]]; then
        echo 'Deployment failed; restoring the previous release.' >&2
        systemctl stop trymosml || true
        if [[ ( -n "$old" && -d "$old" ) || ( $legacy == true && -d "$current" && ! -L "$current" ) ]]; then
            if [[ $switched == true || ! -e "$current" ]]; then
                if [[ -n "$old" && -d "$old" ]]; then
                    point_to "$current" "$old" || true
                else
                    echo 'No previous release to restore' >&2
                fi
            fi
            systemctl daemon-reload || true
            systemctl start trymosml || echo 'RESTORE FAILED: inspect trymosml.service' >&2
        else
            if [[ $switched == true && -L "$current" ]]; then rm "$current"; fi
            if [[ $unit_created == true ]]; then rm -f "$unit"; fi
            if [[ $config_created == true ]]; then rm -f "$config"; fi
            if [[ $caddy_created == true ]]; then rm -f "$caddy_config"; fi
            systemctl daemon-reload || true
        fi
    fi
    exit "$status"
}
trap recover EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

if [[ -L "$current" ]]; then
    old=$(readlink -f "$current")
    [[ -d "$old" ]] || { echo 'Current release symlink is broken' >&2; exit 1; }
elif [[ -d "$current" ]]; then
    legacy=true
elif [[ -e "$current" ]]; then
    echo "$current must be a directory or a symlink" >&2; exit 1
fi
install -d -m 755 "$releases"
if [[ ${1:-} == --rollback ]]; then
    [[ -L "$previous" ]] || { echo 'No previous release available' >&2; exit 1; }
    candidate=$(readlink -f "$previous")
    [[ "$candidate" == "$releases/"* && -d "$candidate" && "$candidate" != "$old" ]] || {
        echo 'Invalid previous release' >&2; exit 1;
    }
else
    [[ -f "$source_dir/webshell/webshell" && -f "$source_dir/web/index.html" ]] || {
        echo 'Run the installer from a complete unpacked bundle' >&2; exit 1;
    }
    candidate=$(mktemp -d "$releases/$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX")
    cp -a "$source_dir/web" "$source_dir/webshell" "$source_dir/deploy" "$candidate/"
    chown -R root:root "$candidate"
    chmod 755 "$candidate" "$candidate/webshell/webshell" "$candidate/deploy/"{repl.py,install.sh}
fi
[[ -x "$candidate/webshell/webshell" && -x "$candidate/deploy/repl.py" ]] || {
    echo 'Release executables are missing' >&2; exit 1;
}

# Validate the configuration that will actually be used, before stopping anything.
# Existing application, Caddy, environment, unit and unit drop-in files are preserved.
app_input=$config
[[ -e "$app_input" ]] || app_input="$candidate/deploy/webshell.json"
caddy_input=$caddy_config
[[ -e "$caddy_input" ]] || caddy_input="$candidate/deploy/Caddyfile"
caddy validate --config "$caddy_input" --adapter caddyfile
health_url=$(python3 - "$app_input" "$candidate" <<'PY'
import json, os, pathlib, sys
cfg = dict(Addr='127.0.0.1:8080', CommandArgs=[],
           SessionTimeoutSeconds=1800, IdleTimeoutSeconds=600,
           MaxSessions=4, MaxMessageBytes=1 << 20)
cfg.update(json.loads(pathlib.Path(sys.argv[1]).read_text()))
for key in ('SessionTimeoutSeconds', 'IdleTimeoutSeconds', 'MaxSessions', 'MaxMessageBytes'):
    if type(cfg[key]) is not int or cfg[key] <= 0:
        raise ValueError(key + ' must be a positive integer')
if not isinstance(cfg['CommandArgs'], list) or not all(isinstance(x, str) for x in cfg['CommandArgs']):
    raise ValueError('CommandArgs must be a list of strings')
addr = cfg['Addr']
if not isinstance(addr, str):
    raise ValueError('Addr must be a loopback address and port')
host, sep, port = addr.rpartition(':')
if host not in ('127.0.0.1', '[::1]', 'localhost') or not sep or not port.isascii() or not port.isdigit() or not 1 <= int(port) <= 65535:
    raise ValueError('Deployment requires a loopback address and explicit port')
def staged_path(value):
    if not isinstance(value, str) or not value:
        raise ValueError('Command must be a nonempty path')
    prefix = '/opt/trymosml/'
    if value.startswith(prefix):
        return pathlib.Path(sys.argv[2]) / value[len(prefix):]
    path = pathlib.Path(value)
    return path if path.is_absolute() else pathlib.Path(sys.argv[2]) / path
root, command = pathlib.Path(sys.argv[2]) / 'web', staged_path(cfg.get('Command'))
if not (root / 'index.html').is_file() or not command.is_file() or not os.access(command, os.X_OK):
    raise ValueError('Configured web directory or launcher is missing from the candidate release')
print('http://' + addr + '/healthz')
PY
)
# Caddy follows the release symlink and needs access to every public asset.
runuser -u caddy -- python3 - "$candidate/web" <<'PY'
import os, sys
def fail(error):
    raise error
for directory, _, files in os.walk(sys.argv[1], onerror=fail):
    if not os.access(directory, os.R_OK | os.X_OK):
        raise PermissionError(directory)
    for name in files:
        path = os.path.join(directory, name)
        if not os.access(path, os.R_OK):
            raise PermissionError(path)
PY
systemctl is-active --quiet docker
if ! id trymosml >/dev/null 2>&1; then
    useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin trymosml
fi

# Check the candidate's image with the same user and environment as the service.
if [[ -f "$candidate/deploy/repl-image.json" ]]; then
    systemd-run --quiet --wait --pipe --collect --service-type=exec \
        --property=User=trymosml --property=Group=trymosml \
        --property=SupplementaryGroups=docker \
        --property=EnvironmentFile=-/etc/trymosml/environment \
        "$candidate/deploy/repl.py" --check-image
else
    # Releases made before image manifests used this fixed recovery image.
    docker image inspect trymosml/repl:legacy-20160924 >/dev/null
fi

# Preserve the original directory during the first migration to versioned releases.
if [[ $legacy == true ]]; then
    old=$(mktemp -d "$releases/legacy-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX")
    rmdir "$old"
fi
stopped=true
if [[ -e "$unit" ]]; then systemctl stop trymosml; fi
if [[ $legacy == true ]]; then mv "$current" "$old"; fi
switched=true
point_to "$current" "$candidate"
install -d -m 755 /etc/trymosml /etc/caddy
if [[ ! -e "$config" ]]; then config_created=true; install -m 644 "$app_input" "$config"; fi
if [[ ! -e "$caddy_config" ]]; then caddy_created=true; install -m 644 "$caddy_input" "$caddy_config"; fi
if [[ ! -e "$unit" ]]; then unit_created=true; install -m 644 "$candidate/deploy/trymosml.service" "$unit"; fi
systemctl daemon-reload
systemctl enable --now docker
systemctl enable trymosml
systemctl start trymosml
curl --fail --silent --show-error --max-time 5 --retry 10 --retry-connrefused --retry-delay 1 "$health_url"
systemctl is-active --quiet trymosml
systemctl enable caddy
if systemctl is-active --quiet caddy; then systemctl reload caddy; else systemctl start caddy; fi
if [[ -n "$old" ]]; then point_to "$previous" "$old"; fi
stopped=false
printf 'Active release: %s\nPrevious release: %s\n' "$candidate" "${old:-none}"
