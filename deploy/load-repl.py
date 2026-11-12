#!/usr/bin/env python3
"""Verify and load the archived production REPL, then assign a portable local tag."""
import hashlib
import json
import pathlib
import subprocess
import sys
import tarfile

ARCHIVE_SHA256 = "26aeaa5762b388529f1e9e7f1783958bd07dc571333a39da5535573a31e6850f"
CONFIG_SHA256 = "d10ec855f06b6cb33c9b86a0b5440c2b924c9fbb7933cf1b63efc819c11fedce"
SOURCE_TAG = "kflarsen/trymosml:latest"
TARGET_TAG = "trymosml/repl:legacy-20160924"


def main():
    if len(sys.argv) != 2:
        raise ValueError("usage: load-repl.py legacy-repl-image.tar.gz")
    path = pathlib.Path(sys.argv[1])
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    if digest.hexdigest() != ARCHIVE_SHA256:
        raise ValueError("REPL archive checksum does not match the production backup")
    with tarfile.open(path) as archive:
        manifest = json.load(archive.extractfile("manifest.json"))
        if len(manifest) != 1 or manifest[0]["RepoTags"] != [SOURCE_TAG]:
            raise ValueError("unexpected image manifest")
        config_bytes = archive.extractfile(manifest[0]["Config"]).read()
        if hashlib.sha256(config_bytes).hexdigest() != CONFIG_SHA256:
            raise ValueError("unexpected image configuration")
        config = json.loads(config_bytes)
    subprocess.run(["docker", "load", "-i", str(path)], check=True)
    loaded = json.loads(subprocess.check_output(["docker", "image", "inspect", SOURCE_TAG]))[0]
    if loaded["Architecture"] != "amd64" or loaded["Os"] != "linux":
        raise ValueError("unexpected loaded image platform")
    if loaded["RootFS"]["Layers"] != config["rootfs"]["diff_ids"]:
        raise ValueError("loaded image filesystem differs from the production backup")
    for field in ("Env", "WorkingDir", "Entrypoint"):
        if loaded["Config"].get(field) != config["config"].get(field):
            raise ValueError("loaded image configuration differs: " + field)
    # The classic store uses a config digest for Id; the containerd store uses
    # a manifest digest. A verified local tag works with either store.
    subprocess.run(["docker", "tag", SOURCE_TAG, TARGET_TAG], check=True)
    print("Verified REPL image: " + TARGET_TAG)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, tarfile.TarError, subprocess.SubprocessError) as error:
        print("REPL image load failed: " + str(error), file=sys.stderr)
        sys.exit(1)
