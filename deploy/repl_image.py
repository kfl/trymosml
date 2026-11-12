"""Image manifest and verification shared by the launcher and offline loader."""
import hashlib
import json
from pathlib import Path
import subprocess
import tarfile

CONFIG_FIELDS = ('Env', 'WorkingDir', 'Entrypoint', 'Cmd', 'User', 'Labels')


def sha256(path):
    digest = hashlib.sha256()
    with Path(path).open('rb') as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b''):
            digest.update(chunk)
    return digest.hexdigest()


def read_manifest():
    data = json.loads(Path(__file__).with_name('repl-image.json').read_text())
    if data['version'] != 1 or data['platform'] != 'linux/amd64':
        raise ValueError('Unsupported REPL image manifest')
    return data


def archive_info(path):
    with tarfile.open(path) as archive:
        entries = json.load(archive.extractfile('manifest.json'))
        if len(entries) != 1 or len(entries[0]['RepoTags']) != 1:
            raise ValueError('Expected exactly one image with one tag')
        config_bytes = archive.extractfile(entries[0]['Config']).read()
        config = json.loads(config_bytes)
    if config['architecture'] != 'amd64' or config['os'] != 'linux':
        raise ValueError('Expected linux/amd64 REPL image')
    return dict(version=1, platform='linux/amd64', image=entries[0]['RepoTags'][0],
                archive=Path(path).name, archive_sha256=sha256(path),
                config_sha256=hashlib.sha256(config_bytes).hexdigest(),
                layers=config['rootfs']['diff_ids'],
                config={key: config['config'].get(key) for key in CONFIG_FIELDS})


def verify_local_image(image, docker='docker', manifest=None):
    loaded = json.loads(subprocess.check_output(
        [docker, 'image', 'inspect', image], timeout=15))[0]
    if loaded['Architecture'] != 'amd64' or loaded['Os'] != 'linux':
        raise ValueError('Loaded image platform differs from linux/amd64')
    # Explicit operator overrides are checked for platform and availability.
    # The release's default image also has its filesystem and configuration checked.
    if manifest is not None and image == manifest['image']:
        if loaded['RootFS']['Layers'] != manifest['layers']:
            raise ValueError('Loaded image filesystem differs from release manifest')
        for key in CONFIG_FIELDS:
            if loaded['Config'].get(key) != manifest['config'].get(key):
                raise ValueError('Loaded image configuration differs: ' + key)
    return loaded
