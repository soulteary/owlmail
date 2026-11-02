#!/usr/bin/env python3
"""Test-only named-volume TLS continuity. No published ports or private-key exports."""
import argparse
import base64
import hashlib
import json
import pathlib
import subprocess
import tempfile
import time

from multiuser import fixture_request, initialize_volume
from recovery import RecoveryRuntime, container, volume_command, manifest, digest, wait_fixture


def openssl(*args):
    return subprocess.run(['openssl', *args], check=True, capture_output=True, timeout=20).stdout


def write(runtime, image, volume, suffix, name, content):
    assert content
    response = volume_command(runtime, image, volume, suffix,
                              'umask 077; mkdir -p /state/tls; base64 -d > /state/tls/' + name,
                              user='1000', data=base64.b64encode(content).decode())
    assert response.returncode == 0
    actual = volume_command(runtime, image, volume, suffix + '-hash',
                            'test -s /state/tls/' + name + '; sha256sum /state/tls/' + name,
                            user='1000').stdout.split()[0]
    assert actual == hashlib.sha256(content).hexdigest()


def create_fixture(runtime, image, volume, suffix):
    name = 'owlmail-tls-' + runtime.run_id + '-' + suffix
    instance = runtime.text('run', '--detach', '--name', name,
                            '--label', 'io.owlmail.verification-run=' + runtime.run_id,
                            '--volume', volume + ':/state', image,
                            '-state-directory', '/state', '-tls-cert', '/state/tls/server.pem',
                            '-tls-key', '/state/tls/server-key.pem', '-tls-ca', '/state/tls/ca.pem')
    runtime.containers.append(instance)
    return instance


def ready(runtime, image, volume, suffix):
    instance = create_fixture(runtime, image, volume, suffix)
    wait_fixture(runtime, instance)
    assert runtime.text('exec', instance, 'id', '-u') == '1000'
    assert runtime.text('exec', instance, 'id', '-g') == '1000'
    return instance


def stop(runtime, instance):
    runtime.text('stop', '--time', '10', instance)


def deliver(runtime, instance, user, subject):
    fixture_request(runtime, instance, '/__fixture__/deliver', body={
        'Recipient': user + '@example.test', 'Subject': subject,
        'Body': 'synthetic verified TLS delivery', 'Attachment': user + ' TLS attachment'})


def probe(runtime, instance, expected):
    result = fixture_request(runtime, instance, '/__fixture__/tls')
    assert result['verifiedChain'] and result['smtpAuthenticated']
    assert result['serverName'] == '127.0.0.1'
    assert result['peerDER_SHA256'] == expected['peerDER_SHA256']
    assert result['peerSPKI_SHA256'] == expected['peerSPKI_SHA256']
    return result


def restore(runtime, image, archive, suffix):
    volume = runtime.volume(suffix)
    container(runtime, image, 'extract-' + suffix, 'tar -xpf /backup/state.tar -C /state; sync',
              [archive + ':/backup:ro', volume + ':/state'])
    return volume


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--runtime', choices=('docker', 'podman'), default='docker')
    parser.add_argument('--connection')
    parser.add_argument('--image', required=True)
    parser.add_argument('--fixture-image', required=True, help='Test-tagged fixture image with persistent TLS flags')
    parser.add_argument('--evidence-directory', required=True)
    args = parser.parse_args()
    runtime = RecoveryRuntime(args.runtime, args.connection)
    evidence = pathlib.Path(args.evidence_directory); evidence.mkdir(parents=True, exist_ok=True)
    report = {'status': 'running', 'runId': runtime.run_id, 'runtime': args.runtime,
              'containerUID': 1000, 'containerGID': 1000, 'privateKeysExported': False,
              'scope': 'build-tagged loopback fixture; production gate unchanged',
              'rootlessMapping': 'UID/GID refer to container namespace; no host ownership/ACL equivalence claimed'}
    try:
        runtime.run('info', timeout=20)
        with tempfile.TemporaryDirectory(prefix='owlmail-tls-continuity-') as directory:
            root = pathlib.Path(directory)
            openssl('req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
                    '-subj', '/CN=OwlMail synthetic recovery CA', '-keyout', str(root / 'ca-key.pem'), '-out', str(root / 'ca.pem'))
            openssl('req', '-new', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=127.0.0.1',
                    '-keyout', str(root / 'server-key.pem'), '-out', str(root / 'server.csr'))
            (root / 'leaf.ext').write_text('subjectAltName=IP:127.0.0.1,DNS:localhost\nextendedKeyUsage=serverAuth\n')
            openssl('x509', '-req', '-in', str(root / 'server.csr'), '-CA', str(root / 'ca.pem'),
                    '-CAkey', str(root / 'ca-key.pem'), '-CAcreateserial', '-days', '1',
                    '-extfile', str(root / 'leaf.ext'), '-out', str(root / 'server.pem'))
            openssl('pkey', '-in', str(root / 'server-key.pem'), '-check', '-noout')
            der = openssl('x509', '-in', str(root / 'server.pem'), '-outform', 'DER')
            public = openssl('x509', '-in', str(root / 'server.pem'), '-pubkey', '-noout')
            (root / 'public.pem').write_bytes(public)
            spki = openssl('pkey', '-pubin', '-in', str(root / 'public.pem'), '-outform', 'DER')
            expected = {'peerDER_SHA256': hashlib.sha256(der).hexdigest(),
                        'peerSPKI_SHA256': hashlib.sha256(spki).hexdigest()}
            source, archive = runtime.volume('tls-source'), runtime.volume('tls-backup')
            initialize_volume(runtime, args.image, source, fixture=True)
            for name in ('ca.pem', 'server.pem', 'server-key.pem'):
                write(runtime, args.image, source, 'write-' + name, name, (root / name).read_bytes())
            original = ready(runtime, args.fixture_image, source, 'source')
            original_peer = probe(runtime, original, expected)
            for user in ('alice', 'bob'):
                deliver(runtime, original, user, user + ' before TLS backup')
            alice = fixture_request(runtime, original, '/api/v1/emails')
            bob = fixture_request(runtime, original, '/api/v1/emails', user='bob')
            original_ids = {alice['emails'][0]['id'], bob['emails'][0]['id']}
            stop(runtime, original)
            before = manifest(runtime, args.image, source, 'stopped-source')
            container(runtime, args.image, 'tar-backup', 'umask 077; tar -cpf /backup/state.tar -C /source .; sync',
                      [source + ':/source:ro', archive + ':/backup'])
            archive_hash = container(runtime, args.image, 'archive-hash', 'sha256sum /backup/state.tar',
                                     [archive + ':/backup:ro']).stdout.split()[0]
            restored = restore(runtime, args.image, archive, 'tls-restored')
            assert manifest(runtime, args.image, restored, 'restored-files') == before
            recovered = ready(runtime, args.fixture_image, restored, 'restored')
            recovered_peer = probe(runtime, recovered, expected)
            assert original_peer == recovered_peer
            for user in ('alice', 'bob'):
                deliver(runtime, recovered, user, user + ' after TLS restore')
            alice = fixture_request(runtime, recovered, '/api/v1/emails')
            bob = fixture_request(runtime, recovered, '/api/v1/emails', user='bob')
            assert alice['total'] == bob['total'] == 2
            assert original_ids.issubset({mail['id'] for mail in alice['emails'] + bob['emails']})
            fixture_request(runtime, recovered, '/api/v1/emails/' + bob['emails'][0]['id'] + '/raw', expected=404)
            stop(runtime, recovered)
            print('PASS TLS continuity: dedicated CA + IP SAN verification, peer DER/SPKI identical, AUTH/delivery/HTTP isolation after volume restore', flush=True)
            # Loading failures must terminate before either readiness or SMTP opens.
            openssl('genpkey', '-algorithm', 'RSA', '-pkeyopt', 'rsa_keygen_bits:2048', '-out', str(root / 'wrong-key.pem'))
            failures = {}
            for case in ('wrong-key', 'missing-cert'):
                broken = restore(runtime, args.image, archive, 'tls-' + case)
                if case == 'wrong-key':
                    write(runtime, args.image, broken, 'replace-key', 'server-key.pem', (root / 'wrong-key.pem').read_bytes())
                else:
                    volume_command(runtime, args.image, broken, 'remove-cert', 'rm /state/tls/server.pem', user='1000')
                instance = create_fixture(runtime, args.fixture_image, broken, case)
                for _ in range(80):
                    state = json.loads(runtime.text('inspect', instance))[0]['State']
                    if not state.get('Running'):
                        break
                    time.sleep(0.1)
                else:
                    raise AssertionError(case + ' did not fail closed')
                assert int(state['ExitCode']) != 0
                captured = runtime.run('logs', instance)
                logs = captured.stdout + captured.stderr
                assert ('fixture persistent TLS pair' in logs if case == 'wrong-key' else 'no such file' in logs)
                assert 'SMTP AUTH required' not in logs and 'SMTP TLS/STARTTLS enabled' not in logs
                failures[case] = {'exitCode': int(state['ExitCode']), 'startupRejectedBeforeListeners': True}
            # Actual CA/hostname validation must reject a TLS handshake rather
            # than accepting any readable certificate or falling back to self-sign.
            openssl('req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1', '-subj', '/CN=Untrusted recovery CA',
                    '-keyout', str(root / 'other-key.pem'), '-out', str(root / 'other-ca.pem'))
            (root / 'wrong-host.ext').write_text('subjectAltName=DNS:wrong-host.example.test\nextendedKeyUsage=serverAuth\n')
            openssl('x509', '-req', '-in', str(root / 'server.csr'), '-CA', str(root / 'ca.pem'),
                    '-CAkey', str(root / 'ca-key.pem'), '-CAcreateserial', '-days', '1',
                    '-extfile', str(root / 'wrong-host.ext'), '-out', str(root / 'wrong-host.pem'))
            for case, name, content in [('wrong-ca', 'ca.pem', (root / 'other-ca.pem').read_bytes()),
                                        ('wrong-host', 'server.pem', (root / 'wrong-host.pem').read_bytes())]:
                rejected = restore(runtime, args.image, archive, 'tls-' + case)
                write(runtime, args.image, rejected, 'write-' + case, name, content)
                instance = ready(runtime, args.fixture_image, rejected, case)
                fixture_request(runtime, instance, '/__fixture__/tls', expected=502)
                fixture_request(runtime, instance, '/__fixture__/deliver', body={
                    'Recipient': 'alice@example.test', 'Subject': 'must reject TLS', 'Body': 'not accepted'}, expected=502)
                assert fixture_request(runtime, instance, '/api/v1/emails')['total'] == 1
                stop(runtime, instance)
                failures[case] = {'TLSVerificationRejected': True, 'newMessageAccepted': False}
            assert manifest(runtime, args.image, source, 'final-source') == before
            assert container(runtime, args.image, 'final-archive', 'sha256sum /backup/state.tar',
                             [archive + ':/backup:ro']).stdout.split()[0] == archive_hash
            report.update(status='passed', peerBefore=original_peer, peerAfter=recovered_peer,
                          negatives=failures, sourceUnchanged=True, archiveUnchanged=True,
                          archiveSHA256=archive_hash, manifestSHA256=digest(before), manifest=before,
                          fixtureImageId=json.loads(runtime.text('image', 'inspect', args.fixture_image))[0]['Id'],
                          productionImageId=json.loads(runtime.text('image', 'inspect', args.image))[0]['Id'],
                          scriptSHA256=hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest())
            print('PASS TLS negatives: wrong key/missing certificate startup denied; wrong CA/hostname handshake and delivery rejected', flush=True)
    except Exception as error:
        report.update(status='failed', errorType=type(error).__name__)
        raise
    finally:
        runtime.close()
        (evidence / 'tls-volume-recovery.json').write_text(json.dumps(report, indent=2) + '\n')
        print('TLS_RECOVERY_EVIDENCE ' + json.dumps({k: v for k, v in report.items() if k != 'manifest'}, sort_keys=True), flush=True)


if __name__ == '__main__':
    main()
