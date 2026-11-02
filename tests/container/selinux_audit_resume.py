#!/usr/bin/env python3
"""Explicit operator repair for an already suspended existing audit daemon.

Never changes policy, configuration, enforcement or rules. SIGCONT requests
internal diagnostics; SIGUSR2 is the documented resume operation. This restores
intended logging rather than restoring the original broken suspended condition.
Existing audit logs are retained; genuine appended records are never truncated.
"""
import argparse
import hashlib
import json
import pathlib
import re
import subprocess
import sys
from multiuser import Runtime
from selinux_recovery import command, machine


def internal_state(runtime, name, pid):
    # Refuse to overwrite any diagnostic file that did not belong to this run.
    return command(runtime, name, 'test ! -e /run/audit/auditd.state; test "$(cat /proc/' + str(pid) + '/comm)" = auditd; kill -CONT ' + str(pid) + '; for n in 1 2 3 4 5 6 7 8 9 10; do test ! -s /run/audit/auditd.state || break; sleep .2; done; cat /run/audit/auditd.state; rm /run/audit/auditd.state')


def snapshot(runtime, name):
    status = command(runtime, name, 'auditctl -s')
    pid = int(re.search(r'^pid\s+(\d+)$', status, re.MULTILINE).group(1))
    assert command(runtime, name, 'cat /proc/' + str(pid) + '/comm') == 'auditd'
    process_stat = command(runtime, name, 'cat /proc/' + str(pid) + '/stat')
    starttime = process_stat.rsplit(')', 1)[1].split()[19]
    return {'pid': pid, 'starttime': starttime, 'exe': command(runtime, name, 'readlink -f /proc/' + str(pid) + '/exe'), 'status': status,
            'configuration_sha256': command(runtime, name, 'sha256sum /etc/audit/auditd.conf').split()[0],
            'active_rules_sha256': command(runtime, name, 'auditctl -l | sha256sum').split()[0],
            'enforcing': machine(runtime, name, 'getenforce; cat /sys/fs/selinux/enforce').stdout.strip(),
            'log_inode_size_mtime': command(runtime, name, 'stat -c "%i %s %Y" /var/log/audit/audit.log'),
            'fd_targets': command(runtime, name, 'ls -l /proc/' + str(pid) + '/fd'),
            'internal_state': internal_state(runtime, name, pid)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--connection', default='podman-machine-default')
    parser.add_argument('--machine', default='podman-machine-default')
    parser.add_argument('--fixture-image', required=True)
    parser.add_argument('--output-directory', required=True)
    parser.add_argument('--resume-existing-suspended-auditd', action='store_true', required=True,
                        help='Explicit authorization for the existing daemon SIGUSR2 resume, without restart or rule/config edits')
    args = parser.parse_args()
    directory = pathlib.Path(args.output_directory)
    directory.mkdir(parents=True, exist_ok=False)
    runtime = Runtime('podman', args.connection)
    report = {'status': 'failed', 'script_sha256': hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),
              'operation': 'documented SIGUSR2 resume of pre-existing auditd; no restart, config/rule/policy changes',
              'restoration_policy': 'retain recovered logging and genuine append-only audit records; never reintroduce suspension'}
    try:
        report['before'] = snapshot(runtime, args.machine)
        (directory / 'repair.json').write_text(json.dumps(report, indent=2) + '\n')
        assert report['before']['enforcing'].splitlines() == ['Enforcing', '1']
        assert 'logging suspended = yes' in report['before']['internal_state']
        pid = report['before']['pid']
        assert command(runtime, args.machine, 'cat /proc/' + str(pid) + '/stat').rsplit(')', 1)[1].split()[19] == report['before']['starttime']
        assert command(runtime, args.machine, 'readlink -f /proc/' + str(pid) + '/exe') == report['before']['exe']
        command(runtime, args.machine, 'test "$(cat /proc/' + str(pid) + '/comm)" = auditd; kill -USR2 ' + str(pid))
        report['after_resume'] = snapshot(runtime, args.machine)
        assert report['after_resume']['pid'] == pid
        assert 'logging suspended = no' in report['after_resume']['internal_state']
        assert '/var/log/audit/audit.log' in report['after_resume']['fd_targets']
        for field in ['configuration_sha256', 'active_rules_sha256', 'enforcing']:
            assert report['after_resume'][field] == report['before'][field]
        report['verification'] = {}
        for kind in ['mcs', 'type']:
            path = directory / (kind + '-evidence.json')
            with (directory / (kind + '.log')).open('w') as log:
                result = subprocess.run([sys.executable, str(pathlib.Path(__file__).with_name('selinux_avc.py')),
                    '--connection', args.connection, '--machine', args.machine, '--fixture-image', args.fixture_image,
                    '--denial-kind', kind, '--output', str(path)], stdout=log, stderr=subprocess.STDOUT, timeout=180)
            data = json.loads(path.read_text())
            report['verification'][kind] = {'exit_code': result.returncode, 'run_id': data['run_id'],
                'status': data['status'], 'correlated_avc_count': len(data.get('correlated_avc', [])),
                'evidence_sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
            assert result.returncode == 0 and data['status'] == 'passed' and data['correlated_avc']
        report['final'] = snapshot(runtime, args.machine)
        for field in ['pid', 'starttime', 'exe', 'configuration_sha256', 'active_rules_sha256', 'enforcing']:
            assert report['final'][field] == report['before'][field]
        assert 'logging suspended = no' in report['final']['internal_state']
        original_log = list(map(int, report['before']['log_inode_size_mtime'].split()))
        final_log = list(map(int, report['final']['log_inode_size_mtime'].split()))
        preserved = command(runtime, args.machine, 'find /var/log/audit -maxdepth 1 -type f -inum ' + str(original_log[0]) + ' -exec stat -c \"%i %s %n\" {} +').splitlines()
        assert len(preserved) == 1 and int(preserved[0].split()[1]) >= original_log[1], 'original audit inode missing or shortened after daemon rotation'
        report['original_log_preserved'] = preserved[0]
        report['normal_rotation_observed'] = final_log[0] != original_log[0]
        report['audit_log_not_truncated'] = True
        report['status'] = 'passed'
        print('PASS: auditd suspension diagnosed and resumed; new MCS and TE events have strictly correlated real AVC records')
    finally:
        (directory / 'repair.json').write_text(json.dumps(report, indent=2) + '\n')


if __name__ == '__main__':
    main()
