# Development checks

The [root README](../README.md) covers startup. The inner Docker daemon listens
only on its own Unix socket. The `trymosml-dev-docker` volume preserves its image
store when the container is recreated; no host Docker socket is mounted. Only one
development container may use this volume at a time. Build natively on Apple
Silicon; the REPL image uses the repository's amd64 Moscow ML package.

Caddy reads the mounted `deploy/Caddyfile` and watches for edits, including editor
saves that replace the file. `TRYMOSML_SITE=http://:80` selects local HTTP; without
that override the same file serves `try.mosml.org` over HTTPS. The web root and
backend address match deployment. Invalid edits leave the last valid configuration
running; check `docker logs trymosml-dev` for errors.

Go reads the mounted `deploy/webshell.json` at startup, using the same paths and
session limits as deployment. Restart the container to apply JSON changes; no
image rebuild is needed.

Changes to Go, `dev/`, or `docker-build/` require a new development image.
Stop the container and repeat the root README's build and run commands.
`./dev/build.sh` builds the existing `docker-build/Dockerfile` with the host's
Docker cache, then bundles the REPL image into the development image. Builds need
internet access for missing dependencies; startup only loads the bundled image.
To discard the inner image cache, stop the container and run
`docker volume rm trymosml-dev-docker`; the next run loads it again.

Check the running site, assets, proxy, and REPL:

```sh
docker exec trymosml-dev /opt/trymosml/bin/smoke \
  -url ws://127.0.0.1/webshell
```

Run the existing Go suite, including Docker session tests, in a separate container
so browser sessions cannot interfere with cleanup assertions:

```sh
docker run --rm --privileged --stop-timeout=45 -e TRYMOSML_DOCKER_TEST=1 \
  trymosml/dev /opt/trymosml/bin/session-tests -test.v
```

The entrypoint loads the bundled image into the inner daemon, then uses
`deploy/repl.py` with a development image tag. The launcher applies the same
session resource limits and cleanup behavior as the server deployment. Startup
and shutdown clean up managed sessions, and Docker stops after the web services.
An unexpected daemon or web-service exit stops the development container.

On the host, run `go test -race ./...` from `webshell/` for race detection.
