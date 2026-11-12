#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Build the existing REPL Dockerfile once, using the host's build cache.
# Passing its archive as a build context keeps generated files out of the checkout.
repl_archive=$(mktemp -d "${TMPDIR:-/tmp}/trymosml-repl.XXXXXX")
trap 'rm -rf "$repl_archive"' EXIT
docker build --platform linux/amd64 -t trymosml/repl:dev docker-build
docker image save --output "$repl_archive/repl.tar" trymosml/repl:dev
docker build --build-context "repl_archive=$repl_archive" \
    -f dev/Dockerfile -t trymosml/dev .
