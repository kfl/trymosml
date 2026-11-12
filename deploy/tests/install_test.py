"""Installer integration tests. Run ONLY in the disposable container below.

docker run --rm -v "$PWD:/src:ro" python:3.14-slim \
    python /src/deploy/tests/install_test.py

The installer uses real filesystem operations at production paths in the container.
Service, Docker, Caddy and HTTP commands are replaced with controlled test doubles.
"""
import json
import os
import pathlib
import shutil
import subprocess
import unittest

P = pathlib.Path


class InstallTests(unittest.TestCase):
    def setUp(self):
        self.assertTrue(P('/.dockerenv').exists(), 'Disposable Docker container required')
        self.assertEqual(os.geteuid(), 0)
        for path in ('/opt/trymosml', '/opt/trymosml-previous', '/opt/trymosml-releases',
                     '/etc/trymosml', '/etc/caddy', '/tmp/install-fixture'):
            p = P(path)
            if p.is_symlink():
                p.unlink()
            elif p.exists():
                shutil.rmtree(p)
        P('/etc/systemd/system').mkdir(parents=True, exist_ok=True)
        P('/etc/systemd/system/trymosml.service').unlink(missing_ok=True)
        self.base = P('/tmp/install-fixture')
        self.bundle = self.base / 'bundle'
        shutil.copytree('/src/deploy', self.bundle / 'deploy')
        (self.bundle / 'web').mkdir()
        (self.bundle / 'web/index.html').write_text('new site')
        (self.bundle / 'webshell').mkdir()
        self.executable(self.bundle / 'webshell/webshell', '#!/bin/sh\nexit 0\n')
        self.log = self.base / 'commands'
        bindir = self.base / 'bin'
        bindir.mkdir()
        self.executable(bindir / 'mock', '''#!/bin/sh
name=${0##*/}
echo "$name $*" >> "$TEST_LOG"
case "$name" in
  caddy) [ "${FAIL_CADDY:-}" != 1 ] ;;
  curl) [ "${FAIL_HEALTH:-}" != 1 ] ;;
  runuser)
    [ "${FAIL_STATIC_ACCESS:-}" != 1 ] || exit 1
    shift 3
    exec "$@" ;;
  docker)
    [ "${FAIL_IMAGE:-}" != 1 ] || exit 1
    python3 -c 'import json,os; m=json.load(open(os.environ["TEST_IMAGE_MANIFEST"])); print(json.dumps([dict(Architecture="amd64",Os="linux",RootFS=dict(Layers=m["layers"]),Config=m["config"])]))'
    ;;
  systemd-run)
    while [ "$#" -gt 0 ]; do
      case "$1" in */repl.py) exec "$@" ;; esac
      shift
    done
    exit 2 ;;
  systemctl)
    if [ "$*" = "start trymosml" ]; then
      readlink -f /opt/trymosml > "$TEST_STARTED_RELEASE"
    fi
    if [ "$*" = "reload caddy" ] && [ "${FAIL_RELOAD:-}" = 1 ]; then exit 1; fi
    exit 0 ;;
  *) exit 0 ;;
esac
''')
        for name in ('systemctl', 'systemd-run', 'docker', 'caddy', 'curl', 'id', 'runuser'):
            (bindir / name).symlink_to('mock')
        self.env = dict(os.environ, PATH=str(bindir) + ':' + os.environ['PATH'],
                        TEST_LOG=str(self.log), TEST_STARTED_RELEASE=str(self.base / 'started'),
                        TRYMOSML_DOCKER=str(bindir / 'docker'),
                        TEST_IMAGE_MANIFEST=str(self.bundle / 'deploy/repl-image.json'))
        P('/run/lock').mkdir(parents=True, exist_ok=True)

    def executable(self, path, text):
        path.write_text(text)
        path.chmod(0o755)

    def legacy(self):
        shutil.copytree(self.bundle, '/opt/trymosml')
        P('/opt/trymosml/deploy/repl-image.json').unlink()
        P('/opt/trymosml/web/obsolete.html').write_text('old only')
        P('/opt/trymosml/web/index.html').write_text('old site')
        P('/etc/trymosml').mkdir()
        cfg = json.loads((self.bundle / 'deploy/webshell.json').read_text())
        cfg['MaxSessions'] = 2
        P('/etc/trymosml/webshell.json').write_text(json.dumps(cfg))
        P('/etc/trymosml/environment').write_text('# local settings\n')
        P('/etc/caddy').mkdir()
        P('/etc/caddy/Caddyfile').write_text('# custom Caddy\n')
        P('/etc/systemd/system/trymosml.service').write_text('# custom unit\n')
        return {p: P(p).read_bytes() for p in (
            '/etc/trymosml/webshell.json', '/etc/trymosml/environment',
            '/etc/caddy/Caddyfile', '/etc/systemd/system/trymosml.service')}

    def run_install(self, *args, **overrides):
        result = subprocess.run(['bash', str(self.bundle / 'deploy/install.sh'), *args],
                                env=dict(self.env, **overrides), capture_output=True, text=True)
        self.last_output = result.stdout + result.stderr
        return result.returncode

    def commands(self):
        return self.log.read_text() if self.log.exists() else ''

    def test_upgrade_preserves_config_and_rollback_restores_entire_release(self):
        configs = self.legacy()
        self.assertEqual(self.run_install(), 0, self.last_output)
        first = P('/opt/trymosml').resolve()
        old = P('/opt/trymosml-previous').resolve()
        self.assertTrue(P('/opt/trymosml').is_symlink())
        self.assertFalse(P('/opt/trymosml/web/obsolete.html').exists())
        self.assertTrue((old / 'web/obsolete.html').exists())
        for path, content in configs.items():
            self.assertEqual(P(path).read_bytes(), content)
        (self.bundle / 'web/index.html').write_text('second release')
        manifest_path = self.bundle / 'deploy/repl-image.json'
        manifest = json.loads(manifest_path.read_text())
        first_image = manifest['image']
        manifest['image'] = 'trymosml/repl:trixie-second-test-release'
        manifest_path.write_text(json.dumps(manifest))
        self.assertEqual(self.run_install(), 0, self.last_output)
        self.assertIn('docker image inspect ' + manifest['image'], self.commands())
        second = P('/opt/trymosml').resolve()
        self.assertEqual(P('/opt/trymosml-previous').resolve(), first)
        self.assertEqual(self.run_install('--rollback'), 0, self.last_output)
        self.assertEqual(P('/opt/trymosml').resolve(), first)
        self.assertEqual(P('/opt/trymosml-previous').resolve(), second)
        self.assertEqual((self.base / 'started').read_text().strip(), str(first))
        self.assertIn('docker image inspect ' + first_image, self.commands().split('systemctl reload caddy')[-2])

    def test_rollback_to_release_without_manifest(self):
        self.legacy()
        self.assertEqual(self.run_install(), 0, self.last_output)
        self.assertEqual(self.run_install('--rollback'), 0, self.last_output)
        self.assertIn('docker image inspect trymosml/repl:legacy-20160924', self.commands())
        self.assertEqual(P('/opt/trymosml/web/index.html').read_text(), 'old site')

    def test_missing_image_does_not_stop_running_service(self):
        self.legacy()
        self.assertNotEqual(self.run_install(FAIL_IMAGE='1'), 0)
        self.assertNotIn('systemctl stop', self.commands())

    def test_invalid_caddy_does_not_stop_running_service(self):
        self.legacy()
        self.assertNotEqual(self.run_install(FAIL_CADDY='1'), 0)
        self.assertNotIn('systemctl stop', self.commands())
        self.assertFalse(P('/opt/trymosml').is_symlink())

    def test_invalid_application_config_does_not_stop_running_service(self):
        self.legacy()
        P('/etc/trymosml/webshell.json').write_text('{"MaxSessions":0}')
        self.assertNotEqual(self.run_install(), 0)
        self.assertNotIn('systemctl stop', self.commands())

    def test_unreadable_static_files_do_not_stop_running_service(self):
        self.legacy()
        self.assertNotEqual(self.run_install(FAIL_STATIC_ACCESS='1'), 0)
        self.assertNotIn('systemctl stop', self.commands())

    def test_failed_health_restores_legacy_release(self):
        configs = self.legacy()
        self.assertNotEqual(self.run_install(FAIL_HEALTH='1'), 0)
        self.assertEqual(P('/opt/trymosml/web/index.html').read_text(), 'old site')
        self.assertTrue(P('/opt/trymosml/web/obsolete.html').exists())
        self.assertEqual((self.base / 'started').read_text().strip(), str(P('/opt/trymosml').resolve()))
        for path, content in configs.items():
            self.assertEqual(P(path).read_bytes(), content)

    def test_failed_reload_restores_current_and_preserves_previous(self):
        self.legacy()
        self.assertEqual(self.run_install(), 0, self.last_output)
        current, previous = P('/opt/trymosml').resolve(), P('/opt/trymosml-previous').resolve()
        self.assertNotEqual(self.run_install(FAIL_RELOAD='1'), 0)
        self.assertEqual(P('/opt/trymosml').resolve(), current)
        self.assertEqual(P('/opt/trymosml-previous').resolve(), previous)
        self.assertEqual((self.base / 'started').read_text().strip(), str(current))

    def test_fresh_install_and_failed_fresh_install(self):
        self.assertNotEqual(self.run_install(FAIL_HEALTH='1'), 0)
        self.assertFalse(P('/opt/trymosml').exists())
        self.assertFalse(P('/etc/trymosml/webshell.json').exists())
        self.assertFalse(P('/etc/systemd/system/trymosml.service').exists())
        self.assertEqual(self.run_install(), 0, self.last_output)
        self.assertTrue(P('/opt/trymosml').is_symlink())
        self.assertTrue(P('/etc/trymosml/webshell.json').exists())


if __name__ == '__main__':
    unittest.main(verbosity=2)
