#!/usr/bin/env python3
"""Isolated native RichACL/NFSv4 lab with offline whole-backend recovery.

The server is user-space LizardFS + Ganesha FSAL_LIZARDFS. Only a dynamically
allocated VM-loopback NFS port is exposed; clients mount in private namespaces.
No production OwlMail binary, exports, fstab, host idmap or SELinux policy changes.
"""
import argparse
import hashlib
import json
import pathlib
import shutil
import shlex
import subprocess
import sys
import tempfile
import time

from recovery import RecoveryRuntime, container, volume_command, manifest, digest


def capture_service_logs(runtime, image, volume, suffix, evidence):
    diagnostic = 'import json,pathlib; print(json.dumps({p.name:p.read_text(errors="replace")[-262144:] for p in pathlib.Path("/state/logs").glob("*") if p.is_file()}))'
    details = container(runtime, image, suffix + '-failure-logs', 'python3 -c ' + shlex.quote(diagnostic), [volume + ':/state:ro'])
    (evidence / (suffix + '-service-failure-logs.json')).write_text(details.stdout)


def start_server(runtime, image, volume, suffix, evidence):
    name = 'owlmail-nfs4-server-' + runtime.run_id + '-' + suffix
    instance = runtime.text('create', '--name', name, '--label', 'io.owlmail.verification-run=' + runtime.run_id,
                            '--hostname', 'owlmail-nfs4-' + runtime.run_id + '-' + suffix, '--volume', volume + ':/state',
                            '--publish', '127.0.0.1::2049', '--entrypoint', 'python3', image, '/tmp/nfs4_lab_server.py')
    runtime.containers.append(instance)
    with tempfile.TemporaryDirectory(prefix='owlmail-nfs4-bootstrap-') as directory:
        script = pathlib.Path(directory) / 'nfs4_lab_server.py'
        shutil.copy2(pathlib.Path(__file__).with_name('nfs4_lab_server.py'), script)
        runtime.text('cp', str(script), instance + ':/tmp/nfs4_lab_server.py')
    runtime.text('start', instance)
    for _ in range(120):
        probe = runtime.run('exec', instance, 'cat', '/state/lab-ready.json', check=False, timeout=5)
        if probe.returncode == 0:
            ready = json.loads(probe.stdout)
            (evidence / (suffix + '-ready.json')).write_text(json.dumps(ready, indent=2) + '\n')
            executable = runtime.text('exec', instance, 'readlink', '/proc/' + str(ready['pids']['ganesha']) + '/exe')
            assert executable == '/usr/local/bin/ganesha.nfsd'
            maps = runtime.text('exec', instance, 'cat', '/proc/' + str(ready['pids']['ganesha']) + '/maps')
            libraries = '\n'.join(line for line in maps.splitlines() if any(name in line for name in
                         ('libganesha_nfsd', 'libfsallizardfs', 'liblizardfs-client', 'liblizardfsmount_shared')))
            assert '/usr/local/lib/libganesha_nfsd' in libraries and '/usr/local/lib/ganesha/libfsallizardfs' in libraries
            (evidence / (suffix + '-loaded-libraries.log')).write_text(libraries + '\n')
            port = int(runtime.text('port', instance, '2049/tcp').splitlines()[0].rsplit(':', 1)[1])
            return instance, port
        state = json.loads(runtime.text('inspect', instance))[0]['State']
        if not state.get('Running'):
            break
        time.sleep(0.5)
    logs = runtime.run('logs', instance, check=False)
    (evidence / (suffix + '-startup.log')).write_text(logs.stdout + logs.stderr)
    capture_service_logs(runtime, image, volume, suffix, evidence)

    raise RuntimeError('native server readiness failed; inspect startup evidence')


def stop_server(runtime, image, instance, volume, suffix, evidence):
    runtime.text('stop', '--time', '40', instance)
    info = json.loads(runtime.text('inspect', instance))[0]
    logs = runtime.run('logs', instance, check=False)
    (evidence / (suffix + '-server.log')).write_text(logs.stdout + logs.stderr)
    stopped = volume_command(runtime, image, volume, suffix + '-stopped', 'cat /state/lab-stopped.json').stdout
    result = json.loads(stopped)
    (evidence / (suffix + '-stopped.json')).write_text(json.dumps(result, indent=2) + '\n')
    assert info['State']['ExitCode'] == 0, 'native server did not close cleanly'
    return result


def client_stage(args, port, stage, case, evidence, manifest_path):
    command = [sys.executable, str(pathlib.Path(__file__).with_name('nfs4_client_runner.py')),
               '--connection', args.client_connection, '--image', args.client_image,
               '--port', str(port), '--stage', stage, '--manifest', str(manifest_path),
               '--case', case, '--principal', '1001', '--evidence-directory', str(evidence / 'client')]
    if args.disable_client_container_label:
        command += ['--disable-container-label']
    with (evidence / ('client-' + stage + '.log')).open('w') as output:
        subprocess.run(command, stdout=output, stderr=subprocess.STDOUT, timeout=240, check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--connection', default='podman-machine-default')
    parser.add_argument('--server-image', default='localhost/owlmail:nfs4-lab-server')
    parser.add_argument('--client-connection', default='podman-machine-default-root')
    parser.add_argument('--client-image', default='localhost/owlmail:nfs4-lab-client')
    parser.add_argument('--disable-client-container-label', action='store_true')
    parser.add_argument('--evidence-directory', required=True)
    args = parser.parse_args()
    runtime = RecoveryRuntime('podman', args.connection)
    evidence = pathlib.Path(args.evidence_directory).resolve(); evidence.mkdir(parents=True, exist_ok=True)
    report = {'status': 'running', 'runId': runtime.run_id,
              'scope': 'native LizardFS RichACL via Ganesha FSAL_LIZARDFS, isolated AUTH_SYS test, not OwlMail NFS deployment or Kerberos',
              'serverConnection': args.connection, 'clientConnection': args.client_connection,
              'noHostExportOrPolicyChanges': True,
              'scriptSHA256': hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
    diagnostic_volumes = []
    try:
        runtime.run('info', timeout=20)
        report['serverImageId'] = json.loads(runtime.text('image', 'inspect', args.server_image))[0]['Id']
        report['bootstrapSHA256'] = hashlib.sha256(pathlib.Path(__file__).with_name('nfs4_lab_server.py').read_bytes()).hexdigest()
        loader = 'import ctypes,os; ctypes.CDLL("/usr/local/lib/liblizardfs-client.so",mode=os.RTLD_NOW); ctypes.CDLL("/usr/local/lib/liblizardfsmount_shared.so",mode=os.RTLD_NOW); print("native shared clients RTLD_NOW passed")'
        preflight = container(runtime, args.server_image, 'native-client-loader', 'python3 -c ' + shlex.quote(loader))
        (evidence / 'native-client-loader.log').write_text(preflight.stdout + preflight.stderr)
        report['nativeClientRTLDNowPassed'] = True
        source, backup, restored = [runtime.volume(name) for name in ('nfs4-source', 'nfs4-backup', 'nfs4-restored')]
        diagnostic_volumes = [('source', source), ('restored', restored)]
        instance, port = start_server(runtime, args.server_image, source, 'source', evidence)
        report['sourcePort'] = port
        case = 'native-' + runtime.run_id
        acceptance_manifest = evidence / 'client-manifest.json'
        client_stage(args, port, 'prepare', case, evidence, acceptance_manifest)
        client_stage(args, port, 'confirm', case, evidence, acceptance_manifest)
        source_stopped = stop_server(runtime, args.server_image, instance, source, 'source', evidence)
        # This is the backing service state (master metadata/changelogs, chunk
        # data, configs and Ganesha recovery), never just a client ACL text dump.
        before = manifest(runtime, args.server_image, source, 'native-source-manifest')
        container(runtime, args.server_image, 'native-backup',
                  'umask 077; tar --numeric-owner --acls --xattrs -cpf /backup/native.tar -C /source .; sync',
                  [source + ':/source:ro', backup + ':/backup'])
        archive_sha = container(runtime, args.server_image, 'native-archive-sha', 'sha256sum /backup/native.tar',
                                 [backup + ':/backup:ro']).stdout.split()[0]
        container(runtime, args.server_image, 'native-restore',
                  'tar --numeric-owner --acls --xattrs -xpf /backup/native.tar -C /state; sync',
                  [backup + ':/backup:ro', restored + ':/state'])
        assert manifest(runtime, args.server_image, restored, 'native-restored-manifest') == before
        recovered, new_port = start_server(runtime, args.server_image, restored, 'restored', evidence)
        report['restoredPort'] = new_port
        client_stage(args, new_port, 'verify', case, evidence, acceptance_manifest)
        restore_stopped = stop_server(runtime, args.server_image, recovered, restored, 'restored', evidence)
        assert manifest(runtime, args.server_image, source, 'native-source-unchanged') == before
        assert container(runtime, args.server_image, 'native-archive-unchanged', 'sha256sum /backup/native.tar',
                         [backup + ':/backup:ro']).stdout.split()[0] == archive_sha
        provenance = container(runtime, args.server_image, 'native-provenance',
                               'cat /build/lizardfs-source-commit /build/ganesha-source-commit; '
                               'sha256sum /build/*.patch; dpkg-query -W nfs-ganesha; '
                               'ldd /usr/local/bin/ganesha.nfsd; ldd /usr/local/lib/ganesha/libfsallizardfs.so; '
                               'sha256sum /usr/local/bin/ganesha.nfsd /usr/local/lib/libganesha_nfsd.so.4.3 /usr/local/lib/ganesha/libfsallizardfs.so').stdout
        report.update(status='passed', sourceStateManifest=before, sourceManifestSHA256=digest(before),
                      nativeBackendArchiveSHA256=archive_sha, sourceUnchanged=True, archiveUnchanged=True,
                      sourceGracefulStop=source_stopped, restoredGracefulStop=restore_stopped,
                      nativeProvenance=provenance,
                      serverImageId=json.loads(runtime.text('image', 'inspect', args.server_image))[0]['Id'],
                      scriptSHA256=hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest())
        print('PASS native NFSv4: ordered ACE/deny/allow and inheritance with real IO; backing metadata/chunks restored into new volume and fresh client mount verified', flush=True)
    except Exception as error:
        report.update(status='failed', errorType=type(error).__name__)
        for diagnostic_stage, diagnostic_volume in diagnostic_volumes:
            try:
                capture_service_logs(runtime, args.server_image, diagnostic_volume, diagnostic_stage, evidence)
            except Exception as diagnostic_error:
                report[diagnostic_stage + 'LogCaptureError'] = type(diagnostic_error).__name__
        raise
    finally:
        report['createdContainerReferences'] = list(runtime.containers)
        report['createdVolumes'] = list(runtime.volumes)
        try:
            runtime.close()
            label = 'io.owlmail.verification-run=' + runtime.run_id
            remaining_containers = json.loads(runtime.text('ps', '--all', '--filter', 'label=' + label, '--format', 'json'))
            remaining_volumes = json.loads(runtime.text('volume', 'ls', '--filter', 'label=' + label, '--format', 'json'))
            report['remainingOwnedContainers'] = [item.get('Id', item.get('ID')) for item in remaining_containers]
            report['remainingOwnedVolumes'] = [item.get('Name', item.get('name')) for item in remaining_volumes]
            report['cleanupVerified'] = not remaining_containers and not remaining_volumes
            if not report['cleanupVerified']:
                report['status'] = 'failed'
        except Exception as cleanup_error:
            report.update(status='failed', cleanupVerified=False, cleanupErrorType=type(cleanup_error).__name__)
        (evidence / 'nfs4-lab-result.json').write_text(json.dumps(report, indent=2) + '\n')
        print('NFS4_LAB_EVIDENCE ' + json.dumps({k: v for k, v in report.items()
              if k not in ('sourceStateManifest', 'nativeProvenance')}, sort_keys=True), flush=True)
        if not report.get('cleanupVerified'):
            raise RuntimeError('owned native lab resource cleanup did not verify')



if __name__ == '__main__':
    main()
