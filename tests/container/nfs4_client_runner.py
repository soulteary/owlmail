#!/usr/bin/env python3
"""Rootful private-namespace client for this run's loopback-only native ACL lab."""
import argparse
import hashlib
import json
import pathlib
import subprocess
import uuid


def inspect_summary(item):
    """Keep only runtime provenance; never persist inherited environment values."""
    config = item.get('Config') or {}
    host = item.get('HostConfig') or {}
    return {'Id': item.get('Id'), 'Image': item.get('Image'),
            'Config': {'User': config.get('User'), 'Labels': {
                'io.owlmail.verification-run': (config.get('Labels') or {}).get('io.owlmail.verification-run')}},
            'HostConfig': {key: host.get(key) for key in [
                'NetworkMode', 'CapAdd', 'CapDrop', 'SecurityOpt', 'Privileged', 'PidMode', 'UsernsMode']},
            'State': {'Pid': (item.get('State') or {}).get('Pid')}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--connection', default='podman-machine-default-root')
    parser.add_argument('--machine', default='podman-machine-default', help='Existing Podman VM corresponding to the client connection; read-only host namespace witness')
    parser.add_argument('--image', default='localhost/owlmail:nfs4-lab-client')
    parser.add_argument('--port', type=int, required=True)
    parser.add_argument('--stage', choices=['prepare', 'confirm', 'verify'], required=True)
    parser.add_argument('--manifest', required=True)
    parser.add_argument('--case', required=True)
    parser.add_argument('--principal', default='1001', help='Numeric AUTH_SYS principal in the isolated lab; no domain/Kerberos acceptance implied')
    parser.add_argument('--evidence-directory', required=True)
    parser.add_argument('--disable-container-label', action='store_true', help='Only if lab NFS access is blocked by the client container label; does not change host SELinux policy')
    args = parser.parse_args()
    if not 1024 <= args.port <= 65535:
        parser.error('isolated loopback server port must be unprivileged')
    evidence = pathlib.Path(args.evidence_directory)
    evidence.mkdir(parents=True, exist_ok=True)
    manifest = pathlib.Path(args.manifest).resolve()
    if args.stage in ['confirm', 'verify'] and not manifest.is_file():
        parser.error('verify requires the existing prepare manifest')
    if args.stage == 'prepare' and manifest.exists():
        parser.error('prepare refuses to overwrite an existing manifest')
    run_id = uuid.uuid4().hex[:12]
    name = 'owlmail-nfs4-client-' + run_id
    prefix = ['podman', '--connection', args.connection]
    report = {'status': 'failed', 'runId': run_id, 'stage': args.stage, 'serverPort': args.port,
              'scope': 'AUTH_SYS, loopback-only isolated native ACL lab; no production or Kerberos claim',
              'containerLabelDisabled': args.disable_container_label,
              'scriptSHA256': hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
    created = False
    mounted = False
    container_id = None

    def run(*parts, check=True, timeout=35):
        p = subprocess.run(prefix + list(parts), capture_output=True, text=True, timeout=timeout)
        logged_stdout = p.stdout
        if parts[0] == 'inspect' and p.returncode == 0:
            logged_stdout = json.dumps([inspect_summary(item) for item in json.loads(p.stdout)])
        with (evidence / (args.stage + '-commands.log')).open('a') as out:
            out.write(json.dumps({'command': list(parts), 'exit': p.returncode, 'stdout': logged_stdout, 'stderr': p.stderr}) + '\n')
        if check and p.returncode:
            raise RuntimeError('client command failed: ' + parts[0] + ': ' + p.stderr.strip())
        return p

    try:
        options = ['run', '--detach', '--pull=never', '--name', name,
                   '--label', 'io.owlmail.verification-run=' + run_id,
                   '--network', 'host', '--cap-add', 'SYS_ADMIN']
        if args.disable_container_label:
            options += ['--security-opt', 'label=disable']
        options += ['--entrypoint', '/bin/sh', args.image, '-c', 'exec sleep 600']
        container_id = run(*options).stdout.strip()
        if not container_id:
            raise RuntimeError("container creation returned no ID")
        created = True
        report['inspect'] = inspect_summary(json.loads(run('inspect', name).stdout)[0])
        report['runtime'] = run('exec', name, '/bin/sh', '-c',
            'id; readlink /proc/self/ns/mnt; cat /proc/self/uid_map; cat /proc/self/gid_map; grep Cap /proc/self/status; cat /proc/self/attr/current; findmnt -n -o PROPAGATION /').stdout
        report['containerMountNamespace'] = run('exec', name, 'readlink', '/proc/self/ns/mnt').stdout.strip()
        host_ns = subprocess.run(['podman', 'machine', 'ssh', args.machine, 'readlink /proc/1/ns/mnt'], capture_output=True, text=True, timeout=20, check=True).stdout.strip()
        report['vmHostMountNamespace'] = host_ns
        if not host_ns.startswith('mnt:[') or report['containerMountNamespace'] == host_ns:
            raise AssertionError('client mount namespace is not independently isolated from VM host')
        root_propagation = run('exec', name, 'findmnt', '-n', '-o', 'PROPAGATION', '/').stdout.strip()
        if root_propagation != 'private':
            raise AssertionError('client root mount must use private propagation')
        # Only private container paths and a loopback endpoint are used.
        run('exec', name, 'mkdir', '-p', '/mnt/native', '/evidence')
        for filename in ['nfs4_client_check.py', 'nfs4_client_acceptance.py']:
            source = pathlib.Path(__file__).with_name(filename)
            report[filename + 'SHA256'] = hashlib.sha256(source.read_bytes()).hexdigest()
            run('cp', str(source.resolve()), name + ':/opt/owlmail-nfs4-client/' + filename)
        mount_options = 'vers=4.1,proto=tcp,sec=sys,soft,timeo=20,retrans=1,retry=0,port=' + str(args.port)
        report['mountOptions'] = mount_options
        run('exec', name, 'timeout', '20', 'mount', '-t', 'nfs4', '-o', mount_options, '127.0.0.1:/lab', '/mnt/native', timeout=25)
        mounted = True
        report['actualMount'] = run('exec', name, 'findmnt', '--json', '-T', '/mnt/native', '-o', 'SOURCE,TARGET,FSTYPE,OPTIONS,PROPAGATION').stdout
        mounted_propagation = run('exec', name, 'findmnt', '-n', '-o', 'PROPAGATION', '-T', '/mnt/native').stdout.strip()
        if mounted_propagation != 'private':
            raise AssertionError('NFS mount propagation must remain private')
        if args.stage in ['confirm', 'verify']:
            run('cp', str(manifest), name + ':/evidence/manifest.json')
        run('exec', name, 'python3', '/opt/owlmail-nfs4-client/nfs4_client_acceptance.py', args.stage,
            '--mountpoint', '/mnt/native', '--case', args.case, '--principal', args.principal,
            '--manifest', '/evidence/manifest.json', '--output', '/evidence/result.json', timeout=140)
        result = json.loads(run('exec', name, 'cat', '/evidence/result.json').stdout)
        if result['status'] != 'passed':
            raise AssertionError('client acceptance did not pass')
        report['clientResult'] = result
        if args.stage == 'prepare':
            manifest.parent.mkdir(parents=True, exist_ok=True)
            run('cp', name + ':/evidence/manifest.json', str(manifest))
        report['manifestSHA256'] = hashlib.sha256(manifest.read_bytes()).hexdigest()
        report['status'] = 'passed'
    except Exception as error:
        report['errorType'] = type(error).__name__
        report['error'] = str(error)
        raise
    finally:
        if not created:
            # A timed-out create may have succeeded remotely before returning
            # its ID. Discover only our exact random label, never a name alone.
            try:
                pending = run('inspect', name, check=False)
                if pending.returncode == 0:
                    item = json.loads(pending.stdout)[0]
                    if item['Config']['Labels'].get('io.owlmail.verification-run') == run_id:
                        container_id = item['Id']
                        created = True
            except Exception as cleanup_error:
                report.update(status='failed', createCleanupDiscoveryError=type(cleanup_error).__name__)
        if created:
            try:
                if mounted:
                    unmount = run('exec', container_id, 'timeout', '15', 'umount', '/mnt/native', check=False, timeout=20)
                    report['unmountExitCode'] = unmount.returncode
                    if unmount.returncode:
                        report['status'] = 'failed'
            except Exception as cleanup_error:
                report.update(status='failed', unmountError=type(cleanup_error).__name__)
            finally:
                try:
                    # The full ID is returned by our successful create; unlike a
                    # reusable name it cannot refer to a preexisting container.
                    run('rm', '--force', container_id)
                except Exception as cleanup_error:
                    report.update(status='failed', removalError=type(cleanup_error).__name__)
            try:
                remaining = json.loads(run('ps', '--all', '--filter', 'label=io.owlmail.verification-run=' + run_id, '--format', 'json').stdout)
                report['remainingOwnedContainers'] = len(remaining)
                if remaining:
                    report['status'] = 'failed'
            except Exception as cleanup_error:
                report.update(status='failed', cleanupVerificationError=type(cleanup_error).__name__)
        (evidence / (args.stage + '-runner.json')).write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps({k: report[k] for k in ['status', 'runId', 'stage', 'remainingOwnedContainers'] if k in report}))
        if report['status'] != 'passed':
            raise RuntimeError('NFSv4 client stage or cleanup failed')


if __name__ == '__main__':
    main()
