#!/usr/bin/env python3
"""Real POSIX ACL/user-xattr offline volume recovery; never assumes stat is enough."""
import argparse
import hashlib
import json
import pathlib
import re

from multiuser import fixture_request, initialize_volume
from recovery import RecoveryRuntime, container, volume_command, manifest, digest, start_fixture


class Unsupported(RuntimeError):
    pass


def metadata(runtime, helper, volume, paths, suffix):
    result = {}
    for index, path in enumerate(paths):
        acl = volume_command(runtime, helper, volume, suffix + '-acl-' + str(index),
                             'getfacl --numeric --omit-header ' + path).stdout
        attr = volume_command(runtime, helper, volume, suffix + '-attr-' + str(index),
                              "getfattr --absolute-names --dump --encoding=hex --match='^user[.]' " + path).stdout
        result[path.removeprefix('/state') or '/'] = {'acl': acl, 'userXattrs': attr}
    return result


def expect_access(runtime, image, volume, target, suffix, uid, readable):
    command = 'cat ' + target + ' >/dev/null'
    if not readable:
        command = 'if ' + command + ' 2>/dev/null; then exit 41; fi; echo read-denied'
    command = 'test "$(id -u)" = ' + str(uid) + '; test "$(id -g)" = ' + str(uid) + '; ' + command
    response = volume_command(runtime, image, volume, suffix, command, user=str(uid) + ':' + str(uid))
    assert response.returncode == 0
    if not readable:
        assert 'read-denied' in response.stdout


def archive_hash(runtime, helper, volume, suffix):
    return container(runtime, helper, suffix, 'sha256sum /backup/metadata.tar /backup/ordinary.tar',
                     [volume + ':/backup:ro']).stdout


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--runtime', choices=('docker', 'podman'), default='docker')
    parser.add_argument('--connection')
    parser.add_argument('--image', required=True, help='Unmodified production image (ordinary BusyBox tar negative)')
    parser.add_argument('--helper-image', required=True, help='Test-only image with acl, attr and GNU tar')
    parser.add_argument('--fixture-image', required=True, help='Build-tagged loopback fixture for synthetic mail')
    parser.add_argument('--evidence-directory', required=True)
    args = parser.parse_args()
    runtime = RecoveryRuntime(args.runtime, args.connection)
    evidence = pathlib.Path(args.evidence_directory); evidence.mkdir(parents=True, exist_ok=True)
    report = {'status': 'running', 'runId': runtime.run_id, 'runtime': args.runtime,
              'scope': 'POSIX ACL and user.* xattrs only; no SELinux/security.* or host ACL equivalence claimed',
              'containerNamespaceIDs': {'owner': 1000, 'namedUser': 1001, 'unauthorizedUser': 1002}}
    try:
        runtime.run('info', timeout=20)
        tools = container(runtime, args.helper_image, 'tools',
                          'for c in getfacl setfacl getfattr setfattr; do command -v "$c"; done; '
                          'tar --version; echo __UID_MAP__; cat /proc/self/uid_map; echo __GID_MAP__; cat /proc/self/gid_map', check=False)
        (evidence / 'metadata-tool-capabilities.log').write_text(tools.stdout + tools.stderr)
        if tools.returncode or 'GNU tar' not in tools.stdout:
            raise Unsupported('ACL/xattr/GNU tar tools unavailable in helper image')
        uid_text, gid_text = tools.stdout.split('__UID_MAP__\n')[1].split('__GID_MAP__\n')
        maps = {kind: [list(map(int, line.split())) for line in text.splitlines()
                       if re.fullmatch(r'\s*\d+\s+\d+\s+\d+\s*', line)]
                for kind, text in [('uid', uid_text), ('gid', gid_text)]}
        for kind, ranges in maps.items():
            if not all(any(start <= identity < start + length for start, _, length in ranges)
                       for identity in (1000, 1001, 1002)):
                raise Unsupported(kind + ' mapping does not cover all three test identities')

        source, backup, restored, ordinary = [runtime.volume(name) for name in
                                               ('metadata-source', 'metadata-backup', 'metadata-restored', 'metadata-ordinary')]
        initialize_volume(runtime, args.image, source, fixture=True)
        writer = start_fixture(runtime, args.fixture_image, source, 'metadata-writer')
        fixture_request(runtime, writer, '/__fixture__/deliver', body={
            'Recipient': 'alice@example.test', 'Subject': 'metadata recovery',
            'Body': 'synthetic ACL/xattr recovery', 'Attachment': 'metadata attachment'})
        runtime.text('stop', '--time', '10', writer)
        filename = volume_command(runtime, args.image, source, 'locate-eml',
                                  'find /state/mail -maxdepth 1 -name "*.eml" -type f').stdout.strip()
        assert filename.startswith('/state/mail/') and filename.endswith('.eml') and '\n' not in filename
        paths = ['/state', '/state/mail', filename]
        setup = volume_command(runtime, args.helper_image, source, 'set-metadata',
                               'setfacl -m u:1001:--x,m::--x /state /state/mail; '
                               'setfacl -m u:1001:rw-,m::r-- ' + filename + '; '
                               'setfattr -n user.owlmail.backup -v 0x001122aaff53796e746865746963 ' + filename + '; '
                               'setfattr -n user.owlmail.directory -v synthetic-directory /state/mail', check=False)
        if setup.returncode:
            if 'not supported' in (setup.stdout + setup.stderr).lower():
                raise Unsupported('named-volume filesystem does not support required POSIX ACL/user xattr operations')
            raise RuntimeError('metadata setup failed: ' + setup.stderr)
        before = manifest(runtime, args.helper_image, source, 'source-content')
        original_metadata = metadata(runtime, args.helper_image, source, paths, 'source')
        assert 'user:1001:rw-' in original_metadata[filename.removeprefix('/state')]['acl']
        assert 'mask::r--' in original_metadata[filename.removeprefix('/state')]['acl']
        assert 'user.owlmail.backup=0x001122aaff53796e746865746963' in original_metadata[filename.removeprefix('/state')]['userXattrs']
        expect_access(runtime, args.image, source, filename, 'source-named-read', 1001, True)
        expect_access(runtime, args.image, source, filename, 'source-other-deny', 1002, False)
        denied = volume_command(runtime, args.image, source, 'source-mask-write-deny',
                                'if ( : >> ' + filename + ' ) 2>/dev/null; then exit 42; fi; echo write-denied', user='1001:1001')
        assert 'write-denied' in denied.stdout
        container(runtime, args.helper_image, 'archive-aware',
                  'umask 077; tar --format=pax --numeric-owner --acls --xattrs --xattrs-include="user.*" '
                  '-cpf /backup/metadata.tar -C /source .; sync', [source + ':/source:ro', backup + ':/backup'])
        container(runtime, args.image, 'archive-ordinary',
                  'umask 077; /bin/busybox tar -cpf /backup/ordinary.tar -C /source .; sync',
                  [source + ':/source:ro', backup + ':/backup'])
        archives_before = archive_hash(runtime, args.helper_image, backup, 'archive-before')
        container(runtime, args.image, 'restore-ordinary',
                  '/bin/busybox tar -xpf /backup/ordinary.tar -C /state; sync',
                  [backup + ':/backup:ro', ordinary + ':/state'])
        ordinary_content = manifest(runtime, args.helper_image, ordinary, 'ordinary-content')
        ordinary_metadata = metadata(runtime, args.helper_image, ordinary, paths, 'ordinary')
        assert ordinary_content == before, 'ordinary tar must preserve bytes/mode/UID for this negative comparison'
        assert ordinary_metadata != original_metadata, 'ordinary tar unexpectedly preserved required metadata'
        assert 'user:1001:' not in ordinary_metadata[filename.removeprefix('/state')]['acl']
        assert 'user.owlmail.backup' not in ordinary_metadata[filename.removeprefix('/state')]['userXattrs']
        # Restore directory traversal only, isolating the file ACL omission as
        # the reason UID1001 cannot read, despite its unchanged displayed mode.
        volume_command(runtime, args.helper_image, ordinary, 'ordinary-traversal',
                       'setfacl -m u:1001:--x,m::--x /state /state/mail')
        expect_access(runtime, args.image, ordinary, filename, 'ordinary-file-acl-deny', 1001, False)
        print('PASS ordinary-tar negative: bytes/stat identical but named ACL/user xattr lost, actual named-user read rejected', flush=True)
        container(runtime, args.helper_image, 'restore-aware',
                  'tar --numeric-owner --acls --xattrs --xattrs-include="user.*" -xpf /backup/metadata.tar -C /state; sync',
                  [backup + ':/backup:ro', restored + ':/state'])
        assert manifest(runtime, args.helper_image, restored, 'restored-content') == before
        restored_metadata = metadata(runtime, args.helper_image, restored, paths, 'restored')
        assert restored_metadata == original_metadata
        expect_access(runtime, args.image, restored, filename, 'restored-named-read', 1001, True)
        expect_access(runtime, args.image, restored, filename, 'restored-other-deny', 1002, False)
        volume_command(runtime, args.helper_image, restored, 'mask-disable', 'setfacl -m m::--- ' + filename)
        expect_access(runtime, args.image, restored, filename, 'restored-mask-deny', 1001, False)
        volume_command(runtime, args.helper_image, restored, 'mask-repair', 'setfacl -m m::r-- ' + filename)
        expect_access(runtime, args.image, restored, filename, 'restored-mask-repaired', 1001, True)
        volume_command(runtime, args.helper_image, restored, 'mask-allow-write', 'setfacl -m m::rw- ' + filename)
        volume_command(runtime, args.image, restored, 'restored-write-allowed',
                       'test "$(id -u)" = 1001; test "$(id -g)" = 1001; : >> ' + filename, user='1001:1001')
        volume_command(runtime, args.helper_image, restored, 'mask-remask-write', 'setfacl -m m::r-- ' + filename)
        denied = volume_command(runtime, args.image, restored, 'restored-write-deny',
                                'if ( : >> ' + filename + ' ) 2>/dev/null; then exit 43; fi; echo write-denied', user='1001:1001')
        assert 'write-denied' in denied.stdout
        assert metadata(runtime, args.helper_image, restored, paths, 'final-restored') == original_metadata
        assert manifest(runtime, args.helper_image, source, 'final-source') == before
        assert metadata(runtime, args.helper_image, source, paths, 'final-source') == original_metadata
        assert archive_hash(runtime, args.helper_image, backup, 'archive-after') == archives_before
        report.update(status='passed', toolsAvailable=True, filesystemSupportsACLAndUserXattr=True,
                      metadataBefore=original_metadata, metadataAfter=restored_metadata, namespaceMappings=maps, fileManifest=before,
                      ordinaryTarRejected=True, ordinaryTarStatAndBytesEqual=True,
                      namedUserReadAllowed=True, namedUserWriteMasked=True, namedUserWriteAllowedWithMaskRW=True, unauthorizedUserReadDenied=True,
                      maskRemovalReadDenied=True, maskRepairReadAllowed=True,
                      sourceUnchanged=True, archivesUnchanged=True, archivesSHA256=archives_before,
                      contentManifestSHA256=digest(before), metadataSHA256=digest(original_metadata),
                      helperImageId=json.loads(runtime.text('image', 'inspect', args.helper_image))[0]['Id'],
                      scriptSHA256=hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest())
        print('PASS metadata-aware restore: exact numeric ACL/user.* values, UID1001 allowed read/masked write, UID1002 denied, mask negative and repair', flush=True)
    except Unsupported as error:
        report.update(status='unsupported', reason=str(error))
        raise SystemExit(2) from error
    except Exception as error:
        report.update(status='failed', errorType=type(error).__name__)
        raise
    finally:
        runtime.close()
        (evidence / 'metadata-volume-recovery.json').write_text(json.dumps(report, indent=2) + '\n')
        print('METADATA_RECOVERY_EVIDENCE ' + json.dumps({k: v for k, v in report.items()
              if k not in ('metadataBefore', 'metadataAfter', 'fileManifest')}, sort_keys=True), flush=True)


if __name__ == '__main__':
    main()
