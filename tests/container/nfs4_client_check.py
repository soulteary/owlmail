#!/usr/bin/env python3
"""Read-only checks inside a real lab NFSv4 client mount namespace.

This stage does not create a server, mount anything, or assert full ACL
acceptance. Run it inside the isolated client after lab orchestration mounts the
export; unsupported is exit 2, actual probe failures are exit 1.
"""
import argparse
import hashlib
import json
import os
import pathlib
import shutil
import subprocess


def run(*args):
    p = subprocess.run(args, capture_output=True, text=True, timeout=15)
    return {'exitCode': p.returncode, 'stdout': p.stdout.strip(), 'stderr': p.stderr.strip()}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--mountpoint', required=True)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    report = {'status': 'unsupported', 'acceptancePassed': False,
              'scope': 'actual mounted NFSv4 client capability only; no ordered ACE, I/O or restore acceptance',
              'scriptSHA256': hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
    try:
        path = pathlib.Path(args.mountpoint)
        if not path.is_absolute() or not path.is_dir():
            report['reason'] = 'Absolute existing client mount directory required.'
            return 2
        report['mount'] = run('findmnt', '--json', '-T', str(path), '-o', 'TARGET,FSTYPE,OPTIONS,SOURCE')
        if report['mount']['exitCode']:
            report.update(status='failed', reason='Cannot inspect actual client mount.')
            return 1
        entries = json.loads(report['mount']['stdout']).get('filesystems', [])
        mount = entries[0] if len(entries) == 1 else {}
        options = mount.get('options', '').split(',')
        if not (mount.get('fstype') == 'nfs4' or (mount.get('fstype') == 'nfs' and any(v == 'vers=4' or v.startswith('vers=4.') for v in options))):
            report['reason'] = 'Actual mount is not NFSv4; POSIX or local file ACL is insufficient.'
            return 2
        report['uidMap'] = pathlib.Path('/proc/self/uid_map').read_text().strip()
        report['gidMap'] = pathlib.Path('/proc/self/gid_map').read_text().strip()
        report['tools'] = {name: shutil.which(name) for name in ['nfs4_getfacl', 'nfs4_setfacl']}
        if not all(report['tools'].values()):
            report['reason'] = 'Required real NFSv4 ACL tools are missing.'
            return 2
        report['acl'] = run('nfs4_getfacl', str(path))
        if report['acl']['exitCode']:
            report['reason'] = 'Actual NFSv4 ACL read failed; backend support is unverified.'
            return 2
        raw = os.getxattr(path, 'system.nfs4_acl')
        report['rawACLBytes'] = len(raw)
        report['rawACLSHA256'] = hashlib.sha256(raw).hexdigest()
        report['capabilityReady'] = True
        report['reason'] = 'Real NFSv4 mount and ACL GETATTR observed; ordered ACE authorization/inheritance and server-side restore remain unverified.'
        return 2
    except (OSError, subprocess.TimeoutExpired, json.JSONDecodeError) as error:
        report.update(status='failed', reason='Client probe failed.', errorType=type(error).__name__)
        return 1
    finally:
        pathlib.Path(args.output).write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps({k: report[k] for k in ['status', 'acceptancePassed', 'reason'] if k in report}))


if __name__ == '__main__':
    raise SystemExit(main())
