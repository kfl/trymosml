"""Run a private Docker daemon, Caddy, and the existing Go/Docker REPL stack."""
import os
from pathlib import Path
import signal
import subprocess
import sys
import time


def stop_process(process, timeout):
    if process is not None and process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()


def main():
    if not Path('/.dockerenv').exists():
        raise RuntimeError('Run this script inside the development container')
    stopping = False
    daemon = None
    children = []

    def stop(signum, frame):
        nonlocal stopping
        stopping = True

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)

    def check_running():
        if stopping:
            raise InterruptedError('Stopping development container')
        if daemon.poll() is not None:
            raise RuntimeError('The inner Docker daemon exited; check its logs above')

    def spawn(command):
        process = subprocess.Popen(command)
        children.append(process)
        return process

    def wait(process, timeout):
        deadline = time.monotonic() + timeout
        while process.poll() is None:
            check_running()
            if time.monotonic() > deadline:
                raise TimeoutError(f'Command timed out: {process.args}')
            time.sleep(0.1)
        if process.returncode:
            raise subprocess.CalledProcessError(process.returncode, process.args)

    try:
        # Supplying "dockerd" explicitly avoids the image's default TCP listeners.
        # REPL sessions always use --network=none.
        # Its nested Tini must be a subreaper because our own init is PID 1.
        daemon = subprocess.Popen([
            'dockerd-entrypoint.sh', 'dockerd', '--host=unix:///var/run/docker.sock',
            '--bridge=none', '--iptables=false', '--ip6tables=false',
            '--ip-forward=false', '--ip-masq=false', '--log-level=warn',
        ], env=dict(os.environ, TINI_SUBREAPER='1'))
        deadline = time.monotonic() + 30
        while True:
            check_running()
            try:
                ready = subprocess.run(['docker', 'info'], stdout=subprocess.DEVNULL,
                                       stderr=subprocess.DEVNULL, timeout=2).returncode == 0
            except subprocess.TimeoutExpired:
                ready = False
            if ready:
                break
            if time.monotonic() > deadline:
                raise TimeoutError('Inner Docker daemon did not become ready')
            time.sleep(0.2)
        print('Loading the bundled Moscow ML image...', flush=True)
        wait(spawn(['docker', 'load', '--input', '/opt/trymosml/repl.tar']), 120)
        wait(spawn(['/opt/trymosml/deploy/repl.py', '--check-image']), 20)
        wait(spawn(['/opt/trymosml/deploy/repl.py', '--cleanup']), 20)
        if sys.argv[1:]:
            command = spawn(sys.argv[1:])
            while command.poll() is None:
                check_running()
                time.sleep(0.1)
            return command.returncode
        backend = spawn(['/opt/trymosml/bin/webshell', '-conf', '/opt/trymosml/deploy/webshell.json'])
        caddy = spawn(['caddy', 'run', '--config', '/opt/trymosml/deploy/Caddyfile',
                       '--adapter', 'caddyfile', '--watch'])
        print('Try Moscow ML is starting on port 80.', flush=True)
        while backend.poll() is None and caddy.poll() is None:
            check_running()
            time.sleep(0.1)
        raise RuntimeError('A web service exited unexpectedly')
    except InterruptedError:
        return 0
    finally:
        # Keep Docker alive until the backend has reaped its active sessions.
        for process in reversed(children):
            stop_process(process, 10)
        if daemon is not None and daemon.poll() is None:
            try:
                subprocess.run(['/opt/trymosml/deploy/repl.py', '--cleanup'], check=True, timeout=15)
            except (OSError, subprocess.SubprocessError) as error:
                print(f'REPL cleanup: {error}', file=sys.stderr)
        stop_process(daemon, 10)


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, RuntimeError, subprocess.SubprocessError) as error:
        print(f'Development startup failed: {error}', file=sys.stderr)
        sys.exit(1)
