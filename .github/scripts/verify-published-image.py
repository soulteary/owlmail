"""Read public metadata, then run only the child pinned by the approved parent."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import time
import urllib.request

REPOSITORY = 'ghcr.io/soulteary/owlmail'
SOURCE = '37495a02b1300ff31f7455dec38f364e3d123220'
VERSION = '0.11.3'
root = Path('verification-evidence')
root.mkdir(exist_ok=True)
parent = os.environ['RELEASE_INDEX_DIGEST']
arch = os.environ['EXPECTED_ARCH']
assert re.fullmatch(r'sha256:[0-9a-f]{64}', parent)
assert arch in ('amd64', 'arm64')
with urllib.request.urlopen('https://ghcr.io/token?service=ghcr.io&scope=repository:soulteary/owlmail:pull', timeout=30) as response:
    token = json.load(response)['token']
accept = 'application/vnd.oci.image.index.v1+json,application/vnd.docker.distribution.manifest.list.v2+json,application/vnd.oci.image.manifest.v1+json'
def metadata(kind, reference):
    request = urllib.request.Request('https://ghcr.io/v2/soulteary/owlmail/' + kind + '/' + reference,
                                    headers={'Authorization': 'Bearer ' + token, 'Accept': accept})
    with urllib.request.urlopen(request, timeout=60) as response:
        raw = response.read()
    digest = 'sha256:' + hashlib.sha256(raw).hexdigest()
    if reference.startswith('sha256:'):
        assert reference == digest, 'registry response bytes differ from requested digest'
    (root / (kind + '-' + reference.replace(':', '_') + '.json')).write_bytes(raw)
    return json.loads(raw), digest

def command(*args, timeout=120):
    result = subprocess.run(args, text=True, capture_output=True, timeout=timeout)
    with (root / 'commands.log').open('a') as log:
        log.write(json.dumps(list(args)) + '\n' + result.stdout + result.stderr + '\n')
    if result.returncode:
        raise RuntimeError('Command failed: ' + args[0] + ': ' + result.stderr)
    return result.stdout.strip()

index, actual_parent = metadata('manifests', parent)
assert actual_parent == parent
assert index['mediaType'] in ('application/vnd.oci.image.index.v1+json', 'application/vnd.docker.distribution.manifest.list.v2+json')
_, current_tag = metadata('manifests', VERSION)
assert current_tag == parent, 'published version tag differs from approved parent'
children = [row for row in index['manifests'] if row.get('platform', {}).get('os') == 'linux' and row['platform'].get('architecture') == arch]
assert len(children) == 1, 'missing or ambiguous platform child'
child = children[0]['digest']
assert re.fullmatch(r'sha256:[0-9a-f]{64}', child)
manifest, _ = metadata('manifests', child)
assert manifest['mediaType'] in ('application/vnd.oci.image.manifest.v1+json', 'application/vnd.docker.distribution.manifest.v2+json')
config, _ = metadata('blobs', manifest['config']['digest'])
assert config['architecture'] == arch and config['os'] == 'linux'
labels = config['config']['Labels']
assert labels['org.opencontainers.image.revision'] == SOURCE
assert labels['org.opencontainers.image.version'] == VERSION
assert labels['org.opencontainers.image.source'] == 'https://github.com/soulteary/owlmail'
image = REPOSITORY + '@' + child
report = {'parentIndex': parent, 'platformManifest': child, 'platform': 'linux/' + arch, 'source': SOURCE, 'configLabels': labels}
(root / 'report.json').write_text(json.dumps(report, indent=2) + '\n')
command('docker', 'pull', '--platform', 'linux/' + arch, image, timeout=300)
local = json.loads(command('docker', 'image', 'inspect', image))[0]
assert local['Architecture'] == arch and local['Os'] == 'linux'
assert image in local['RepoDigests']
container = None
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
try:
    container = command('docker', 'run', '--detach', '--platform', 'linux/' + arch,
                        '--publish', '127.0.0.1:11025:1025', '--publish', '127.0.0.1:11080:1080', image)
    deadline = time.monotonic() + 45
    while True:
        state = json.loads(command('docker', 'inspect', container))[0]['State']
        assert state['Running'], 'published container exited'
        try:
            with opener.open('http://127.0.0.1:11080/readyz', timeout=2) as response:
                assert response.status == 200
            break
        except OSError:
            assert time.monotonic() < deadline, 'published container readiness timeout'
            time.sleep(0.5)
    with opener.open('http://127.0.0.1:11080/api/v1/version', timeout=5) as response:
        version = json.load(response)
    for key, value in {'version': VERSION, 'commit': SOURCE, 'branch': 'v' + VERSION,
                       'go_version': 'go1.27.3', 'platform': 'linux/' + arch}.items():
        assert version[key] == value, (key, version)
    report['actualVersionAPI'] = version
    environment = dict(os.environ, TEST_SMTP_HOST='127.0.0.1', TEST_SMTP_PORT='11025', TEST_MAIL_API='http://127.0.0.1:11080')
    result = subprocess.run(['python3', 'examples/testing/python/email_test.py'], env=environment, text=True, capture_output=True, timeout=60)
    (root / 'smtp.log').write_text(result.stdout + result.stderr)
    assert result.returncode == 0, 'released SMTP capture/read/delete example failed'
    command('docker', 'exec', container, '/app/owlmail', 'healthcheck', '--container')
    command('docker', 'exec', container, '/app/owlmail', 'healthcheck', '--ready', '--container')
    report['readyVersionSMTPAndManualHealthPassed'] = True
finally:
    if container:
        try:
            command('docker', 'logs', container)
        finally:
            command('docker', 'rm', '--force', container)
    (root / 'report.json').write_text(json.dumps(report, indent=2) + '\n')
command('python3', 'tests/container/healthcheck.py', '--image', image, timeout=600)
report['fullPublishedContainerHealthSuitePassed'] = True
(root / 'report.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps(report, indent=2))
