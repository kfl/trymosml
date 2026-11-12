#!/usr/bin/env bash
# Build and export locally; never push to a registry.
set -euo pipefail
cd "$(dirname "$0")/.."
[[ $# -le 1 ]] || { echo 'Usage: build-repl.sh [unique-tag]' >&2; exit 1; }
tag=${1:-trymosml/repl:trixie-$(date -u +%Y%m%dT%H%M%SZ)}
[[ "$tag" =~ ^trymosml/repl:trixie-[A-Za-z0-9_.-]+$ ]] || {
    echo 'Use a unique tag beginning with trymosml/repl:trixie-' >&2; exit 1;
}
if docker image inspect "$tag" >/dev/null 2>&1; then
    echo "Tag already exists; choose a new tag to preserve rollback: $tag" >&2; exit 1
fi
mkdir -p dist
archive="dist/repl-${tag##*:}.tar.gz"
[[ ! -e "$archive" ]] || { echo "Archive already exists: $archive" >&2; exit 1; }
docker build --pull --no-cache --platform linux/amd64 -t "$tag" docker-build
docker image save "$tag" | gzip -n > "$archive"
manifest=$(mktemp)
trap 'rm -f "$manifest"' EXIT
python3 - "$archive" > "$manifest" <<'PY'
import json, sys
sys.path.insert(0, 'deploy')
from repl_image import archive_info, sha256
info = archive_info(sys.argv[1])
info['dockerfile_sha256'] = sha256('docker-build/Dockerfile')
info['mosml_package_sha256'] = sha256('docker-build/mosmlfiles/mosml_2.10.1-0ubuntu2_amd64.deb')
print(json.dumps(info, indent=2))
PY
install -m 644 "$manifest" deploy/repl-image.json
printf 'Image: %s\nArchive: %s\nManifest: deploy/repl-image.json\n' "$tag" "$archive"
