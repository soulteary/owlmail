#!/usr/bin/env python3
"""Offline named-volume recovery drill. Synthetic identities only; no gate bypass.

No network ports are published. The immutable archive (including synthetic key
material) stays inside this run's private backup volume and is removed finally.
Evidence contains hashes and numeric metadata, never private key bytes.
"""
import argparse
import base64
import hashlib
import json
import pathlib
import subprocess
import tempfile
import time

from multiuser import Runtime, fixture_request, initialize_volume


class RecoveryRuntime(Runtime):
    def close(self):
        # Check the run label before removing even uniquely named containers:
        # a failed create/name collision must never delete an existing resource.
        for instance in reversed(self.containers):
            response = self.run('inspect', instance, check=False)
            if response.returncode == 0:
                labels = json.loads(response.stdout)[0].get('Config', {}).get('Labels') or {}
                if labels.get('io.owlmail.verification-run') == self.run_id:
                    self.run('rm', '--force', instance, check=False)
        self.containers = []
        super().close()


def container(runtime, image, suffix, script, mounts=(), user='0', check=True, data=None):
    name = 'owlmail-recovery-' + runtime.run_id + '-' + suffix
    runtime.containers.append(name)
    args = ['run', '--name', name, '--label', 'io.owlmail.verification-run=' + runtime.run_id, '--user', user]
    if data is not None:
        args += ['--interactive']
    for mount in mounts:
        args += ['--volume', mount]
    return runtime.run(*args, '--entrypoint', '/bin/sh', image, '-c', 'set -eu; ' + script,
                       check=check, input=data, timeout=45)


def volume_command(runtime, image, volume, suffix, script, **kwargs):
    return container(runtime, image, suffix, script, [volume + ':/state'], **kwargs)


def manifest(runtime, image, volume, suffix):
    # Delimiters/newlines are not accepted in synthetic fixture paths.
    result = volume_command(runtime, image, volume, suffix,
                            "find /state -exec stat -c '%F|%a|%u|%g|%s|%n' {} +; "
                            "echo __HASHES__; find /state -type f -exec sha256sum {} +").stdout
    metadata, checksums = result.split('__HASHES__\n')
    hashes = {}
    for line in checksums.splitlines():
        digest, name = line.split('  ', 1)
        hashes[name] = digest
    entries = {}
    for line in metadata.splitlines():
        kind, mode, uid, gid, size, name = line.split('|', 5)
        if kind not in ('directory', 'regular file', 'regular empty file'):
            raise AssertionError('unexpected nonregular snapshot entry')
        item = {'kind': 'directory' if kind == 'directory' else 'file',
                'mode': mode, 'uid': int(uid), 'gid': int(gid)}
        if kind != 'directory':
            item.update(size=int(size), sha256=hashes[name])
        entries[name.removeprefix('/state') or '/'] = item
    return entries


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True).encode()).hexdigest()


def wait_fixture(runtime, instance):
    for _ in range(60):
        response = runtime.run('exec', instance, 'wget', '-qO-',
                               'http://127.0.0.1:21180/__fixture__/health', check=False, timeout=5)
        if response.returncode == 0:
            return
        state = json.loads(runtime.text('inspect', instance))[0]['State']
        if not state.get('Running'):
            raise RuntimeError('fixture exited: ' + runtime.text('logs', instance))
        time.sleep(0.25)
    raise RuntimeError('fixture readiness deadline expired')


def start_fixture(runtime, image, volume, suffix):
    name = 'owlmail-recovery-' + runtime.run_id + '-' + suffix
    instance = runtime.text('run', '--detach', '--name', name, '--label', 'io.owlmail.verification-run=' + runtime.run_id,
                            '--volume', volume + ':/state', image, '-state-directory', '/state')
    runtime.containers.append(instance)
    wait_fixture(runtime, instance)
    assert runtime.text('exec', instance, 'id', '-u') == '1000'
    assert runtime.text('exec', instance, 'id', '-g') == '1000'
    return instance


def credential_bundle(runtime, image, volume):
    with tempfile.TemporaryDirectory(prefix='owlmail-recovery-cert-') as directory:
        root = pathlib.Path(directory)
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
                        '-subj', '/CN=synthetic-recovery', '-keyout', str(root / 'key.pem'),
                        '-out', str(root / 'cert.pem')], check=True, capture_output=True, timeout=20)
        subprocess.run(['openssl', 'pkey', '-in', str(root / 'key.pem'), '-check', '-noout'],
                       check=True, capture_output=True, timeout=10)
        subprocess.run(['openssl', 'x509', '-in', str(root / 'cert.pem'), '-noout'],
                       check=True, capture_output=True, timeout=10)
        for name in ('key.pem', 'cert.pem'):
            original = (root / name).read_bytes()
            assert original
            encoded = base64.b64encode(original).decode()
            volume_command(runtime, image, volume, 'write-' + name,
                           'umask 077; mkdir -p /state/credentials; base64 -d > /state/credentials/' + name,
                           user='1000', data=encoded)
            actual = volume_command(runtime, image, volume, 'verify-' + name,
                                    'test -s /state/credentials/' + name + '; sha256sum /state/credentials/' + name, user='1000').stdout.split()[0]
            assert actual == hashlib.sha256(original).hexdigest(), 'synthetic credential bytes were not written'
    # The fixture itself generates an ephemeral TLS key. This explicitly backed
    # up operator credential bundle is synthetic; no TLS continuity is claimed.
    config = {'testOnly': True, 'stateDirectory': '/state', 'uid': 1000, 'gid': 1000,
              'fixtureTLS': 'ephemeral', 'operatorCredentials': 'credentials/key.pem',
              'systemSMTPAccounts': 'accounts.yaml'}
    config_bytes = json.dumps(config)
    volume_command(runtime, image, volume, 'write-config',
                   'umask 077; cat > /state/deployment.json', user='1000', data=config_bytes)
    actual_config = volume_command(runtime, image, volume, 'verify-config',
                                   'test -s /state/deployment.json; cat /state/deployment.json', user='1000').stdout
    assert json.loads(actual_config) == config
    assert hashlib.sha256(actual_config.encode()).digest() == hashlib.sha256(config_bytes.encode()).digest()


def verify_mail(runtime, instance, alice_id, bob_id):
    alice = fixture_request(runtime, instance, '/api/v1/emails')
    bob = fixture_request(runtime, instance, '/api/v1/emails', user='bob')
    assert alice['total'] == bob['total'] == 1
    assert alice['emails'][0]['id'] == alice_id and bob['emails'][0]['id'] == bob_id
    assert alice['emails'][0]['subject'] == 'alice named-volume recovery'
    assert bob['emails'][0]['subject'] == 'bob named-volume recovery'
    fixture_request(runtime, instance, '/api/v1/emails/' + bob_id + '/raw', expected=404)
    detail = fixture_request(runtime, instance, '/api/v1/emails/' + alice_id)
    # The full message route returns the email DTO directly.
    mail = detail.get('email', detail)
    attachment = mail['attachments'][0]
    filename = attachment.get('generatedFileName') or attachment['fileName']
    response = runtime.text('exec', instance, 'wget', '-qO-', '--header=Cookie: fixture-user=alice',
                            'http://127.0.0.1:21180/api/v1/emails/' + alice_id + '/attachments/' + filename)
    assert response == 'alice recovery attachment', response


def run_index(runtime, image, binary, evidence):
    volume = runtime.volume('sqlite-index')
    initialize_volume(runtime, image, volume, fixture=True)
    name = 'owlmail-recovery-' + runtime.run_id + '-sqlite-index'
    runtime.containers.append(name)
    runtime.text('create', '--name', name, '--label', 'io.owlmail.verification-run=' + runtime.run_id, '--user', '1000:1000', '--volume', volume + ':/recovery',
                 '--env', 'OWLMAIL_SQLITE_RECOVERY_ROOT=/recovery', '--entrypoint', '/tmp/recovery.test', image,
                 '-test.v', '-test.run', '^TestMultiuserSQLiteIndexDeleteAndRebuild$', '-test.timeout', '120s')
    runtime.text('cp', str(pathlib.Path(binary).resolve()), name + ':/tmp/recovery.test')
    result = runtime.run('start', '--attach', name, timeout=140)
    assert runtime.text('inspect', name, '--format', '{{.State.ExitCode}}') == '0'
    assert 'PASS: TestMultiuserSQLiteIndexDeleteAndRebuild' in result.stdout
    if evidence:
        (evidence / 'sqlite-index-volume.log').write_text(result.stdout + result.stderr)
    print('PASS named-volume SQLite index: nonroot test binary; authoritative control/mail preserved', flush=True)


def verify(args, runtime):
    source, backup, restored = [runtime.volume(name) for name in ('source', 'immutable-backup', 'restored')]
    initialize_volume(runtime, args.image, source, fixture=True)
    volume_command(runtime, args.image, source, 'source-mode', 'chmod 700 /state')
    writer = start_fixture(runtime, args.fixture_image, source, 'writer')
    credential_bundle(runtime, args.image, source)
    for user in ('alice', 'bob'):
        fixture_request(runtime, writer, '/__fixture__/deliver', body={
            'Recipient': user + '@example.test', 'Subject': user + ' named-volume recovery',
            'Body': 'private-' + user, 'Attachment': user + ' recovery attachment'})
    assert fixture_request(runtime, writer, '/api/v1/emails', verified=False)['total'] == 0
    # Alice exists without claiming, so verified login after restore performs a
    # real pending->existing-personal merge; Bob remains a fresh pending box.
    original_ids = {}
    original_rows = runtime.text('exec', writer, '/bin/sh', '-c',
                                'for f in /state/mail/*.eml; do basename "$f" .eml | tr "\\n" "|"; sed -n "s/^Subject: //p" "$f"; done')
    for row in original_rows.splitlines():
        mail_id, subject = row.split('|', 1)
        original_ids[subject.strip().split(' ', 1)[0]] = mail_id
    assert set(original_ids) == {'alice', 'bob'}
    runtime.text('stop', '--time', '10', writer)
    before = manifest(runtime, args.image, source, 'source-manifest')
    assert '/control.sqlite' in before and '/accounts.yaml' in before
    assert before['/credentials/key.pem']['mode'] == '600'
    for name in ('/credentials/key.pem', '/credentials/cert.pem', '/deployment.json', '/accounts.yaml'):
        assert before[name]['size'] > 0, 'empty deployment credential/config file: ' + name
    container(runtime, args.image, 'archive',
              'umask 077; tar -cpf /backup/state.tar -C /source .; sync; sha256sum /backup/state.tar',
              [source + ':/source:ro', backup + ':/backup'])
    archive_hash = container(runtime, args.image, 'archive-hash', 'sha256sum /backup/state.tar',
                             [backup + ':/backup:ro']).stdout.split()[0]
    container(runtime, args.image, 'restore', 'tar -xpf /backup/state.tar -C /state; sync',
              [backup + ':/backup:ro', restored + ':/state'])
    assert manifest(runtime, args.image, restored, 'restored-manifest') == before, 'numeric ownership/mode/hash changed'
    print('PASS archive: stopped writer; complete named volume; numeric uid/gid/modes and hashes retained', flush=True)
    # Make permissions actually wrong, then test actual reads/writes as UID1000.
    volume_command(runtime, args.image, restored, 'break-owner', 'chown -R 0:0 /state; chmod 700 /state')
    denied = volume_command(runtime, args.image, restored, 'permission-denial',
                            'if cat /state/control.sqlite >/dev/null 2>&1; then exit 31; fi; '
                            'if ( : >> /state/credentials/key.pem ) 2>/dev/null; then exit 32; fi; '
                            'if ( : > /state/write-probe ) 2>/dev/null; then exit 33; fi; echo access-denied', user='1000')
    assert 'access-denied' in denied.stdout
    # Restore permissions from archived numeric metadata, rather than chmod777.
    container(runtime, args.image, 'repair-from-archive', 'tar -xpf /backup/state.tar -C /state; sync',
              [backup + ':/backup:ro', restored + ':/state'])
    assert manifest(runtime, args.image, restored, 'repaired-manifest') == before
    volume_command(runtime, args.image, restored, 'repaired-access',
                   'test -r /state/control.sqlite; test -r /state/credentials/key.pem; '
                   ': >> /state/credentials/key.pem; : > /state/write-probe; rm /state/write-probe', user='1000')
    restored_writer = start_fixture(runtime, args.fixture_image, restored, 'restored-writer')
    assert fixture_request(runtime, restored_writer, '/api/v1/emails', verified=False)['total'] == 0
    alice = fixture_request(runtime, restored_writer, '/api/v1/emails')
    bob = fixture_request(runtime, restored_writer, '/api/v1/emails', user='bob')
    verify_mail(runtime, restored_writer, original_ids['alice'], original_ids['bob'])
    runtime.text('stop', '--time', '10', restored_writer)
    assert manifest(runtime, args.image, source, 'unchanged-source') == before
    assert container(runtime, args.image, 'unchanged-archive', 'sha256sum /backup/state.tar',
                     [backup + ':/backup:ro']).stdout.split()[0] == archive_hash
    print('PASS restored UID1000: pending claim/merge, cross-user denial, attachment; wrong-owner actual read/write denial then archive metadata repair', flush=True)
    info = json.loads(runtime.text('info', '--format', '{{json .}}'))
    image = json.loads(runtime.text('image', 'inspect', args.image))[0]
    report = {'status': 'running', 'runtime': args.runtime, 'runId': runtime.run_id, 'imageId': image['Id'],
              'fixtureImageId': json.loads(runtime.text('image', 'inspect', args.fixture_image))[0]['Id'],
              'containerUID': 1000, 'containerGID': 1000, 'manifest': before,
              'manifestSHA256': digest(before), 'archiveSHA256': archive_hash,
              'rootlessMapping': 'numeric IDs are container-namespace IDs, not host UID/GID; no host ACL/xattr/SELinux equivalence claimed',
              'rootless': info.get('host', {}).get('security', {}).get('rootless'),
              'privateKeysExported': False, 'fixtureTLS': 'ephemeral; operator credential archive roundtrip only'}
    evidence = pathlib.Path(args.evidence_directory) if args.evidence_directory else None
    if evidence:
        evidence.mkdir(parents=True, exist_ok=True)
    run_index(runtime, args.image, args.recovery_binary, evidence)
    report.update(status='passed', sqliteIndex='passed', sourceUnchanged=True,
                  immutableArchiveUnchanged=True, sourceMessageIDsPreserved=True,
                  wrongOwnerAccessDenied=True, numericPermissionsRepaired=True,
                  nonemptyCredentialBytesVerified=True, sourceCredentialPEMParsed=True,
                  deploymentJSONParsed=True)
    if evidence:
        (evidence / 'named-volume-recovery.json').write_text(json.dumps(report, indent=2) + '\n')
    print('RECOVERY_EVIDENCE ' + json.dumps({k: v for k, v in report.items() if k != 'manifest'}, sort_keys=True), flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--runtime', choices=('docker', 'podman'), default='docker')
    parser.add_argument('--connection')
    parser.add_argument('--image', required=True)
    parser.add_argument('--fixture-image', required=True)
    parser.add_argument('--recovery-binary', required=True, help='Linux test binary containing the SQLite index recovery component test')
    parser.add_argument('--evidence-directory')
    args = parser.parse_args()
    runtime = RecoveryRuntime(args.runtime, args.connection)
    try:
        runtime.run('info', timeout=20)
        verify(args, runtime)
    except Exception as error:
        if args.evidence_directory:
            evidence = pathlib.Path(args.evidence_directory); evidence.mkdir(parents=True, exist_ok=True)
            (evidence / 'named-volume-recovery.json').write_text(json.dumps({
                'status': 'failed', 'runId': runtime.run_id, 'runtime': args.runtime,
                'errorType': type(error).__name__, 'privateKeysExported': False}, indent=2) + '\n')
        raise
    finally:
        runtime.close()


if __name__ == '__main__':
    main()
