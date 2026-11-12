# Try Moscow ML

A browser editor and Standard ML console, with Caddy, a Go backend, and Moscow ML.

## Run locally

Requires Docker. From the repository root:

```sh
./dev/build.sh
docker run --rm -d --name trymosml-dev \
  --privileged --stop-timeout=45 \
  -p 127.0.0.1:18080:80 \
  --mount type=volume,source=trymosml-dev-docker,target=/var/lib/docker \
  --mount "type=bind,source=$PWD/web,target=/opt/trymosml/web,readonly" \
  --mount "type=bind,source=$PWD/deploy,target=/opt/trymosml/deploy,readonly" \
  trymosml/dev
```

Startup loads the bundled REPL image; the named volume keeps its layers between runs.
Follow progress with `docker logs -f trymosml-dev`.
Then open **http://localhost:18080/**. Edit `web/` and refresh. Edits to
`deploy/Caddyfile` reload automatically. After editing `deploy/webshell.json`, run
`docker restart -t 45 trymosml-dev`.

Stop with:

```sh
docker stop trymosml-dev
```

After changes to Go, `dev/`, or `docker-build/`, stop the container, then repeat
the build and run commands above. Restarting an existing container keeps its old image.

One container runs Caddy, Go, and Docker-in-Docker, using the existing Docker
session launcher. DinD requires `--privileged`. See [development checks](dev/README.md) and
[server deployment](deploy/README.md).
