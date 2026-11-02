#!/usr/bin/env python3
"""Read-only NFSv4 ACL capability probe; never an authorization/recovery pass.

Only inspects an existing Podman machine and optionally an already mounted,
operator-selected directory. Does not mount, install tools, start NFS, or write
ACE data. Even a capable target exits 2 until a separate real authorization and
backup/restore acceptance drill has been executed and reviewed.
"""
import argparse
import hashlib
import json
import pathlib
import shlex
import subprocess


def probe(machine, script):
    result = subprocess.run(['podman', 'machine', 'ssh', machine,
                             '/bin/sh -c ' + shlex.quote(script)],
                            capture_output=True, text=True, timeout=30)
    return {'exitCode': result.returncode, 'stdout': result.stdout.strip(),
            'stderr': result.stderr.strip()}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--machine', default='podman-machine-default')
    parser.add_argument('--target', help='Absolute existing NFSv4 directory inside this machine; read-only inspection only')
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    if args.target and not pathlib.PurePosixPath(args.target).is_absolute():
        parser.error('--target must be an absolute path inside the selected machine')
    report = {'status': 'unsupported', 'acceptancePassed': False,
              'scope': 'existing Podman Linux VM; read-only capability probe, no ACE authorization or recovery drill',
              'machine': args.machine,
              'scriptSHA256': hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
    try:
        report['mounts'] = probe(args.machine, 'findmnt --json --list -o TARGET,FSTYPE,OPTIONS')
        report['filesystems'] = probe(args.machine, 'cat /proc/filesystems')
        report['nfsClientVolumes'] = probe(args.machine, 'if test -r /proc/fs/nfsfs/volumes; then cat /proc/fs/nfsfs/volumes; else echo unavailable; fi')
        report['tools'] = {name: probe(args.machine, 'command -v ' + name)
                           for name in ['nfs4_getfacl', 'nfs4_setfacl', 'nfsstat', 'findmnt']}
        report['nfsServerActivity'] = probe(args.machine, 'if test -r /proc/fs/nfsd/threads; then cat /proc/fs/nfsd/threads; else echo unavailable; fi')
        if report['mounts']['exitCode']:
            report['status'] = 'failed'
            report['reason'] = 'Cannot inspect machine mounts; unsupported must not hide probe failure.'
            return 1
        report['nfsMounts'] = [entry for entry in json.loads(report['mounts']['stdout']).get('filesystems', [])
                              if entry.get('fstype') in ('nfs', 'nfs4')]
        report['reason'] = 'No actual mounted NFSv4 ACL target selected; POSIX ACL on XFS/ext4 is not NFSv4 ACL.'
        if args.target:
            target = shlex.quote(args.target)
            report['targetMount'] = probe(args.machine, 'test -d ' + target + ' && findmnt --json -T ' + target + ' -o TARGET,FSTYPE,OPTIONS')
            if report['targetMount']['exitCode']:
                report['reason'] = 'Selected target is unavailable or cannot be inspected.'
                return 2
            entries = json.loads(report['targetMount']['stdout']).get('filesystems', [])
            mount = entries[0] if len(entries) == 1 else {}
            options = mount.get('options', '').split(',')
            nfs4 = mount.get('fstype') == 'nfs4' or (mount.get('fstype') == 'nfs' and any(v == 'vers=4' or v.startswith('vers=4.') for v in options))
            report['targetIsNFSv4'] = nfs4
            if not nfs4:
                report['reason'] = 'Selected target is not mounted NFSv4; local POSIX ACL/SMB are insufficient.'
            elif not all(report['tools'][name]['exitCode'] == 0 for name in ['nfs4_getfacl', 'nfs4_setfacl']):
                report['reason'] = 'NFSv4 mount exists but required NFSv4 ACL tools are unavailable.'
            else:
                report['targetACLRead'] = probe(args.machine, 'nfs4_getfacl ' + target)
                report['capabilityReady'] = report['targetACLRead']['exitCode'] == 0
                report['reason'] = ('Read-only NFSv4 ACL capability observed; ordered allow/deny ACE, actual mapped-principal I/O, inheritance, and backup/restore remain unverified.'
                                    if report['capabilityReady'] else 'NFSv4 mount/tools exist but actual ACL read failed; server ACL support is unverified.')
        return 2
    except (OSError, subprocess.TimeoutExpired, json.JSONDecodeError) as error:
        report.update(status='failed', reason='Capability probe failed', errorType=type(error).__name__)
        return 1
    finally:
        output = pathlib.Path(args.output)
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps({key: report[key] for key in ['status', 'acceptancePassed', 'reason'] if key in report}))


if __name__ == '__main__':
    raise SystemExit(main())
