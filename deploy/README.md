# Deployment

Caddy owns ports 80 and 443, manages HTTPS, and serves static files from
/opt/trymosml/web. It sends `.sml` examples as `text/plain; charset=utf-8`.
Only /webshell and /healthz are proxied to the Go server on 127.0.0.1:8080,
which launches one Docker REPL per WebSocket connection. The page and examples
remain available while Go restarts. Both services run under systemd; logs go
to the journal. Caddy follows the active release symlink on updates and rollback.

`deploy/Caddyfile` defaults to `try.mosml.org`. Local development uses the same
file with `TRYMOSML_SITE=http://:80` to serve HTTP inside the development container.

The REPL image uses Debian 13 (`trixie-slim`), with the base pinned by digest in
docker-build/Dockerfile. The build verifies the repository's Moscow ML package,
applies available Debian package updates, and installs Moscow ML 2.10.1 for amd64.
The compiler binary is unchanged; a supported base does not imply that the old
compiler itself receives security maintenance.

Each application release contains deploy/repl-image.json, which identifies its
image tag, exported archive checksum, image configuration and filesystem layers.
The loader verifies those contents before and after Docker import, supporting
both classic and containerd image stores. Existing tags with different contents
are rejected. The launcher never pulls images automatically.

On 2026-11-12, after verification of the Debian 13 deployment, the owner requested
removal of the old releases, legacy/test REPL images, migration backup archives,
and the pre-rebuild DigitalOcean snapshot. Those copies have been deleted locally
and on the server as applicable. Subsequent application deployments can create
a new previous-release rollback target; they do not restore those deleted backups.

## Build and transfer the REPL image

```sh
./deploy/build-repl.sh
```

This builds linux/amd64 locally with a unique timestamped tag and exports a
compressed Docker archive under dist/. It updates deploy/repl-image.json for the
next application bundle. It never pushes to a registry. An explicit unique tag
can be supplied, for example `trymosml/repl:trixie-20261112-r1`. Existing tags and
archives are not overwritten.

The current release uses `trymosml/repl:trixie-20261112-r1`, exported as
`dist/repl-trixie-20261112-r1.tar.gz`. Copy that archive alongside the application
bundle, unpack the application, and use its loader before running install.sh.
The manifest and archive should be transferred over the same trusted SSH
connection. Checksums detect mismatches, not an attacker replacing both files.

Rebuild to incorporate Debian updates; running containers do not update
themselves. The build bypasses cached apt layers. Refresh the pinned base digest
deliberately, rerun integration tests, and retain the exported image for exact
rollback: apt repositories change, so rebuilding is not bit-for-bit reproducible.

## Limits and cleanup

Defaults are four simultaneous sessions, 30 minutes maximum lifetime, ten minutes
without submitted input, and 1 MiB maximum WebSocket message. Ping/pong detects
dead connections. Expiry closes the session and discards its scratch files.
Each session queues at most eight input messages while its interpreter is busy;
overflow closes the session. A separate input writer keeps WebSocket ping and
disconnect handling responsive. A normal close is acknowledged only after the
REPL is stopped and its slot is released, so Reset can reconnect at capacity.
The browser enables Run only after the interpreter acknowledges initialization,
including after Reset; opening the WebSocket alone does not mean it is ready.
Run sends the filename and UTF-8 source as escaped SML strings in one command;
it does not depend on a timed second message or mix lexer input with raw stdin.

Each container has 32 MiB RAM, no swap, half a CPU, 32 processes, 128 file
descriptors, no network, no Linux capabilities, and no privilege escalation.
It runs as uid/gid 4242 with a read-only root and two bounded 8 MiB temporary
filesystems. A container-side 30-minute timeout also bounds a REPL if its launcher
dies. The launcher removes its exact named container in a finally block.
Service startup and shutdown remove only containers labeled
`org.trymosml.managed=true`, including leftovers from crashes.

These controls reduce resource abuse; Docker shares the host kernel and is not a
complete security boundary for hostile code. The service user's Docker socket
access is effectively privileged even though the Go process itself is not root.
Do not grant Caddy access to that socket.

Configuration is /etc/trymosml/webshell.json. Optional launcher overrides can be
placed in /etc/trymosml/environment (TRYMOSML_REPL_IMAGE, TRYMOSML_DOCKER).
If changing lifetime, keep the container timeout in repl.py aligned.
The old Rootdir configuration key is ignored; static-file roots belong in Caddy.

## Prepare and verify locally

Use Go 1.27.4 or a newer security-patched release. The Makefile downloads 1.27.4;
the package script uses Go from PATH.

```sh
# After building the image (substitute the archive name printed by the build):
python3 deploy/load-repl.py dist/repl-trixie-20261112-r1.tar.gz
cd webshell
go test -race ./...
TRYMOSML_DOCKER_TEST=1 go test -race -run TestDocker -count=1
cd ..
./deploy/package.sh
```

Installer failure and rollback tests run in a disposable local container, with
real filesystem operations and simulated service commands:

```sh
docker run --rm -v "$PWD:/src:ro" python:3.14-slim python /src/deploy/tests/install_test.py
```

The optional Docker test requires the release image and a local Docker daemon. It
evaluates arithmetic, writes and loads an SML file, starts an infinite computation,
disconnects, and checks that its container was removed. It also checks that the
compiler's GMP-backed IntInf extension works with the current libraries. Additional Docker tests
cover expiry, server shutdown, Reset at capacity, and disconnect with blocked
input during an infinite computation. Run in an environment with
no other sessions using the application's management label.

The output is dist/trymosml-linux-amd64.tar.gz, with a static Linux executable,
web assets, and deployment files. Local backups and artifacts are gitignored.
The original migration backups and provider snapshot have been deleted. Current
build artifacts remain under dist/. Certificate private keys were never copied
into this repository.

## Rebuild the same DigitalOcean Droplet

Target verified on 2026-11-12: Droplet 2054497 (try-mosml), 146.185.185.49,
ams2, 512 MiB RAM, 20 GiB disk. Recheck identity before any destructive action.

The host now runs Ubuntu 26.04 at its original IP. The pre-migration snapshot
254259636 (`trymosml-before-caddy-2026-11-12`) was deleted at the owner's request.
Before any future rebuild, create and verify a new backup; the following steps
describe that future process, not an available recovery point.

1. Verify local backup checksums and the deployment tests. Confirm provider
   Recovery Console access and a usable SSH key or password recovery path.
2. Shut down cleanly, then create a named DigitalOcean snapshot and wait for the
   action to complete. Record its image ID outside the Droplet. A snapshot is
   disk-only: it does not preserve the running screen session.
3. Rebuild this same Droplet using Ubuntu 26.04 amd64, retaining its IP. Rebuilding
   erases the disk. Never perform this step without the completed snapshot and
   off-server application/image backups.
4. Verify the rebuilt host's SSH fingerprint through the provider console before
   accepting the changed key. Check SSH access before proceeding.
5. Copy the deployment archive and matching REPL image archive to /root on the new host,
   verify their checksums, and unpack the application archive.
6. Bootstrap the OS, load the release image, and install the services:

```sh
tar -xzf /root/trymosml-linux-amd64.tar.gz -C /root
/root/trymosml/deploy/bootstrap-ubuntu.sh
python3 /root/trymosml/deploy/load-repl.py /root/repl-trixie-20261112-r1.tar.gz
# Fresh host only: replace the packaged Caddy welcome-page configuration.
install -m 644 /root/trymosml/deploy/Caddyfile /etc/caddy/Caddyfile
/root/trymosml/deploy/install.sh
```

The bootstrap script adds 1 GiB swap on this small host without changing its paid
plan, applies OS updates, installs Ubuntu's Docker package and Caddy's official repository package,
and bounds journal storage. It is intended for a fresh Ubuntu 26.04 amd64 host.

Ensure host/provider firewalls allow SSH and TCP 80/443. The backend must not be
publicly reachable on 8080. Caddy obtains a new certificate; its managed state is
under /var/lib/caddy and must persist across application updates.

After verifying a fresh SSH key login, disable SSH password authentication using
/etc/ssh/sshd_config.d/00-trymosml-key-only.conf:

```text
PubkeyAuthentication yes
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin prohibit-password
```

Validate with `sshd -t`, reload `ssh.service`, and verify another key login while
keeping the original connection open. Use a timed rollback when doing this
remotely. Console password recovery is unaffected. This was applied and verified
on the production host on 2026-11-12. The installer never changes SSH settings.

## Verify the deployment

```sh
systemctl status caddy trymosml docker
curl --fail http://127.0.0.1:8080/healthz
curl -I http://try.mosml.org/
curl -I https://try.mosml.org/
ss -lntp
journalctl -u trymosml -u caddy --since '10 minutes ago'
```

From the development machine:

```sh
cd webshell
go run ./cmd/smoke -url wss://try.mosml.org/webshell
```

The smoke command checks the page, JavaScript, CSS, all three examples (including
their text MIME type), the proxied health endpoint, and REPL arithmetic. Use
`-static-only` to verify the static site while the Go service is stopped.

Also exercise the browser's Run action and Reset, confirm session cleanup, restart
trymosml with an active session, and reboot the host once to verify automatic
startup. /healthz checks the HTTP server only; the WebSocket smoke test checks
Docker and Moscow ML too.

## Updates and rollback

Build a new bundle locally, copy/unpack it under /root, and run deploy/install.sh.
It stages a fresh directory under /opt/trymosml-releases, validates the effective
Caddy and application configuration, checks that the Caddy user can read all
staged web assets, and checks the candidate's image before stopping the
application. It then switches /opt/trymosml to the staged release using a symlink,
starts the service and checks HTTP health. Obsolete application files are not
carried into a new release. An installer lock prevents overlapping deployments.

Existing /etc/trymosml/webshell.json, environment overrides, /etc/caddy/Caddyfile,
the systemd unit and unit drop-ins are preserved. Bundled configuration and unit
files are defaults for files that do not yet exist. Review and apply intentional
configuration or unit changes separately; they are not silently installed during
an application update. When migrating from the original proxy-all Caddyfile,
validate and install the new Caddyfile and reload Caddy **before** installing
the backend that no longer serves static files. Confirm static checks pass;
keep the previous Caddyfile until the migration is verified. Caddy needs read
access to the web files and traversal access to their parent directories, but
no Docker access. Image preflight runs as the service user with the same
environment file, and checks the default image's layers and configuration against
the release manifest. Explicit TRYMOSML_REPL_IMAGE overrides are instead checked
for availability and platform. Keep Caddy's data directory intact. The installer requires
an explicit loopback listening port and a launcher path available in the release
or at its configured absolute path.

Installing stops active REPL sessions. Startup, health-check or Caddy-reload
failures restore the previous application release and attempt to restart it.
This recovery handles ordinary command failures and termination signals, not
power loss or SIGKILL. Inspect service status after any failed deployment. The
installer's HTTP health check does not replace the WebSocket smoke test above.

Successful deployments create /opt/trymosml-previous pointing to the prior
release. The earlier cleanup removed the pointer that existed then. When a
previous release is available, roll back using:

```sh
/opt/trymosml/deploy/install.sh --rollback
```

Rollback validates and switches to the previous release, retaining the current
configuration. It also exchanges the current and previous pointers, so the same
command can switch forward again. Each release's manifest restores its default
image choice; images must remain loaded under their original tags. An explicit
TRYMOSML_REPL_IMAGE environment override takes precedence and must be changed
separately if needed. Rollback does not restore separately edited configuration.
Releases, including failed candidates, are retained;
review disk usage and remove unused releases manually, preserving both symlink
targets. Without a previous-release pointer, --rollback exits without changing
the running service. Recovery to the original Ubuntu 16.04 installation is no
longer available through the deleted snapshot or migration archives.

References:
- https://docs.digitalocean.com/products/droplets/how-to/rebuild/
- https://docs.digitalocean.com/products/snapshots/how-to/snapshot-droplets/
- https://caddyserver.com/docs/install
- https://caddyserver.com/docs/caddyfile/directives/reverse_proxy
