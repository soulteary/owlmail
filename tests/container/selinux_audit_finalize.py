#!/usr/bin/env python3
"""Read-only final verification of an existing audit-resume run with log rotation.

Does not send resume or modify configuration. SIGCONT only requests a temporary
internal diagnostic, preserved-file checks accept normal existing log rotation.
The original execution report is retained, including its overly strict same-
inode assertion; this verifier checks the correct preserved-original condition.
"""
import argparse
import hashlib
import json
import pathlib
import re
import shlex
from multiuser import Runtime
from selinux_audit_resume import snapshot
from selinux_recovery import command


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--connection', default='podman-machine-default')
    parser.add_argument('--machine', default='podman-machine-default')
    parser.add_argument('--run-directory', required=True)
    args = parser.parse_args()
    directory = pathlib.Path(args.run_directory)
    original = directory / 'repair.json'
    report = json.loads(original.read_text())
    result = {'status': 'failed', 'executed_resume_script_sha256': report['script_sha256'],
              'current_resume_script_sha256': hashlib.sha256(pathlib.Path(__file__).with_name('selinux_audit_resume.py').read_bytes()).hexdigest(), 'original_execution_report_sha256': hashlib.sha256(original.read_bytes()).hexdigest(),
              'script_sha256': hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),
              'reason_for_separate_verification': 'Original runner expected current log inode unchanged; configured normal rotation moved original inode to audit.log.1. No second resume is sent.'}
    runtime = Runtime('podman', args.connection)
    try:
        live = snapshot(runtime, args.machine)
        result['live'] = live
        for field in ['pid', 'starttime', 'exe', 'configuration_sha256', 'active_rules_sha256', 'enforcing']:
            assert live[field] == report['before'][field]
        assert 'logging suspended = no' in live['internal_state']
        assert '/var/log/audit/audit.log' in live['fd_targets']
        old_inode, old_size, _ = map(int, report['before']['log_inode_size_mtime'].split())
        old_files = command(runtime, args.machine, 'find /var/log/audit -maxdepth 1 -type f -inum ' + str(old_inode) + ' -exec stat -c "%i %s %n" {} +').splitlines()
        assert len(old_files) == 1
        assert int(old_files[0].split()[1]) >= old_size
        result['original_log_preserved'] = old_files[0]
        result['normal_rotation_observed'] = int(live['log_inode_size_mtime'].split()[0]) != old_inode
        result['verified_events'] = {}
        for kind in ['mcs', 'type']:
            path = directory / (kind + '-evidence.json')
            evidence = json.loads(path.read_text())
            assert hashlib.sha256(path.read_bytes()).hexdigest() == report['verification'][kind]['evidence_sha256']
            assert evidence['status'] == 'passed' and evidence['correlated_avc']
            assert not evidence['remaining_owned_containers'] and not evidence['remaining_owned_volumes']
            line = evidence['correlated_avc'][0]['record']
            serial = re.search(r'msg=audit\([^)]+\)', line).group(0)
            actual = command(runtime, args.machine, 'grep -F ' + shlex.quote(serial) + ' /var/log/audit/audit.log* | grep -F ' + shlex.quote(evidence['object_name']))
            assert actual
            result['verified_events'][kind] = {'run_id': evidence['run_id'], 'actual_persisted_audit_record': actual,
                'evidence_sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
        result['configuration_rules_enforcement_unchanged'] = True
        result['same_original_daemon_identity'] = True
        result['original_log_inode_preserved_without_shortening'] = True
        result['status'] = 'passed'
        print('PASS: same daemon healthy, configuration/rules/enforcement unchanged, original log retained across normal rotation, real MCS and TE AVC persisted')
    finally:
        (directory / 'final-verification.json').write_text(json.dumps(result, indent=2) + '\n')


if __name__ == '__main__':
    main()
