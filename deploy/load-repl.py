#!/usr/bin/env python3
"""Verify and load the exact REPL image identified by this application release."""
import argparse
import subprocess
import sys
import tarfile

from repl_image import archive_info, read_manifest, sha256, verify_local_image


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('archive')
    parser.add_argument('--legacy', action='store_true', help='load the original recovery image')
    args = parser.parse_args()
    if args.legacy:
        expected = dict(
            archive_sha256='26aeaa5762b388529f1e9e7f1783958bd07dc571333a39da5535573a31e6850f',
            config_sha256='d10ec855f06b6cb33c9b86a0b5440c2b924c9fbb7933cf1b63efc819c11fedce',
            image='kflarsen/trymosml:latest')
    else:
        expected = read_manifest()
    if sha256(args.archive) != expected['archive_sha256']:
        raise ValueError('REPL archive checksum does not match the release manifest')
    actual = archive_info(args.archive)
    for field in ('image', 'config_sha256'):
        if actual[field] != expected[field]:
            raise ValueError('REPL archive differs from release manifest: ' + field)
    if not args.legacy:
        for field in ('layers', 'config'):
            if actual[field] != expected[field]:
                raise ValueError('REPL archive differs from release manifest: ' + field)
    # Refuse to repoint an existing release tag to different contents.
    present = subprocess.run(['docker', 'image', 'inspect', actual['image']],
                             stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=15)
    if present.returncode == 0:
        verify_local_image(actual['image'], manifest=actual)
    subprocess.run(['docker', 'load', '-i', args.archive], check=True)
    verify_local_image(actual['image'], manifest=actual)
    if args.legacy:
        subprocess.run(['docker', 'tag', actual['image'], 'trymosml/repl:legacy-20160924'], check=True)
    print('Verified REPL image: ' + actual['image'])


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, tarfile.TarError, subprocess.SubprocessError) as error:
        print('REPL image load failed: ' + str(error), file=sys.stderr)
        sys.exit(1)
