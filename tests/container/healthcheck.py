#!/usr/bin/env python3
"""Exercise the packaged HEALTHCHECK without relying on host path mounts."""
import argparse
import json
import pathlib
import subprocess
import shlex
import tempfile
import time


def command(*args):
    result = subprocess.run(args, capture_output=True, text=True, timeout=30)
    if result.returncode:
        raise RuntimeError(args[0] + ' failed: ' + result.stderr.strip())
    return result.stdout.strip()


def verify(image, name, flags, fixture, environment=(), expect_healthy=True, health_override=None):
    metadata = json.loads(command('docker', 'image', 'inspect', image))[0]['Config']
    test = metadata.get('Healthcheck', {}).get('Test', [])
    if test and test[0] in ('CMD', 'CMD-SHELL'):
        health_command = test[1] if test[0] == 'CMD-SHELL' else shlex.join(test[1:])
    elif health_override:
        health_command = health_override
    else:
        raise RuntimeError('Image must define an executable healthcheck; OCI-only local runtimes may use --health-command explicitly')
    container = command('docker', 'create', '--name', name, '--health-cmd', health_command, '--health-interval=1s',
                        '--health-start-period=1s', '--health-retries=2', *environment, image, *flags)
    try:
        command('docker', 'cp', str(fixture), container + ':/fixtures')
        command('docker', 'start', container)
        wanted = 'healthy' if expect_healthy else 'unhealthy'
        for _ in range(40):
            state = json.loads(command('docker', 'inspect', container))[0]['State']
            if state.get('Health', {}).get('Status') == wanted:
                break
            if not state.get('Running'):
                raise RuntimeError('Container exited: ' + command('docker', 'logs', container))
            time.sleep(1)
        else:
            raise RuntimeError('Health state did not reach ' + wanted + ': ' + json.dumps(state))
        result = subprocess.run(['docker', 'exec', container, '/app/owlmail', 'healthcheck', '--container'], capture_output=True)
        if (result.returncode == 0) != expect_healthy:
            raise RuntimeError('Manual probe disagrees with expected health status')
        if name in ('owlmail-ci-health-smtp-only', 'owlmail-ci-health-smtp-env'):
            probe = ['docker', 'exec', container, '/app/owlmail', 'healthcheck', '--ready', '--container']
            for _ in range(10):
                if subprocess.run(probe, capture_output=True, timeout=15).returncode == 0:
                    break
                time.sleep(1)
            else:
                raise RuntimeError('Runtime readiness never became ready')
            command('docker', 'exec', container, 'chmod', '500', '/app/mail')
            try:
                for _ in range(10):
                    failed = subprocess.run(probe, capture_output=True, text=True, timeout=15)
                    if failed.returncode == 1:
                        break
                    time.sleep(1)
                else:
                    raise RuntimeError('Read-only storage passed readiness: ' + failed.stdout + failed.stderr)
                command('docker', 'exec', container, '/app/owlmail', 'healthcheck', '--container')
            finally:
                command('docker', 'exec', container, 'chmod', '700', '/app/mail')
            for _ in range(40):
                if subprocess.run(probe, capture_output=True, timeout=15).returncode == 0:
                    break
                time.sleep(1)
            else:
                raise RuntimeError('Runtime readiness did not recover')
            print('PASS:', name, 'readiness tracks storage; liveness remains healthy', flush=True)
        print('PASS:', name, wanted, flush=True)
    finally:
        command('docker', 'rm', '--force', container)


def verify_conflict(image, feature, environment=False):
    """Exercise the production entrypoint, including its nonzero exit status."""
    options = ['-e', 'OWLMAIL_DISABLE_WEB=true'] if environment else []
    flags = [] if environment else ['-disable-web']
    result = subprocess.run(['docker', 'run', '--rm', *options, image, *flags,
                             '-' + feature], capture_output=True, text=True, timeout=30)
    output = result.stdout + result.stderr
    if result.returncode != 1 or 'disable-web cannot be combined' not in output:
        raise RuntimeError('SMTP-only conflict was not rejected: ' + feature + ': ' + output)
    print('PASS: SMTP-only rejects', feature, 'via', 'environment' if environment else 'CLI', flush=True)


def verify_positional_arguments(image):
    """Startup must reject arguments the healthcheck configuration cannot consume."""
    for flags in ([''], ['-disable-web', ''], ['-disable-web', '--', '']):
        result = subprocess.run(['docker', 'run', '--rm', image, *flags],
                                capture_output=True, text=True, timeout=30)
        output = result.stdout + result.stderr
        if result.returncode != 1 or 'does not accept positional arguments' not in output:
            raise RuntimeError('Unconsumed arguments were not rejected: ' + repr(flags) + ': ' + output)
    print('PASS: startup rejects empty positional arguments', flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--image', required=True)
    parser.add_argument('--health-command', help='Local-only override for OCI runtimes that omit image healthcheck metadata; CI must not use it')
    args = parser.parse_args()
    def scenario(name, flags, root, environment=(), expect_healthy=True):
        verify(args.image, name, flags, root, environment, expect_healthy, args.health_command)
    verify_positional_arguments(args.image)
    for feature in ('mcp-enabled', 'metrics-enabled', 'maildev-rest-compat', 'mailcatcher-rest-compat'):
        verify_conflict(args.image, feature)
        verify_conflict(args.image, feature, environment=True)
    with tempfile.TemporaryDirectory(prefix='owlmail-health-ci-') as folder:
        root = pathlib.Path(folder)
        root.chmod(0o755)
        command('openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
                '-subj', '/CN=localhost', '-addext', 'subjectAltName=DNS:localhost',
                '-keyout', str(root / 'key.pem'), '-out', str(root / 'cert.pem'))
        for name in ('key.pem', 'cert.pem'):
            (root / name).chmod(0o644)
        (root / 'config.yaml').write_text('web: 18081\nbase-pathname: /secure\nhttps: true\nhttps-cert: /fixtures/cert.pem\nhttps-key: /fixtures/key.pem\n')
        (root / 'config.yaml').chmod(0o644)
        scenario('owlmail-ci-health-env', [], root,
               ('-e', 'OWLMAIL_WEB_PORT=18082', '-e', 'OWLMAIL_BASE_PATHNAME=/env'))
        scenario('owlmail-ci-health-cli', ['-web', '18080', '-base-pathname', '/mail'], root)
        scenario('owlmail-ci-health-https', ['-config', '/fixtures/config.yaml'], root)
        # Put the empty bind-host value last to catch procfs argv trimming.
        scenario('owlmail-ci-health-smtp-only', ['-disable-web', '-smtp', '18084', '-ip', ''], root)
        scenario('owlmail-ci-health-smtp-env', [], root, ('-e', 'OWLMAIL_DISABLE_WEB=true', '-e', 'OWLMAIL_SMTP_PORT=18085'))
        scenario('owlmail-ci-health-ipv6', ['-web-ip', '::', '-web', '18083'], root)
        # Keep stdin open for a non-server mode; it must never probe a default
        # web listener or report a healthy container.
        scenario('owlmail-ci-health-stdio', ['mcp-stdio'], root, ('-i',), False)


if __name__ == '__main__':
    main()
