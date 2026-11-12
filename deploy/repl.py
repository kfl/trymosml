#!/usr/bin/env python3
"""Launch one bounded REPL; remove its named container on every exit path."""
import os
import signal
import subprocess
import sys
import time
import uuid

LABEL = "org.trymosml.managed=true"
IMAGE = os.environ.get("TRYMOSML_REPL_IMAGE", "trymosml/repl:legacy-20160924")
DOCKER = os.environ.get("TRYMOSML_DOCKER", "/usr/bin/docker")


def cleanup_all():
    # Only this application's containers, including remnants of a crashed service.
    ids = subprocess.check_output([DOCKER, "ps", "-aq", "--filter", "label=" + LABEL],
                                  timeout=10).decode().split()
    for cid in ids:
        subprocess.run([DOCKER, "rm", "-f", cid], check=True, timeout=10)


def run():
    name = "trymosml-" + uuid.uuid4().hex
    stopping = False
    attach = None

    def stop(signum, frame):
        nonlocal stopping
        stopping = True

    for sig in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP):
        signal.signal(sig, stop)

    try:
        subprocess.run([
            DOCKER, "create", "--pull=never", "--name", name, "--label", LABEL,
            "--platform", "linux/amd64", "--interactive", "--init",
            "--network", "none", "--memory", "32m", "--memory-swap", "32m",
            "--cpus", "0.5", "--pids-limit", "32",
            "--ulimit", "nofile=128:128", "--cap-drop", "ALL",
            "--security-opt", "no-new-privileges", "--read-only",
            "--user", "4242:4242", "--hostname", "mosmlserver",
            "--tmpfs", "/home/mosmluser:rw,nosuid,nodev,noexec,size=8m,uid=4242,gid=4242",
            "--tmpfs", "/tmp:rw,nosuid,nodev,noexec,size=8m",
            "--log-driver", "none", "--entrypoint", "/usr/bin/timeout",
            IMAGE, "-s", "KILL", "1800", "/usr/bin/mosml", "-P", "full", "-liberal",
        ], check=True, stdout=subprocess.DEVNULL, timeout=3)
        if stopping:
            return 0
        attach = subprocess.Popen([DOCKER, "start", "--attach", "--interactive", name])
        while not stopping:
            result = attach.poll()
            if result is not None:
                return result
            time.sleep(0.1)
        return 0
    finally:
        # Killing docker's attached client alone does not stop the container.
        try:
            subprocess.run([DOCKER, "rm", "-f", name], check=True,
                           stdout=subprocess.DEVNULL, timeout=3)
        finally:
            if attach is not None:
                try:
                    attach.wait(timeout=1)
                except subprocess.TimeoutExpired:
                    attach.kill()
                    attach.wait()


if __name__ == "__main__":
    try:
        if sys.argv[1:] == ["--cleanup"]:
            cleanup_all()
        elif sys.argv[1:]:
            raise ValueError("usage: repl.py [--cleanup]")
        else:
            sys.exit(run())
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        print("REPL launcher: " + str(error), file=sys.stderr)
        sys.exit(1)
