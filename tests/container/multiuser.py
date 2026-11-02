#!/usr/bin/env python3
"""Real container checks; no production gate bypass or external Python packages.

Build the production image normally, then the explicitly tagged fixture image:
  docker build --build-arg BASE_IMAGE=owlmail:check --build-arg TARGETARCH=amd64 \
    -f tests/container/Dockerfile.multiuser-fixture -t owlmail:fixture .
  python3 tests/container/multiuser.py --image owlmail:check --fixture-image owlmail:fixture

Podman remote may use --runtime podman --connection <name>. Only uniquely named
resources created by this process are removed. Fixture ports are not published.
"""
import argparse
import base64
import json
import pathlib
import smtplib
import subprocess
import tempfile
import time
import uuid
from email.message import EmailMessage
from urllib.error import HTTPError, URLError
from urllib.request import ProxyHandler, Request, build_opener

HTTP = build_opener(ProxyHandler({}))


class Runtime:
    def __init__(self, name, connection=None):
        self.command = [name]
        if connection:
            if name != 'podman':
                raise ValueError('--connection is supported only with Podman')
            self.command += ['--connection', connection]
        self.containers = []
        self.volumes = []
        self.run_id = uuid.uuid4().hex[:12]

    def run(self, *args, check=True, timeout=60, input=None):
        result = subprocess.run(self.command + list(args), capture_output=True,
                                text=True, timeout=timeout, input=input)
        if check and result.returncode:
            raise RuntimeError('container command failed: ' + ' '.join(args[:3]) + '\n' + result.stderr.strip() + '\n' + result.stdout.strip())
        return result

    def text(self, *args, **kwargs):
        return self.run(*args, **kwargs).stdout.strip()

    def volume(self, suffix):
        name = 'owlmail-check-' + self.run_id + '-' + suffix
        self.text('volume', 'create', '--label', 'io.owlmail.verification-run=' + self.run_id, name)
        self.volumes.append(name)
        return name

    def start(self, image, suffix, *args, volume=None, fixture=False):
        name = 'owlmail-check-' + self.run_id + '-' + suffix
        options = ['run', '--detach', '--name', name]
        if volume:
            options += ['--volume', volume + (':/state' if fixture else ':/data')]
        if not fixture:
            options += ['--publish', '127.0.0.1::1025', '--publish', '127.0.0.1::1080']
        container = self.text(*options, image, *args)
        self.containers.append(container)
        return container

    def close(self):
        for container in reversed(self.containers):
            self.run('rm', '--force', container, check=False)
        for volume in reversed(self.volumes):
            metadata = self.run('volume', 'inspect', volume, check=False)
            if metadata.returncode == 0 and json.loads(metadata.stdout)[0].get('Labels', {}).get('io.owlmail.verification-run') == self.run_id:
                self.run('volume', 'rm', volume, check=False)


def port(runtime, container, number):
    return int(runtime.text('port', container, str(number) + '/tcp').splitlines()[0].rsplit(':', 1)[1])


def get(url):
    with HTTP.open(Request(url), timeout=5) as response:
        return response.status, response.read()


def wait_ready(runtime, container):
    endpoint = 'http://127.0.0.1:' + str(port(runtime, container, 1080))
    for _ in range(60):
        try:
            if get(endpoint + '/readyz')[0] == 200:
                return endpoint
        except (URLError, TimeoutError):
            pass
        state = json.loads(runtime.text('inspect', container))[0]['State']
        if not state.get('Running'):
            raise RuntimeError('container exited: ' + runtime.text('logs', container))
        time.sleep(0.5)
    raise RuntimeError('container did not become ready: ' + runtime.text('logs', container))


def initialize_volume(runtime, image, volume, fixture=False):
    target = '/state' if fixture else '/data'
    script = 'set -eu; mkdir -p ' + target + '; chown 1000:1000 ' + target
    if not fixture:
        script += '; mkdir -p /data/mail /data/fixtures /data/control /data/multi-mail; chown -R 1000:1000 /data'
    runtime.text('run', '--rm', '--user', '0', '--volume', volume + ':' + target,
                 '--entrypoint', '/bin/sh', image, '-c', script)


def helper(runtime, image, volume, script, input=None):
    return runtime.text('run', '--rm', '--volume', volume + ':/data',
                        '--entrypoint', '/bin/sh', image, '-c', 'set -eu; ' + script, input=input)


def write_fixture(runtime, image, volume, name, content):
    # These names are constants controlled by this test, never external input.
    encoded = base64.b64encode(content).decode()
    helper(runtime, image, volume, 'umask 077; base64 -d > /data/fixtures/' + name, input=encoded)


def verify_production(runtime, image):
    volume = runtime.volume('production')
    initialize_volume(runtime, image, volume)
    container = runtime.start(image, 'standalone', '-mail-directory', '/data/mail', volume=volume)
    endpoint = wait_ready(runtime, container)
    assert runtime.text('exec', container, 'id', '-u') == '1000', 'production process must run as uid 1000'
    status, page = get(endpoint + '/')
    assert status == 200 and b'<html' in page.lower(), 'embedded HTML missing from image'
    message = EmailMessage()
    message['From'] = 'phorge@example.test'
    message['To'] = 'alice@example.test'
    message['Subject'] = 'container-' + runtime.run_id
    message['X-OwlMail-Run-ID'] = runtime.run_id
    message.set_content('synthetic container persistence check')
    message.add_attachment(b'synthetic attachment', maintype='text', subtype='plain', filename='container.txt')
    with smtplib.SMTP('127.0.0.1', port(runtime, container, 1025), timeout=10) as client:
        client.send_message(message)
    emails = json.loads(get(endpoint + '/api/v1/emails')[1])['emails']
    assert len(emails) == 1 and emails[0]['subject'] == message['Subject']
    email_id = emails[0]['id']
    assert emails[0]['attachments'], 'SMTP attachment was not retained'
    assert runtime.text('exec', container, 'stat', '-c', '%a:%u:%g', '/data/mail/' + email_id + '.eml') == '600:1000:1000'
    runtime.text('exec', container, '/app/owlmail', 'healthcheck', '--container')
    runtime.text('exec', container, '/app/owlmail', 'healthcheck', '--ready', '--container')
    runtime.text('restart', container)
    endpoint = wait_ready(runtime, container)
    restarted = json.loads(get(endpoint + '/api/v1/emails')[1])['emails']
    assert len(restarted) == 1 and restarted[0]['id'] == email_id, 'restart lost committed mail'
    runtime.text('exec', container, '/app/owlmail', 'healthcheck', '--container')
    runtime.text('stop', '--time', '5', container)
    before = sorted(helper(runtime, image, volume, 'find /data/mail -type f -exec sha256sum {} +').splitlines())
    imported = runtime.text('run', '--rm', '--volume', volume + ':/data', image,
                           'migrate-mailbox', '-source', '/data/mail', '-destination', '/data/imported',
                           '-direction', 'standalone-to-multiuser', '-identity-namespace', 'container',
                           '-subject', 'alice', '-offline')
    receipt = json.loads(imported)
    assert receipt['emails'] == 1 and receipt['attachments'] == 1
    assert helper(runtime, image, volume, 'stat -c "%a:%u:%g" /data/imported/control.sqlite') == '600:1000:1000'
    assert helper(runtime, image, volume, 'test -f /data/imported/mail/.owlmail-multiuser; test ! -e /data/imported/mail/control.sqlite; echo separated') == 'separated'
    exported = runtime.text('run', '--rm', '--volume', volume + ':/data', image,
                           'migrate-mailbox', '-source', '/data/imported/mail', '-control-database', '/data/imported/control.sqlite',
                           '-destination', '/data/exported', '-direction', 'multiuser-to-standalone',
                           '-mailbox-id', receipt['mailboxId'], '-offline')
    assert json.loads(exported)['emails'] == 1
    after = sorted(helper(runtime, image, volume, 'find /data/mail -type f -exec sha256sum {} +').splitlines())
    assert before == after, 'offline migration modified its source'
    recovered = runtime.start(image, 'exported', '-mail-directory', '/data/exported/mail', volume=volume)
    exported_api = wait_ready(runtime, recovered)
    assert len(json.loads(get(exported_api + '/api/v1/emails')[1])['emails']) == 1
    runtime.text('stop', '--time', '5', recovered)
    verify_gates(runtime, image, volume)
    print('PASS production: nonroot, embedded HTML, SMTP/attachment, HTTP, restart, packaged healthcheck, offline round-trip and gate', flush=True)


def verify_gates(runtime, image, volume):
    with tempfile.TemporaryDirectory(prefix='owlmail-container-cert-') as directory:
        root = pathlib.Path(directory)
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
                        '-subj', '/CN=localhost', '-addext', 'subjectAltName=DNS:localhost',
                        '-keyout', str(root / 'key.pem'), '-out', str(root / 'cert.pem')],
                       check=True, capture_output=True, timeout=30)
        write_fixture(runtime, image, volume, 'key.pem', (root / 'key.pem').read_bytes())
        write_fixture(runtime, image, volume, 'cert.pem', (root / 'cert.pem').read_bytes())
    # A synthetic bcrypt fixture; production startup must reject at its public
    # mode gate before listeners or a control database are created.
    accounts = b'version: 1\naccounts:\n  - id: gorge\n    username: gorge\n    password-hash: $2b$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2uheWG/igi.\n    namespace: main\n    enabled: true\n    allowed-domains: [example.test]\n'
    write_fixture(runtime, image, volume, 'accounts.yaml', accounts)
    flags = ['-mode', 'multiuser', '-mail-directory', '/data/multi-mail', '-control-database', '/data/control/control.sqlite',
             '-identity-trusted-proxies', '127.0.0.1/32', '-claim-policy', 'verified-email',
             '-system-smtp-accounts-file', '/data/fixtures/accounts.yaml', '-recipient-allowed-domains', 'example.test',
             '-tls', '-tls-cert', '/data/fixtures/cert.pem', '-tls-key', '/data/fixtures/key.pem', '-smtp-auth-require-tls', '-smtps-port', '1465']
    denied = runtime.run('run', '--rm', '--volume', volume + ':/data:ro', image, *flags, check=False)
    assert denied.returncode != 0 and 'not available yet' in denied.stdout + denied.stderr, 'production multiuser gate was bypassed'
    denied = runtime.run('run', '--rm', '--volume', volume + ':/data:ro', image, 'mcp-stdio', '-mode', 'multiuser',
                         '-mail-directory', '/data/imported/mail', check=False)
    assert denied.returncode != 0 and 'unavailable outside standalone' in denied.stdout + denied.stderr, 'multiuser stdio MCP was allowed'
    denied = runtime.run('run', '--rm', '--volume', volume + ':/data:ro', image, 'mcp-stdio',
                         '-mail-directory', '/data/imported/mail', check=False)
    assert denied.returncode != 0 and 'multiuser' in denied.stdout + denied.stderr, 'stdio read a multiuser directory as standalone'
    assert helper(runtime, image, volume, 'test ! -e /data/control/control.sqlite; stat -c "%a:%u:%g" /data/fixtures/key.pem') == '600:1000:1000'
    readonly = runtime.text('run', '--rm', '--volume', volume + ':/data:ro', '--entrypoint', '/bin/sh', image,
                           '-c', 'test -r /data/fixtures/key.pem && test -r /data/fixtures/accounts.yaml; if ( : >> /data/fixtures/key.pem ) 2>/dev/null; then exit 1; fi; if ( : >> /data/fixtures/accounts.yaml ) 2>/dev/null; then exit 1; fi; echo readonly')
    assert readonly == 'readonly', 'mounted credentials must be read-only'


def fixture_request(runtime, container, path, user='alice', body=None, verified=True, expected=200):
    # Test-only synthetic proxy is loopback-only inside the container. Never
    # publish it or the synthetic credentials to the host network.
    args = ['exec', container, 'wget', '--quiet', '--server-response', '--output-document=-',
            '--header=Cookie: fixture-user=' + user + '; fixture-verified=' + ('true' if verified else 'false')]
    if body is not None:
        args += ['--header=Content-Type: application/json', '--post-data=' + json.dumps(body)]
    args += ['http://127.0.0.1:21180' + path]
    response = runtime.run(*args, check=False)
    if expected != 200:
        assert response.returncode != 0 and str(expected) in response.stderr, 'unexpected fixture status: ' + response.stderr
        return None
    if response.returncode:
        raise RuntimeError('fixture request failed: ' + response.stderr)
    return json.loads(response.stdout)


def verify_fixture(runtime, image):
    volume = runtime.volume('fixture')
    initialize_volume(runtime, image, volume, fixture=True)
    container = runtime.start(image, 'fixture', '-state-directory', '/state', volume=volume, fixture=True)
    for _ in range(60):
        response = runtime.run('exec', container, 'wget', '--quiet', '--output-document=-',
                               'http://127.0.0.1:21180/__fixture__/health', check=False)
        if response.returncode == 0:
            break
        time.sleep(0.5)
    else:
        raise RuntimeError('fixture did not become ready: ' + runtime.text('logs', container))
    assert runtime.text('exec', container, 'id', '-u') == '1000'
    for user in ['alice', 'bob']:
        fixture_request(runtime, container, '/__fixture__/deliver', body={
            'Recipient': user + '@example.test', 'Subject': user + ' container private',
            'Body': 'private-' + user, 'Attachment': user + ' attachment'})
    pending = fixture_request(runtime, container, '/api/v1/emails', verified=False)
    assert pending['total'] == 0, 'unverified user claimed pending history'
    contract = fixture_request(runtime, container, '/api/v1/openapi.json')
    assert contract['openapi'].startswith('3.1'), 'fixture did not serve the OpenAPI contract'
    assert '/emails' in contract['paths'] and '/openapi.json' in contract['paths']
    assert '/ws' not in contract['paths'] and '/emails/reload' not in contract['paths'], 'global exits leaked into multiuser contract'
    alice = fixture_request(runtime, container, '/api/v1/emails')
    bob = fixture_request(runtime, container, '/api/v1/emails', user='bob')
    assert alice['total'] == bob['total'] == 1
    alice_id = alice['emails'][0]['id']
    bob_id = bob['emails'][0]['id']
    assert 'private-bob' not in json.dumps(alice) and 'private-alice' not in json.dumps(bob)
    fixture_request(runtime, container, '/api/v1/emails/' + bob_id + '/raw', expected=404)
    fixture_request(runtime, container, '/api/v1/ws', expected=404)
    fixture_request(runtime, container, '/mcp', expected=404)
    fixture_request(runtime, container, '/email', expected=404)
    runtime.text('restart', container)
    for _ in range(60):
        response = runtime.run('exec', container, 'wget', '--quiet', '--output-document=-', 'http://127.0.0.1:21180/__fixture__/health', check=False)
        if response.returncode == 0:
            break
        time.sleep(0.5)
    else:
        raise RuntimeError('fixture restart failed')
    again = fixture_request(runtime, container, '/api/v1/emails')
    assert again['total'] == 1 and again['emails'][0]['id'] == alice_id, 'fixture restart lost bound mail'
    print('PASS test-only fixture: real TLS SMTP, unverified pending isolation, claim, served mode contract, cross-ID/legacy denial and restart', flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--image', required=True)
    parser.add_argument('--fixture-image')
    parser.add_argument('--runtime', choices=['docker', 'podman'], default='docker')
    parser.add_argument('--connection')
    args = parser.parse_args()
    runtime = Runtime(args.runtime, args.connection)
    try:
        runtime.run('info', timeout=20)
        verify_production(runtime, args.image)
        if args.fixture_image:
            verify_fixture(runtime, args.fixture_image)
    finally:
        runtime.close()


if __name__ == '__main__':
    main()
