# Deployment

Caddy owns ports 80 and 443 and manages HTTPS. The Go server binds only
127.0.0.1:8080, serves static files and /webshell, and launches one Docker REPL
per connection. Both services run under systemd; logs go to the journal.

The initial migration preserves the working Moscow ML 2.10 image from production:
`sha256:d10ec855f06b6cb33c9b86a0b5440c2b924c9fbb7933cf1b63efc819c11fedce`.
The loader verifies the archive checksum, image configuration, and filesystem
layers, then assigns the local tag `trymosml/repl:legacy-20160924`. This works
with both classic and containerd image stores, which expose different image IDs.
The launcher never pulls images automatically.

It is amd64 and based on Debian Wheezy. It is a compatibility artifact, not a
maintained base image. Do not rebuild the historical docker-build/Dockerfile
against current package mirrors or assume the public latest tag is equivalent.
A replacement REPL image is separate follow-up work.

## Limits and cleanup

Defaults are four simultaneous sessions, 30 minutes maximum lifetime, ten minutes
without submitted input, and 1 MiB maximum WebSocket message. Ping/pong detects
dead connections. Expiry closes the session and discards its scratch files.

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

## Prepare and verify locally

Use Go 1.27.4 or a newer security-patched release. The Makefile downloads 1.27.4;
the package script uses Go from PATH.

```sh
python3 deploy/load-repl.py deployment-backups/2026-11-12/legacy-repl-image.tar.gz
cd webshell
go test -race ./...
TRYMOSML_DOCKER_TEST=1 go test -race -run TestDocker -count=1
cd ..
./deploy/package.sh
```

The optional Docker test requires the saved image and a local Docker daemon. It
evaluates arithmetic, writes and loads an SML file, starts an infinite computation,
disconnects, and checks that its container was removed. Additional Docker tests
cover expiry and server shutdown during an infinite computation. Run in an environment with
no other sessions using the application's management label.

The output is dist/trymosml-linux-amd64.tar.gz, with a static Linux executable,
web assets, and deployment files. Local backups and artifacts are gitignored.
Backups include the old application and Docker image; certificate private keys
are preserved in the provider snapshot rather than copied into this repository.

## Rebuild the same DigitalOcean Droplet

Target verified on 2026-11-12: Droplet 2054497 (try-mosml), 146.185.185.49,
ams2, 512 MiB RAM, 20 GiB disk. Recheck identity before any destructive action.

The completed pre-migration snapshot is **254259636**, named
**trymosml-before-caddy-2026-11-12**. It contains the Ubuntu 16.04 installation.
The rebuild target is Ubuntu 26.04; the Droplet IP is unchanged.

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
5. Copy the deployment archive and saved REPL image to /root on the new host,
   verify their checksums, and unpack the application archive.
6. Bootstrap the OS, load the saved image, and install the services:

```sh
tar -xzf /root/trymosml-linux-amd64.tar.gz -C /root
/root/trymosml/deploy/bootstrap-ubuntu.sh
python3 /root/trymosml/deploy/load-repl.py /root/legacy-repl-image.tar.gz
/root/trymosml/deploy/install.sh
```

The bootstrap script adds 1 GiB swap on this small host without changing its paid
plan, applies OS updates, installs Ubuntu's Docker package and Caddy's official repository package,
and bounds journal storage. It is intended for a fresh Ubuntu 26.04 amd64 host.

Ensure host/provider firewalls allow SSH and TCP 80/443. The backend must not be
publicly reachable on 8080. Caddy obtains a new certificate; its managed state is
under /var/lib/caddy and must persist across application updates.

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

Also exercise the browser's Run action and Reset, confirm session cleanup, restart
trymosml with an active session, and reboot the host once to verify automatic
startup. /healthz checks the HTTP server only; the WebSocket smoke test checks
Docker and Moscow ML too.

## Updates and rollback

Build a new bundle locally, retain the previous bundle, copy/unpack it under /root,
and run deploy/install.sh. Installing stops active REPL sessions and replaces
the application configuration. Review configuration overrides before doing so.
Do not replace Caddy's data directory.

For an application-only rollback, reinstall the previous known-good bundle.
For failure during this initial migration, restore the pre-migration DigitalOcean
snapshot to Droplet 2054497, boot it, then launch the old application:

```sh
cd /root/trymosml
screen -S trymosml
./webshell/webshell -conf docker-conf.json 2>&1 | tee -a /root/trymosml-log.txt
```

Detach with Ctrl-A then D. Verify HTTPS and a REPL. Restoring a snapshot overwrites
changes since that snapshot; the old Go server again owns ports 80/443/8080.
Keep the snapshot until the modern deployment has passed reboot and REPL checks.

References:
- https://docs.digitalocean.com/products/droplets/how-to/rebuild/
- https://docs.digitalocean.com/products/snapshots/how-to/snapshot-droplets/
- https://caddyserver.com/docs/install
- https://caddyserver.com/docs/caddyfile/directives/reverse_proxy
