#!/usr/bin/env python3
"""Real POSIX default ACL inheritance across offline named-volume recovery."""
import argparse
import hashlib
import json
import pathlib
import re

from multiuser import initialize_volume
from recovery import RecoveryRuntime, container, volume_command, manifest, digest


def acl(runtime, image, volume, suffix, path):
    return volume_command(runtime, image, volume, suffix,
                          'getfacl --numeric --omit-header ' + path).stdout


def access(runtime, image, volume, suffix, path, uid, gid, allowed):
    action = 'cat ' + path + ' >/dev/null; : >> ' + path
    if not allowed:
        action = 'if cat ' + path + ' >/dev/null 2>&1; then exit 31; fi; if ( : >> ' + path + ' ) 2>/dev/null; then exit 32; fi; echo denied'
    result = volume_command(runtime, image, volume, suffix,
                            'test "$(id -u)" = ' + str(uid) + '; test "$(id -g)" = ' + str(gid) + '; ' + action,
                            user=str(uid) + ':' + str(gid))
    if not allowed:
        assert 'denied' in result.stdout


def create_children(runtime, helper, volume, suffix, parent, prefix, binary):
    # Shell redirection opens with mode0666, mkdir with0777. POSIX default ACL
    # takes precedence over umask while intersection with creation mode remains.
    volume_command(runtime, helper, volume, suffix,
                   'umask 077; printf synthetic > ' + parent + '/' + prefix + '-wide; '
                   'mkdir ' + parent + '/' + prefix + '-directory; '
                   'printf nested > ' + parent + '/' + prefix + '-directory/child', user='1000:1000')
    name = 'owlmail-default-acl-' + runtime.run_id + '-' + suffix
    runtime.text('create', '--name', name, '--label', 'io.owlmail.verification-run=' + runtime.run_id,
                 '--user', '1000:1000', '--volume', volume + ':/state', '--entrypoint', '/tmp/acl-creation',
                 helper, parent, prefix)
    runtime.containers.append(name)
    runtime.text('cp', str(pathlib.Path(binary).resolve()), name + ':/tmp/acl-creation')
    runtime.run('start', '--attach', name)
    assert runtime.text('inspect', name, '--format', '{{.State.ExitCode}}') == '0'


def verify_children(runtime, helper, image, volume, suffix, parent, prefix):
    wide, directory, child, narrow, private, private_child = [parent + '/' + prefix + ending for ending in
                                               ('-wide', '-directory', '-directory/child', '-narrow', '-private-directory', '-private-directory/child')]
    values = {name: acl(runtime, helper, volume, suffix + '-acl-' + str(i), path)
              for i, (name, path) in enumerate([('wide', wide), ('directory', directory), ('nestedChild', child),
                                               ('narrow', narrow), ('privateDirectory', private), ('privateGrandchild', private_child)])}
    assert 'user:1001:rwx' in values['wide'] and 'group:1003:rwx' in values['wide'] and 'mask::rw-' in values['wide']
    assert 'default:user:1001:rwx' in values['directory'] and 'default:group:1003:rwx' in values['directory']
    assert 'user:1001:rwx' in values['nestedChild'] and 'group:1003:rwx' in values['nestedChild']
    assert 'mask::---' in values['narrow'] and 'mask::---' in values['privateDirectory']
    assert 'default:user:1001:rwx' in values['privateDirectory'] and 'default:group:1003:rwx' in values['privateDirectory']
    assert 'user:1001:rwx' in values['privateGrandchild'] and 'group:1003:rwx' in values['privateGrandchild'] and 'mask::rw-' in values['privateGrandchild']
    access(runtime, image, volume, suffix + '-named-allow', wide, 1001, 1001, True)
    access(runtime, image, volume, suffix + '-group-allow', wide, 1002, 1003, True)
    access(runtime, image, volume, suffix + '-other-deny', wide, 1004, 1004, False)
    access(runtime, image, volume, suffix + '-narrow-user-deny', narrow, 1001, 1001, False)
    access(runtime, image, volume, suffix + '-narrow-group-deny', narrow, 1002, 1003, False)
    access(runtime, image, volume, suffix + '-nested-allow', child, 1001, 1001, True)
    access(runtime, image, volume, suffix + '-private-grandchild-deny', private_child, 1001, 1001, False)
    volume_command(runtime, image, volume, suffix + '-private-search-deny',
                   'test "$(id -u)" = 1001; if cd ' + private + ' 2>/dev/null; then exit 33; fi; echo search-denied', user='1001:1001')
    return values


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--runtime', choices=('docker', 'podman'), default='docker')
    parser.add_argument('--connection')
    parser.add_argument('--image', required=True)
    parser.add_argument('--helper-image', required=True)
    parser.add_argument('--evidence-directory', required=True)
    parser.add_argument('--creation-binary', required=True, help='Linux tagged acl_creation helper for true open0600/mkdir0700 syscalls')
    args = parser.parse_args()
    runtime = RecoveryRuntime(args.runtime, args.connection)
    evidence = pathlib.Path(args.evidence_directory); evidence.mkdir(parents=True, exist_ok=True)
    report = {'status': 'running', 'runId': runtime.run_id, 'scope': 'POSIX default ACL only; not NFSv4 ACL',
              'runtime': args.runtime, 'namespaceIdentities': {'owner': [1000, 1000], 'namedUser': [1001, 1001],
                                                           'namedGroupMember': [1002, 1003], 'unauthorized': [1004, 1004]}}
    try:
        runtime.run('info', timeout=20)
        mapping = container(runtime, args.helper_image, 'namespace-maps',
                            'echo __UID_MAP__; cat /proc/self/uid_map; echo __GID_MAP__; cat /proc/self/gid_map').stdout
        uid_text, gid_text = mapping.split('__UID_MAP__\n')[1].split('__GID_MAP__\n')
        maps = {kind: [list(map(int, row.split())) for row in text.splitlines()
                       if re.fullmatch(r'\s*\d+\s+\d+\s+\d+\s*', row)]
                for kind, text in [('uid',uid_text),('gid',gid_text)]}
        for ranges in maps.values():
            assert all(any(start <= value < start+length for start,_,length in ranges) for value in range(1000,1005)), 'namespace mapping must cover all test identities'

        source, backup, restored, ordinary = [runtime.volume(name) for name in
                                               ('default-source', 'default-backup', 'default-restored', 'default-ordinary')]
        initialize_volume(runtime, args.image, source, fixture=True)
        volume_command(runtime, args.helper_image, source, 'pre-default',
                       'umask 077; mkdir /state/inherit; mkdir /state/inherit/preexisting-directory; printf before > /state/inherit/preexisting', user='1000:1000')
        volume_command(runtime, args.helper_image, source, 'set-default',
                       'setfacl -m u:1001:--x,g:1003:--x,m::--x /state; '
                       'setfacl -m u:1001:rwx,g:1003:rwx,m::rwx /state/inherit; '
                       'setfacl -m d:u::rwx,d:u:1001:rwx,d:g::---,d:g:1003:rwx,d:m::rwx,d:o::--- /state/inherit')
        before_acl = acl(runtime, args.helper_image, source, 'before-acl', '/state/inherit/preexisting')
        assert 'user:1001:' not in before_acl and 'group:1003:' not in before_acl
        before_directory_acl = acl(runtime, args.helper_image, source, 'before-directory-acl', '/state/inherit/preexisting-directory')
        assert 'default:' not in before_directory_acl and 'user:1001:' not in before_directory_acl
        access(runtime, args.image, source, 'pre-user-deny', '/state/inherit/preexisting', 1001, 1001, False)
        create_children(runtime, args.helper_image, source, 'create-source', '/state/inherit', 'source', args.creation_binary)
        inherited = verify_children(runtime, args.helper_image, args.image, source, 'source', '/state/inherit', 'source')
        source_manifest = manifest(runtime, args.helper_image, source, 'source-manifest')
        source_default = acl(runtime, args.helper_image, source, 'source-default', '/state/inherit')
        container(runtime, args.helper_image, 'archive-aware',
                  'umask 077; tar --numeric-owner --acls --xattrs -cpf /backup/aware.tar -C /source .; sync',
                  [source + ':/source:ro', backup + ':/backup'])
        container(runtime, args.image, 'archive-ordinary',
                  'umask 077; /bin/busybox tar -cpf /backup/ordinary.tar -C /source .; sync',
                  [source + ':/source:ro', backup + ':/backup'])
        archive_before = container(runtime, args.helper_image, 'archive-hashes', 'sha256sum /backup/*.tar',
                                   [backup + ':/backup:ro']).stdout
        container(runtime, args.helper_image, 'restore-aware',
                  'tar --numeric-owner --acls --xattrs -xpf /backup/aware.tar -C /state; sync',
                  [backup + ':/backup:ro', restored + ':/state'])
        assert manifest(runtime, args.helper_image, restored, 'restored-manifest') == source_manifest
        assert acl(runtime, args.helper_image, restored, 'restored-default', '/state/inherit') == source_default
        restored_old = verify_children(runtime, args.helper_image, args.image, restored, 'restored-old', '/state/inherit', 'source')
        assert restored_old == inherited
        create_children(runtime, args.helper_image, restored, 'create-after-restore', '/state/inherit', 'after', args.creation_binary)
        newly_inherited = verify_children(runtime, args.helper_image, args.image, restored, 'after', '/state/inherit', 'after')
        assert newly_inherited == inherited, 'new children after restore did not inherit exact default ACL semantics'
        container(runtime, args.image, 'restore-ordinary', '/bin/busybox tar -xpf /backup/ordinary.tar -C /state; sync',
                  [backup + ':/backup:ro', ordinary + ':/state'])
        assert manifest(runtime, args.helper_image, ordinary, 'ordinary-manifest') == source_manifest
        ordinary_acl = acl(runtime, args.helper_image, ordinary, 'ordinary-default', '/state/inherit')
        assert 'default:' not in ordinary_acl
        # Grant only access/traversal ACL so failure below is specifically loss
        # of inherited default ACL, rather than inaccessible parent directories.
        volume_command(runtime, args.helper_image, ordinary, 'ordinary-traversal',
                       'setfacl -m u:1001:--x,g:1003:--x,m::--x /state; '
                       'setfacl -m u:1001:rwx,g:1003:rwx,m::rwx /state/inherit')
        volume_command(runtime, args.helper_image, ordinary, 'ordinary-create',
                       'umask 077; printf ordinary > /state/inherit/new-after-ordinary', user='1000:1000')
        no_inheritance = acl(runtime, args.helper_image, ordinary, 'ordinary-new-acl', '/state/inherit/new-after-ordinary')
        assert 'user:1001:' not in no_inheritance and 'group:1003:' not in no_inheritance
        access(runtime, args.image, ordinary, 'ordinary-user-deny', '/state/inherit/new-after-ordinary', 1001, 1001, False)
        access(runtime, args.image, ordinary, 'ordinary-group-deny', '/state/inherit/new-after-ordinary', 1002, 1003, False)
        assert manifest(runtime, args.helper_image, source, 'unchanged-source') == source_manifest
        assert acl(runtime, args.helper_image, source, 'unchanged-default', '/state/inherit') == source_default
        assert container(runtime, args.helper_image, 'unchanged-backup', 'sha256sum /backup/*.tar',
                         [backup + ':/backup:ro']).stdout == archive_before
        report.update(status='passed', sourceDefaultACL=source_default, preexistingACL=before_acl, preexistingDirectoryACL=before_directory_acl, namespaceMappings=maps,
                      inheritedBefore=inherited, inheritedAfterRestore=newly_inherited,
                      ordinaryDefaultACL=ordinary_acl, ordinaryNewChildACL=no_inheritance,
                      preexistingNotRetroactivelyChanged=True, umask077With0666Inherited=True,
                      creationMode0600Restricted=True, creationMode0700Restricted=True, privateDirectoryRetainsDefaultACL=True, privateGrandchildInheritedButSearchDenied=True,
                      newChildAndGrandchildAfterRestoreInherited=True, namedUserAndGroupIOVerified=True,
                      ordinaryTarLossDetected=True, ordinaryTarBytesAndStatEqual=True,
                      sourceUnchanged=True, archiveUnchanged=True, archiveHashes=archive_before,
                      sourceManifestSHA256=digest(source_manifest),
                      helperImageId=json.loads(runtime.text('image', 'inspect', args.helper_image))[0]['Id'],
                      scriptSHA256=hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),
                      creationSourceSHA256=hashlib.sha256(pathlib.Path(__file__).with_name('acl_creation').joinpath('main.go').read_bytes()).hexdigest(),
                      creationBinarySHA256=hashlib.sha256(pathlib.Path(args.creation_binary).read_bytes()).hexdigest())
        print('PASS POSIX default ACL: preexisting unchanged, new file/directory/grandchild inherit despite umask077, creation mode restricts effective mask', flush=True)
        print('PASS restored default ACL: new children still inherit; named-user/group actual IO allow/deny; ordinary tar loses default ACL and new-child access', flush=True)
    except Exception as error:
        report.update(status='failed', errorType=type(error).__name__)
        raise
    finally:
        runtime.close()
        (evidence / 'default-acl-recovery.json').write_text(json.dumps(report, indent=2) + '\n')
        print('DEFAULT_ACL_EVIDENCE ' + json.dumps({k: v for k, v in report.items()
              if k not in ('inheritedBefore', 'inheritedAfterRestore')}, sort_keys=True), flush=True)


if __name__ == '__main__':
    main()
