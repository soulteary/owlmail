#!/usr/bin/env python3
"""Strict SELinux recovery drill on an existing enforcing Podman machine.

Never changes enforcement or policy. Old MCS labels are deliberately rejected;
restoration excludes security.selinux and relies on destination-policy :Z relabel.
Disabled/permissive environments are unsupported (exit 2), never successful tests.
"""
import argparse
import hashlib
import json
import pathlib
import shlex
import subprocess
from recovery import RecoveryRuntime, wait_fixture
from multiuser import fixture_request, initialize_volume
from uid_mapping import check_mail


def machine(runtime, name, script, check=True):
    return subprocess.run([runtime.command[0], 'machine', 'ssh', name, script],
                          capture_output=True, text=True, timeout=45, check=check)


def command(runtime, name, script):
    return machine(runtime, name, 'sudo /bin/sh -c ' + shlex.quote('set -eu; ' + script)).stdout.strip()


def label(runtime, machine_name, path):
    value = command(runtime, machine_name, 'getfattr --only-values -n security.selinux ' + shlex.quote(path) + ' 2>/dev/null')
    return value.rstrip('\x00')


def create(runtime, image, volume, suffix, relabel='', service=False):
    name = 'owlmail-selinux-' + runtime.run_id + '-' + suffix
    args = ['run', '--detach', '--pull=never', '--name', name, '--label', 'io.owlmail.verification-run=' + runtime.run_id,
            '--volume', volume + ':/state' + (':' + relabel if relabel else '')]
    if service:
        args += [image, '-state-directory', '/state']
    else:
        args += ['--entrypoint', '/bin/sh', image, '-c', 'exec sleep 600']
    instance = runtime.text(*args)
    runtime.containers.append(instance)
    if service:
        wait_fixture(runtime, instance)
    return instance


def inspect(runtime, instance):
    info = json.loads(runtime.text('inspect', instance))[0]
    return {'process_label': info.get('ProcessLabel'), 'mount_label': info.get('MountLabel'),
            'image_id': info['Image'], 'runtime_user': info['Config'].get('User')}


def mountpoint(runtime, volume):
    return json.loads(runtime.text('volume', 'inspect', volume))[0]['Mountpoint']


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--connection', default='podman-machine-default')
    parser.add_argument('--machine', default='podman-machine-default')
    parser.add_argument('--fixture-image', required=True)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    runtime = RecoveryRuntime('podman', args.connection)
    evidence = {'status': 'unsupported', 'scope': 'one existing Podman Linux VM; target-policy relabel, not cross-host policy portability',
                'run_id': runtime.run_id, 'script_sha256': hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
    try:
        enforcement = machine(runtime, args.machine, 'getenforce; cat /sys/fs/selinux/enforce', check=False)
        evidence['enforcement_probe'] = enforcement.stdout.strip()
        info = json.loads(runtime.text('info', '--format', 'json'))
        evidence['runtime_security'] = info['host']['security']
        if enforcement.returncode or enforcement.stdout.strip().splitlines() != ['Enforcing', '1'] or not evidence['runtime_security'].get('selinuxEnabled'):
            evidence['reason'] = 'Requires actual enforcing SELinux and enabled container labeling; no successful skip.'
            raise SystemExit(2)
        evidence['status'] = 'failed'
        tools = command(runtime, args.machine, 'command -v getfattr; command -v chcon; command -v tar')
        evidence['host_tools'] = tools.splitlines()
        source_volume, backup_volume, target_volume = [runtime.volume('selinux-' + name) for name in ['source', 'backup', 'target']]
        for volume in [source_volume, target_volume]:
            initialize_volume(runtime, args.fixture_image, volume, fixture=True)
        source = create(runtime, args.fixture_image, source_volume, 'source', 'Z', True)
        evidence['source'] = inspect(runtime, source)
        for user in ['alice', 'bob']:
            fixture_request(runtime, source, '/__fixture__/deliver', body={'Recipient': user + '@example.test', 'Subject': user + ' mapping recovery', 'Body': user, 'Attachment': user + ' mapping attachment'})
        ids = check_mail(runtime, source)
        runtime.text('stop', '--time', '10', source)
        source_path, backup_path, target_path = [mountpoint(runtime, volume) for volume in [source_volume, backup_volume, target_volume]]
        evidence['source_file_label'] = label(runtime, args.machine, source_path + '/control.sqlite')
        assert evidence['source_file_label'] == evidence['source']['mount_label']
        source_hashes = command(runtime, args.machine, 'cd ' + shlex.quote(source_path) + '; find . -type f -exec sha256sum {} + | sort')
        archive = backup_path + '/stopped.tar'
        command(runtime, args.machine, 'umask 077; chmod 700 ' + shlex.quote(backup_path) + '; tar --numeric-owner --xattrs --xattrs-exclude=security.selinux -cpf ' + shlex.quote(archive) + ' -C ' + shlex.quote(source_path) + ' .')
        evidence['archive_mode'] = command(runtime, args.machine, 'stat -c %a ' + shlex.quote(archive))
        assert evidence['archive_mode'] == '600'
        evidence['archive_sha256'] = command(runtime, args.machine, 'sha256sum ' + shlex.quote(archive)).split()[0]
        wrong = create(runtime, args.fixture_image, target_volume, 'wrong-label')
        evidence['wrong_target'] = inspect(runtime, wrong)
        assert evidence['wrong_target']['mount_label'] != evidence['source']['mount_label'], 'MCS labels must differ'
        command(runtime, args.machine, 'tar --numeric-owner --xattrs --xattrs-exclude=security.selinux -xpf ' + shlex.quote(archive) + ' -C ' + shlex.quote(target_path))
        # Reproduce blindly retaining the old source MCS category, only on this
        # run's destination tree; this is deliberately not the restore strategy.
        command(runtime, args.machine, 'chcon -R ' + shlex.quote(evidence['source']['mount_label']) + ' ' + shlex.quote(target_path))
        evidence['wrong_file_label'] = label(runtime, args.machine, target_path + '/control.sqlite')
        evidence['wrong_file_dac'] = command(runtime, args.machine, 'stat -c %a:%u:%g ' + shlex.quote(target_path + '/control.sqlite'))
        evidence['wrong_file_sha256'] = command(runtime, args.machine, 'sha256sum ' + shlex.quote(target_path + '/control.sqlite')).split()[0]
        assert runtime.text('exec', wrong, 'id', '-u') == '1000'
        evidence['wrong_root_dac'] = command(runtime, args.machine, 'stat -c %a:%u:%g ' + shlex.quote(target_path))
        evidence['wrong_actual_process_label'] = runtime.text('exec', wrong, 'cat', '/proc/self/attr/current').rstrip('\x00')
        denial = runtime.run('exec', '--user', '1000:1000', wrong, '/bin/sh', '-c',
                            'if cat /state/control.sqlite >/dev/null; then exit 31; fi; if touch /state/wrong-write; then exit 32; fi; echo both-denied', check=False)
        assert denial.returncode == 0 and denial.stdout.strip() == 'both-denied', 'old MCS label did not deny real nonroot I/O'
        evidence['wrong_mcs_read_and_write_denied'] = True
        evidence['wrong_mcs_denial_stderr'] = denial.stderr.strip()
        audit = command(runtime, args.machine, "journalctl -k --since '-2 minutes' --no-pager 2>/dev/null | grep -F denied | grep -F " + shlex.quote(evidence['wrong_actual_process_label']) + ' | tail -8 || true')
        evidence['matching_avc_kernel_records'] = audit.splitlines()
        evidence['audit_record_observable'] = bool(audit)
        if not audit:
            evidence['audit_limitation'] = 'No matching denial record exposed by existing kernel journal; actual MCS/unchanged DAC differential and read/write denial are verified.'
        runtime.text('stop', '--time', '1', wrong)
        restored = create(runtime, args.fixture_image, target_volume, 'restored', 'Z', True)
        evidence['restored'] = inspect(runtime, restored)
        evidence['restored_file_label'] = label(runtime, args.machine, target_path + '/control.sqlite')
        evidence['restored_file_dac'] = command(runtime, args.machine, 'stat -c %a:%u:%g ' + shlex.quote(target_path + '/control.sqlite'))
        evidence['restored_file_sha256'] = command(runtime, args.machine, 'sha256sum ' + shlex.quote(target_path + '/control.sqlite')).split()[0]
        evidence['restored_root_dac'] = command(runtime, args.machine, 'stat -c %a:%u:%g ' + shlex.quote(target_path))
        evidence['restored_actual_process_label'] = runtime.text('exec', restored, 'cat', '/proc/self/attr/current').rstrip('\x00')
        assert evidence['restored_root_dac'] == evidence['wrong_root_dac']
        assert evidence['restored_file_dac'] == evidence['wrong_file_dac']
        assert evidence['restored_file_sha256'] == evidence['wrong_file_sha256']
        assert evidence['restored_file_label'] == evidence['restored']['mount_label']
        assert evidence['restored_file_label'] != evidence['source_file_label']
        assert runtime.text('exec', restored, 'id', '-u') == '1000'
        runtime.text('exec', restored, '/bin/sh', '-c', 'touch /state/nonroot-write; rm /state/nonroot-write')
        check_mail(runtime, restored, ids)
        runtime.text('stop', '--time', '10', restored)
        assert command(runtime, args.machine, 'cd ' + shlex.quote(source_path) + '; find . -type f -exec sha256sum {} + | sort') == source_hashes
        evidence['source_unchanged'] = True
        evidence['restored_ids_attachments_and_isolation'] = True
        # Shared :z uses local container_file_t:s0, without copying old MCS.
        shared_volume = runtime.volume('selinux-shared')
        initialize_volume(runtime, args.fixture_image, shared_volume, fixture=True)
        shared_a = create(runtime, args.fixture_image, shared_volume, 'shared-a', 'z')
        shared_b = create(runtime, args.fixture_image, shared_volume, 'shared-b', 'z')
        runtime.text('exec', shared_a, '/bin/sh', '-c', 'echo synthetic-shared >/state/probe')
        assert runtime.text('exec', shared_b, 'cat', '/state/probe') == 'synthetic-shared'
        evidence['shared_z_file_label'] = label(runtime, args.machine, mountpoint(runtime, shared_volume) + '/probe')
        assert evidence['shared_z_file_label'].endswith(':container_file_t:s0')
        evidence['shared_z_actual_access'] = True
        evidence['enforcement_after'] = machine(runtime, args.machine, 'getenforce; cat /sys/fs/selinux/enforce').stdout.strip()
        assert evidence['enforcement_after'].splitlines() == ['Enforcing', '1']
        evidence['status'] = 'passed'
        print('PASS: actual enforcing SELinux, old MCS I/O denial, destination :Z relabel, nonroot service recovery, shared :z')
    finally:
        runtime.close()
        evidence['remaining_owned_containers'] = len(json.loads(runtime.text('ps', '--all', '--filter', 'label=io.owlmail.verification-run=' + runtime.run_id, '--format', 'json')))
        evidence['remaining_owned_volumes'] = len(json.loads(runtime.text('volume', 'ls', '--filter', 'label=io.owlmail.verification-run=' + runtime.run_id, '--format', 'json')))
        if evidence['remaining_owned_containers'] or evidence['remaining_owned_volumes']:
            evidence['status'] = 'failed'
        pathlib.Path(args.output).write_text(json.dumps(evidence, indent=2) + '\n')
        assert not evidence['remaining_owned_containers'] and not evidence['remaining_owned_volumes'], 'owned resources remain after cleanup'


if __name__ == '__main__':
    main()
