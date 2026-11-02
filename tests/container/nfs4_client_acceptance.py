#!/usr/bin/env python3
"""Native NFSv4 ACL client stages, inside an isolated mounted lab namespace.

prepare creates only a new case directory. verify must run against an independent
server-side restored volume using the prepare manifest stored OUTSIDE the export.
The orchestrator must prove that independent server/volume restore separately.
"""
import argparse
import errno
import hashlib
import json
import os
import pathlib
import re
import subprocess


EXPECTED_CONTENTS = {
    'deny-first': b'native NFSv4 ordered ACE fixture\n',
    'allow-first': b'native NFSv4 ordered ACE fixture\n',
    'identity-control/principal-owned': b'numeric AUTH_SYS identity control\n',
    'inherit/child': b'inherited ACL fixture\n',
    'inherit/nested/grandchild': b'inherited grandchild\n',
}
PLAIN_MODE_PATHS = {'.', 'identity-control', 'identity-control/principal-owned'}
EXPECTED_OBJECT_PATHS = {'.', 'identity-control', 'identity-control/principal-owned',
                         'deny-first', 'allow-first', 'inherit', 'inherit/child',
                         'inherit/nested', 'inherit/nested/grandchild'}


def assert_fixture_contents(root):
    for relative, expected in EXPECTED_CONTENTS.items():
        actual = (root / relative).read_bytes()
        if actual != expected:
            raise AssertionError('fixed fixture bytes mismatch (NUL/empty/truncated data must fail): ' + relative)


PAGINATION_FILES = {
    ('item-%03d-' % index) + ('p' * 180): ('pagination item %03d fixed bytes\n' % index).encode()
    for index in range(137)
}


def pagination_fixture(mount, case, phase):
    directory = mount / (case + '-pagination')
    if phase == 'prepare':
        directory.mkdir(mode=0o755)
        for name, content in PAGINATION_FILES.items():
            with (directory / name).open('xb') as out:
                out.write(content)
                out.flush()
                os.fsync(out.fileno())
    entries = list(os.scandir(directory))
    names = [entry.name for entry in entries]
    if len(names) != len(PAGINATION_FILES) or len(set(names)) != len(names) or set(names) != set(PAGINATION_FILES):
        raise AssertionError('actual paginated READDIR must enumerate all 137 fixed files exactly once')
    observed = {}
    for entry in entries:
        if entry.is_symlink() or not entry.is_file(follow_symlinks=False):
            raise AssertionError('unexpected paginated directory object')
        content = (directory / entry.name).read_bytes()
        if content != PAGINATION_FILES[entry.name]:
            raise AssertionError('paginated fixed file bytes mismatch: ' + entry.name)
        observed[entry.name] = hashlib.sha256(content).hexdigest()
    return {'directory': directory.name, 'files': len(names), 'noDuplicateOrMissingNames': True,
            'fixedContentsVerified': True,
            'namesSHA256': hashlib.sha256(json.dumps(sorted(names)).encode()).hexdigest(),
            'contentsSHA256': hashlib.sha256(json.dumps(observed, sort_keys=True).encode()).hexdigest()}


IDENTITY_CODE = ('import os,sys; u=int(sys.argv[2]); '
                 'assert os.getuid()==os.geteuid()==os.getgid()==os.getegid()==u; '
                 'assert os.getgroups()==[]; status=open("/proc/self/status").read(); '
                 'assert int(next(l.split()[1] for l in status.splitlines() if l.startswith("CapEff:")),16)==0; ')


def command(*args, check=True):
    p = subprocess.run(args, capture_output=True, text=True, timeout=20)
    if check and p.returncode:
        raise RuntimeError('client command failed: ' + args[0] + ': ' + p.stderr.strip())
    return p


def acl(path):
    return [line.strip() for line in command('nfs4_getfacl', str(path)).stdout.splitlines()
            if line.strip() and not line.startswith('#')]


def read_as(path, uid, expected):
    p = command('setpriv', '--reuid=' + str(uid), '--regid=' + str(uid), '--clear-groups',
                'python3', '-c', IDENTITY_CODE + 'f=open(sys.argv[1],"rb"); f.read(); f.close()',
                str(path), str(uid), check=False)
    if (p.returncode == 0) != expected:
        raise AssertionError('real principal I/O result differs from ordered ACE contract: ' + path.name + ': ' + p.stderr.strip())
    if not expected and 'PermissionError' not in p.stderr:
        raise AssertionError('denial was not a real file access permission error: ' + p.stderr)


def write_as(path, uid, expected):
    code = IDENTITY_CODE + 'f=open(sys.argv[1],"r+b"); old=f.read(); f.seek(0,2); f.write(b"X"); f.flush(); os.fsync(f.fileno()); f.seek(0); assert f.read()==old+b"X"; f.truncate(len(old)); f.flush(); os.fsync(f.fileno()); f.seek(0); assert f.read()==old; f.close()'
    p = command('setpriv', '--reuid=' + str(uid), '--regid=' + str(uid), '--clear-groups', 'python3', '-c', code, str(path), str(uid), check=False)
    if (p.returncode == 0) != expected or (not expected and 'PermissionError' not in p.stderr):
        raise AssertionError('real principal write authorization mismatch: ' + path.name)


def direct_read_as(path, uid, expected):
    # O_DIRECT is intentional: unsupported direct I/O is a failure, never a
    # fallback to a page-cache result. mmap supplies an aligned writable buffer.
    code = IDENTITY_CODE + """
import mmap,json
expected=bytes.fromhex(sys.argv[3])
fd=os.open(sys.argv[1],os.O_RDONLY|os.O_DIRECT)
records=[]
try:
    buffer=mmap.mmap(-1,4096)
    try:
        for offset in [0,3,17,len(expected)]:
            count=os.preadv(fd,[buffer],offset)
            wanted=expected[offset:offset+4096]
            assert count==len(wanted) and bytes(buffer[:count])==wanted, 'direct NFS fixed bytes mismatch'
            records.append({'offset':offset,'bytes':count})
    finally:
        buffer.close()
finally:
    os.close(fd)
print(json.dumps(records))
"""
    return json.loads(command('setpriv', '--reuid=' + str(uid), '--regid=' + str(uid), '--clear-groups',
                              'python3', '-c', code, str(path), str(uid), expected.hex()).stdout)


def snapshot(root):
    result = {}
    for path in [root, *sorted(root.rglob('*'))]:
        if path.is_symlink() or not (path.is_file() or path.is_dir()):
            raise AssertionError('unexpected lab object')
        relative = str(path.relative_to(root))
        if relative in result:
            raise AssertionError('actual READDIR returned a duplicate fixture path: ' + relative)
        try:
            raw = os.getxattr(path, 'system.nfs4_acl')
            actual_acl = acl(path)
        except OSError as error:
            if relative not in PLAIN_MODE_PATHS or error.errno not in (errno.ENODATA, errno.EOPNOTSUPP):
                raise
            raw = None
            actual_acl = None
        item = {'acl': actual_acl, 'rawACLSHA256': hashlib.sha256(raw).hexdigest() if raw is not None else None,
                'uid': path.stat().st_uid, 'gid': path.stat().st_gid, 'mode': oct(path.stat().st_mode & 0o7777), 'directory': path.is_dir()}
        if path.is_file():
            item['sha256'] = hashlib.sha256(path.read_bytes()).hexdigest()
        result[str(path.relative_to(root))] = item
    if set(result) != EXPECTED_OBJECT_PATHS:
        raise AssertionError('actual READDIR enumeration is incomplete or duplicated: missing=' +
                             repr(sorted(EXPECTED_OBJECT_PATHS - set(result))) +
                             ' unexpected=' + repr(sorted(set(result) - EXPECTED_OBJECT_PATHS)))
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('phase', choices=['prepare', 'confirm', 'verify'])
    parser.add_argument('--mountpoint', required=True)
    parser.add_argument('--case', required=True, help='Unique new single directory basename')
    parser.add_argument('--principal', required=True, help='Real server-mapped NFSv4 who string for uid (not synthetic ACL text)')
    parser.add_argument('--uid', type=int, default=1001)
    parser.add_argument('--other-uid', type=int, default=1002)
    parser.add_argument('--manifest', required=True, help='Prepare snapshot outside the NFS export')
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    report = {'status': 'failed', 'phase': args.phase, 'scope': 'AUTH_SYS isolated lab, real mounted client ACE/I/O; external native server restore provenance required; no Kerberos or OwlMail SQLite deployment claim',
              'scriptSHA256': hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
    try:
        mount = pathlib.Path(args.mountpoint).resolve(strict=True)
        if not re.fullmatch(r'[a-zA-Z0-9_-]+', args.case) or not re.fullmatch(r'[a-zA-Z0-9_.@-]+', args.principal):
            raise ValueError('unsafe case or principal')
        if not (0 < args.uid != args.other_uid > 0) or os.geteuid() != 0:
            raise ValueError('root client fixture and two distinct nonroot mapped UIDs required')
        where = json.loads(command('findmnt', '--json', '-T', str(mount), '-o', 'FSTYPE,OPTIONS,SOURCE,TARGET').stdout)['filesystems'][0]
        if where['fstype'] != 'nfs4' and not (where['fstype'] == 'nfs' and re.search(r'(?:^|,)vers=4(?:[.,]|$)', where['options'])):
            raise AssertionError('client path is not a real NFSv4 mount')
        report['actualMount'] = where
        report['uidMap'] = pathlib.Path('/proc/self/uid_map').read_text().strip()
        report['gidMap'] = pathlib.Path('/proc/self/gid_map').read_text().strip()
        for text in [report['uidMap'], report['gidMap']]:
            ranges = [list(map(int, line.split())) for line in text.splitlines()]
            if not all(any(start <= uid < start + count for start, _, count in ranges) for uid in [args.uid, args.other_uid]):
                raise AssertionError('test UID/GID is not mapped')
        root = mount / args.case
        manifest = pathlib.Path(args.manifest).resolve()
        if manifest == mount or mount in manifest.parents:
            raise ValueError('manifest must stay outside exported mount')
        if args.phase == 'prepare':
            root.mkdir(mode=0o755)  # No existing operator content can be overwritten.
            root.chmod(0o755)  # Permit search independent of the client fixture's umask.
            identity_dir = root / 'identity-control'
            identity_dir.mkdir(mode=0o777)
            identity_dir.chmod(0o777)
            command('setpriv', '--reuid=' + str(args.uid), '--regid=' + str(args.uid), '--clear-groups',
                    'python3', '-c', IDENTITY_CODE + 'f=open(sys.argv[1],"xb"); f.write(bytes.fromhex(sys.argv[3])); f.flush(); os.fsync(f.fileno()); f.close()',
                    str(identity_dir / 'principal-owned'), str(args.uid), EXPECTED_CONTENTS['identity-control/principal-owned'].hex())
            for name in ['deny-first', 'allow-first']:
                (root / name).write_bytes(EXPECTED_CONTENTS[name])
            owner = 'A::OWNER@:rwaDxtTnNcCy'
            attrs = 'A::EVERYONE@:tcy'
            command('nfs4_setfacl', '-s', ','.join([owner, 'D::' + args.principal + ':rwa', 'A::' + args.principal + ':rwa', attrs]), str(root / 'deny-first'))
            command('nfs4_setfacl', '-s', ','.join([owner, 'A::' + args.principal + ':rwa', 'D::' + args.principal + ':rwa', attrs]), str(root / 'allow-first'))
            parent = root / 'inherit'
            parent.mkdir(mode=0o755)
            command('nfs4_setfacl', '-s', ','.join(['A:fd:OWNER@:rwaDxtTnNcCy', 'A::' + args.principal + ':xtncy', 'A:fd:' + args.principal + ':rxtncy', attrs]), str(parent))
            (parent / 'child').write_bytes(EXPECTED_CONTENTS['inherit/child'])
            (parent / 'nested').mkdir()
            (parent / 'nested' / 'grandchild').write_bytes(EXPECTED_CONTENTS['inherit/nested/grandchild'])
        if mount.stat().st_uid != 0 or mount.stat().st_gid != 0 or root.stat().st_uid != 0 or root.stat().st_gid != 0:
            raise AssertionError('numeric AUTH_SYS root identity mapped incorrectly (anonymous owner is unacceptable)')
        identity = root / 'identity-control' / 'principal-owned'
        if identity.stat().st_uid != args.uid or identity.stat().st_gid != args.uid:
            raise AssertionError('numeric AUTH_SYS principal inode owner mapping is incorrect')
        report['numericIdentityObserved'] = {'exportOwnerUID': mount.stat().st_uid, 'caseOwnerUID': root.stat().st_uid, 'createdPrincipalUID': identity.stat().st_uid, 'createdPrincipalGID': identity.stat().st_gid}
        report['orderedACLAtAdmission'] = {name: acl(root / name) for name in ['deny-first', 'allow-first']}
        assert_fixture_contents(root)
        report['fixedExpectedContentVerified'] = True
        for owned in [root / 'deny-first', root / 'allow-first']:
            if owned.stat().st_uid in [args.uid, args.other_uid]:
                raise AssertionError('test principal unexpectedly owns the object; named ACE is not isolated')
        read_as(root / 'deny-first', args.uid, False)
        read_as(root / 'allow-first', args.uid, True)
        read_as(root / 'allow-first', args.other_uid, False)
        if args.phase != 'confirm':
            write_as(root / 'deny-first', args.uid, False)
            write_as(root / 'allow-first', args.uid, True)
            write_as(root / 'allow-first', args.other_uid, False)
        report['directReadFixedOffsetsVerified'] = direct_read_as(root / 'allow-first', args.uid, EXPECTED_CONTENTS['allow-first'])
        read_as(root / 'inherit' / 'child', args.uid, True)
        read_as(root / 'inherit' / 'nested' / 'grandchild', args.uid, True)
        for inherited in [root / 'inherit' / 'child', root / 'inherit' / 'nested', root / 'inherit' / 'nested' / 'grandchild']:
            if not any(args.principal in line for line in acl(inherited)):
                raise AssertionError('principal ACE not inherited by actual created object')
        data = snapshot(root)
        pagination = pagination_fixture(mount, args.case, args.phase)
        report['paginationObserved'] = pagination
        if args.phase == 'prepare':
            with manifest.open('x') as out:
                json.dump({'case': args.case, 'principal': args.principal, 'uid': args.uid, 'otherUid': args.other_uid, 'objects': data, 'pagination': pagination}, out, indent=2)
        else:
            expected = json.loads(manifest.read_text())
            if expected['case'] != args.case or expected['principal'] != args.principal or expected['uid'] != args.uid or expected['otherUid'] != args.other_uid or expected['objects'] != data or expected['pagination'] != pagination:
                raise AssertionError('server restore changed ordered ACL, raw ACL, ownership or data')
        if args.phase == 'verify':
            fresh = root / 'inherit' / 'created-after-restore'
            with fresh.open('xb') as out:
                out.write(b'new inherited object after server restore\n')
                out.flush()
                os.fsync(out.fileno())
            read_as(fresh, args.uid, True)
            read_as(fresh, args.other_uid, False)
            expected_acl = expected['objects']['inherit/child']['acl']
            actual_acl = acl(fresh)
            actual_raw_hash = hashlib.sha256(os.getxattr(fresh, 'system.nfs4_acl')).hexdigest()
            if actual_acl != expected_acl or actual_raw_hash != expected['objects']['inherit/child']['rawACLSHA256']:
                raise AssertionError('new restored child did not inherit original ordered ACEs exactly')
            if fresh.read_bytes() != b'new inherited object after server restore\n':
                raise AssertionError('new restored child bytes corrupted')
            report['newObjectAfterRestoreInherited'] = True
            report['newObjectAfterRestoreOriginalACEVerified'] = actual_acl
            report['newObjectAfterRestoreRawACLSHA256'] = actual_raw_hash
        report.update(status='passed', orderedDenyBeforeAllowDenied=True, orderedAllowBeforeDenyAllowed=True,
                      otherPrincipalDenied=True, realWriteAndFsyncVerified=args.phase != 'confirm', confirmReadOnly=args.phase == 'confirm', nonrootIOEffectiveCapabilitiesZero=True, inheritedChildAllowed=True, exactSnapshotVerified=args.phase in ['confirm', 'verify'],
                      snapshotSHA256=hashlib.sha256(json.dumps(data, sort_keys=True).encode()).hexdigest())
    finally:
        pathlib.Path(args.output).write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps(report))


if __name__ == '__main__':
    main()
