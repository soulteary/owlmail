#!/usr/bin/env python3
"""Podman-machine-only drill: distinct real user namespaces in one Linux VM.
Requires existing rootless connection, auto ranges and read-only machine SSH sudo
probes. Does not edit VM configuration and does not prove dual-host recovery.
"""
import argparse
import json
import pathlib
import shlex
import subprocess
import time
from recovery import RecoveryRuntime, wait_fixture
from multiuser import fixture_request


def execute(runtime, instance, script, user='0', check=True):
    return runtime.run('exec', '--user', user, instance, '/bin/sh', '-c', 'set -eu; ' + script, check=check)


def anchor(runtime, image, volume, suffix):
    name = 'owlmail-mapping-' + runtime.run_id + '-' + suffix
    instance = runtime.text('run', '--detach', '--pull=never', '--name', name,
                            '--label', 'io.owlmail.verification-run=' + runtime.run_id,
                            '--userns=auto:size=65536', '--user', '0', '--volume', volume + ':/state',
                            '--entrypoint', '/bin/sh', image, '-c', 'exec sleep 600')
    runtime.containers.append(instance)
    execute(runtime, instance, 'chown 1000:1000 /state; chmod 700 /state')
    return instance


def start(runtime, instance):
    runtime.text('exec', '--detach', '--user', '1000:1000', instance, '/bin/sh', '-c',
                 'echo $$ >/state/fixture.pid; exec /app/owlmail-fixture -state-directory /state >/state/service.log 2>&1')
    wait_fixture(runtime, instance)


def stop(runtime, instance):
    execute(runtime, instance, 'kill -TERM "$(cat /state/fixture.pid)"')
    for _ in range(100):
        result = execute(runtime, instance, 'kill -0 "$(cat /state/fixture.pid)"', check=False)
        if result.returncode:
            execute(runtime, instance, 'rm /state/fixture.pid /state/service.log')
            return
        time.sleep(.1)
    raise RuntimeError('fixture did not stop before mapping snapshot')


def host(runtime, machine, command):
    result = subprocess.run([runtime.command[0], 'machine', 'ssh', machine, 'sudo ' + command],
                            capture_output=True, text=True, timeout=30, check=True)
    return result.stdout.strip()


def maps(runtime, machine, instance):
    info = json.loads(runtime.text('inspect', instance))[0]
    pid = info['State']['Pid']
    return {'container_uid_map': execute(runtime, instance, 'cat /proc/self/uid_map').stdout.strip(),
            'container_gid_map': execute(runtime, instance, 'cat /proc/self/gid_map').stdout.strip(),
            'vm_host_uid_map': host(runtime, machine, 'cat /proc/' + str(pid) + '/uid_map'),
            'vm_host_gid_map': host(runtime, machine, 'cat /proc/' + str(pid) + '/gid_map')}


def check_mail(runtime, instance, expected=None):
    result = {}
    for user in ['alice', 'bob']:
        inbox = fixture_request(runtime, instance, '/api/v1/emails', user=user)
        assert inbox['total'] == 1
        mail = inbox['emails'][0]
        assert mail['subject'] == user + ' mapping recovery'
        result[user] = mail['id']
        detail = fixture_request(runtime, instance, '/api/v1/emails/' + mail['id'], user=user)
        item = detail.get('email', detail)
        attachment = item['attachments'][0]
        name = attachment.get('generatedFileName') or attachment['fileName']
        body = runtime.text('exec', instance, 'wget', '-qO-', '--header=Cookie: fixture-user=' + user,
                            'http://127.0.0.1:21180/api/v1/emails/' + mail['id'] + '/attachments/' + name)
        assert body == user + ' mapping attachment'
    fixture_request(runtime, instance, '/api/v1/emails/' + result['bob'] + '/raw', expected=404)
    if expected is not None:
        assert result == expected
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--connection', default='podman-machine-default')
    parser.add_argument('--machine', default='podman-machine-default')
    parser.add_argument('--fixture-image', required=True)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    runtime = RecoveryRuntime('podman', args.connection)
    evidence = {'scope': 'same-Linux-VM cross-userns mapping; independent-host acceptance remains open'}
    try:
        source_volume, target_volume = runtime.volume('mapping-source'), runtime.volume('mapping-target')
        source = anchor(runtime, args.fixture_image, source_volume, 'source')
        target = anchor(runtime, args.fixture_image, target_volume, 'target')
        evidence['source_maps'], evidence['target_maps'] = maps(runtime, args.machine, source), maps(runtime, args.machine, target)
        for kind in ['vm_host_uid_map', 'vm_host_gid_map']:
            source_map = [list(map(int, line.split())) for line in evidence['source_maps'][kind].splitlines()]
            target_map = [list(map(int, line.split())) for line in evidence['target_maps'][kind].splitlines()]
            assert source_map != target_map
            assert all(a[1] + a[2] <= b[1] or b[1] + b[2] <= a[1] for a in source_map for b in target_map), 'host ranges overlap'
        start(runtime, source)
        for user in ['alice', 'bob']:
            fixture_request(runtime, source, '/__fixture__/deliver', body={'Recipient': user + '@example.test', 'Subject': user + ' mapping recovery', 'Body': user, 'Attachment': user + ' mapping attachment'})
        expected = check_mail(runtime, source)
        stop(runtime, source)
        source_path = json.loads(runtime.text('volume', 'inspect', source_volume))[0]['Mountpoint']
        target_path = json.loads(runtime.text('volume', 'inspect', target_volume))[0]['Mountpoint']
        evidence['source_vm_host_owner'] = host(runtime, args.machine, 'stat -c %u:%g ' + shlex.quote(source_path + '/control.sqlite'))
        before = execute(runtime, source, "find /state -type f -exec sha256sum {} + | sort").stdout
        source_metadata = execute(runtime, source, "find /state -exec stat -c '%F|%a|%u|%g|%s|%n' {} + | sort").stdout
        archive = execute(runtime, source, 'tar -C /state -cf - . | base64').stdout
        # Ancestor rootless namespace preserves physical IDs, deliberately wrong
        # for the target's different map. No :U or automatic recursive chown.
        name = 'owlmail-mapping-' + runtime.run_id + '-wrong-copy'
        runtime.containers.append(name)
        runtime.text('run', '--pull=never', '--name', name, '--label', 'io.owlmail.verification-run=' + runtime.run_id,
                     '--userns=host', '--user', '0', '--volume', source_volume + ':/source:ro', '--volume', target_volume + ':/target',
                     '--entrypoint', '/bin/sh', args.fixture_image, '-c', 'find /source -mindepth 1 -maxdepth 1 -exec cp -a {} /target/ \\;')
        evidence['wrong_copy_vm_host_owner'] = host(runtime, args.machine, 'stat -c %u:%g ' + shlex.quote(target_path + '/control.sqlite'))
        assert evidence['wrong_copy_vm_host_owner'] == evidence['source_vm_host_owner'], 'negative copy did not preserve physical source owner'
        denied = execute(runtime, target, 'if cat /state/control.sqlite >/dev/null; then exit 31; fi; if touch /state/wrong-write; then exit 32; fi; echo both-denied', user='1000:1000', check=False)
        assert denied.returncode == 0 and denied.stdout.strip() == 'both-denied', 'wrong physical owner copy unexpectedly readable or writable'
        evidence['wrong_physical_copy_denied'] = True
        # Remove bad copy in its ancestor namespace; restore numeric archive IDs
        # inside destination mapping, producing new physical host owners.
        parent_owner = []
        for kind in ['container_uid_map', 'container_gid_map']:
            entries = [list(map(int, line.split())) for line in evidence['target_maps'][kind].splitlines()]
            parent_owner.append(str(next(outside + 1000 - inside for inside, outside, count in entries if inside <= 1000 < inside + count)))
        runtime.text('run', '--pull=never', '--rm', '--userns=host', '--user', '0', '--volume', target_volume + ':/target',
                     '--entrypoint', '/bin/sh', args.fixture_image, '-c', 'find /target -mindepth 1 -maxdepth 1 -exec rm -rf {} +; chown ' + ':'.join(parent_owner) + ' /target; chmod 700 /target')
        execute(runtime, target, 'chown 1000:1000 /state; chmod 700 /state')
        runtime.run('exec', '--interactive', '--user', '0', target, '/bin/sh', '-c', 'base64 -d | tar -C /state -xpf -', input=archive)
        after = execute(runtime, target, "find /state -type f -exec sha256sum {} + | sort").stdout
        assert before == after
        evidence['target_vm_host_owner'] = host(runtime, args.machine, 'stat -c %u:%g ' + shlex.quote(target_path + '/control.sqlite'))
        assert evidence['source_vm_host_owner'] != evidence['target_vm_host_owner']
        for side in ['source', 'target']:
            expected_owner = []
            for kind in ['vm_host_uid_map', 'vm_host_gid_map']:
                matches = [outside + 1000 - inside for inside, outside, count in [list(map(int, line.split())) for line in evidence[side + '_maps'][kind].splitlines()] if inside <= 1000 < inside + count]
                assert len(matches) == 1, 'runtime UID/GID 1000 not uniquely mapped'
                expected_owner.append(str(matches[0]))
            assert evidence[side + '_vm_host_owner'] == ':'.join(expected_owner), 'physical inode owner does not match actual runtime mapping'
        assert execute(runtime, target, 'stat -c %u:%g /state/control.sqlite').stdout.strip() == '1000:1000'
        execute(runtime, target, 'touch /state/nonroot-write; rm /state/nonroot-write', user='1000:1000')
        start(runtime, target)
        check_mail(runtime, target, expected)
        evidence.update(numeric_restore_hashes_equal=True, target_nonroot_read_write=True, restored_ids_attachments_isolation=True)
        stop(runtime, target)
        assert execute(runtime, source, "find /state -type f -exec sha256sum {} + | sort").stdout == before
        assert execute(runtime, source, "find /state -exec stat -c '%F|%a|%u|%g|%s|%n' {} + | sort").stdout == source_metadata
        evidence['source_unchanged'] = True
        evidence['source_numeric_metadata_unchanged'] = True
        pathlib.Path(args.output).write_text(json.dumps(evidence, indent=2) + '\n')
        print('PASS: different real userns maps, VM host physical owners, wrong-copy denial, numeric restore, nonroot service isolation')
    finally:
        runtime.close()


if __name__ == '__main__':
    main()
