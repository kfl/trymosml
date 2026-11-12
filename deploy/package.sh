#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p dist
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/trymosml/webshell" "$stage/trymosml/deploy"
(cd webshell && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "$stage/trymosml/webshell/webshell" .)
cp -R web "$stage/trymosml/"
cp deploy/{Caddyfile,webshell.json,repl.py,load-repl.py,trymosml.service,install.sh,bootstrap-ubuntu.sh,README.md} "$stage/trymosml/deploy/"
chmod 755 "$stage/trymosml/deploy/"{repl.py,install.sh,bootstrap-ubuntu.sh}
COPYFILE_DISABLE=1 tar --no-xattrs --exclude='.DS_Store' --exclude='._*' -C "$stage" -czf dist/trymosml-linux-amd64.tar.gz trymosml
shasum -a 256 dist/trymosml-linux-amd64.tar.gz
