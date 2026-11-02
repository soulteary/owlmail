#!/usr/bin/env python3
"""Strict, read-only audit collection for a new enforcing SELinux denial.

Default MCS and optional user_home_t type probes are reported separately.
Uses an existing Podman machine; changes only labels on this run's own probe
file. Never changes audit rules, dontaudit, enforcement, or system policy.
Missing observable correlated AVC is unsupported/pending (exit 2), not a pass.
"""
import argparse
import hashlib
import json
import pathlib
import re
import shlex
import time
from recovery import RecoveryRuntime
from multiuser import initialize_volume
from selinux_recovery import command, create, inspect, label, machine, mountpoint


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--connection', default='podman-machine-default')
    parser.add_argument('--machine', default='podman-machine-default')
    parser.add_argument('--fixture-image', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--denial-kind', choices=['mcs', 'type'], default='mcs', help='type uses only this owned file labeled user_home_t; never changes policy')
    args = parser.parse_args()
    runtime = RecoveryRuntime('podman', args.connection)
    evidence = {'status': 'unsupported', 'run_id': runtime.run_id, 'scope': 'existing enforcing Podman Linux VM; newly generated event only', 'denial_kind': args.denial_kind, 'mcs_avc_verified': False, 'audit_acceptance_passed': False, 'script_sha256': hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest()}
    output = pathlib.Path(args.output)
    try:
        evidence['enforcement_before'] = machine(runtime, args.machine, 'getenforce; cat /sys/fs/selinux/enforce', check=False).stdout.strip()
        evidence['runtime_security'] = json.loads(runtime.text('info', '--format', 'json'))['host']['security']
        probes = {
            'audit_status': 'sudo auditctl -s',
            'audit_service': 'systemctl is-active auditd',
            'audit_file': 'sudo stat -c "%i %s %Y" /var/log/audit/audit.log',
            'tools': 'for x in auditctl auditd ausearch journalctl dmesg; do command -v "$x" || true; done',
            'kernel_journal': 'sudo journalctl -k -n 0 --no-pager',
            'kernel_ring': 'sudo dmesg >/dev/null',
            'audit_configuration': "sudo grep -E '^[[:space:]]*(log_file|write_logs|log_format|flush|freq|space_left_action|admin_space_left_action|disk_full_action|disk_error_action)[[:space:]]*=' /etc/audit/auditd.conf",
            'audit_service_journal': 'sudo journalctl -u auditd -n 12 --no-pager',
        }
        evidence['capabilities'] = {}
        for name, script in probes.items():
            probe = machine(runtime, args.machine, script, check=False)
            evidence['capabilities'][name] = {'returncode': probe.returncode, 'stdout': probe.stdout.strip(), 'stderr': probe.stderr.strip()}
        audit_pid = re.search(r'^pid\s+(\d+)$', evidence['capabilities']['audit_status']['stdout'], re.MULTILINE)
        if audit_pid:
            fd_probe = machine(runtime, args.machine, "sudo find /proc/" + audit_pid.group(1) + "/fd -maxdepth 1 -type l -lname '*/audit.log*' -exec stat -Lc '%n|%i|%s' {} +", check=False)
            evidence['capabilities']['audit_log_open_fds'] = {'returncode': fd_probe.returncode, 'stdout': fd_probe.stdout.strip(), 'stderr': fd_probe.stderr.strip()}
        if evidence['enforcement_before'].splitlines() != ['Enforcing', '1'] or not evidence['runtime_security'].get('selinuxEnabled'):
            evidence['reason'] = 'Actual enforcing SELinux and enabled container labeling required.'
            print('UNSUPPORTED: ' + evidence['reason'])
            raise SystemExit(2)
        evidence['status'] = 'failed'
        volume = runtime.volume('avc-target')
        initialize_volume(runtime, args.fixture_image, volume, fixture=True)
        target = create(runtime, args.fixture_image, volume, 'avc-target', 'Z')
        donor_volume = runtime.volume('avc-donor')
        initialize_volume(runtime, args.fixture_image, donor_volume, fixture=True)
        donor = create(runtime, args.fixture_image, donor_volume, 'avc-donor', 'Z')
        evidence['target'], evidence['donor'] = inspect(runtime, target), inspect(runtime, donor)
        assert evidence['target']['mount_label'] != evidence['donor']['mount_label']
        root = mountpoint(runtime, volume)
        name = 'avc-probe-' + runtime.run_id
        path = root + '/' + name
        runtime.text('exec', target, '/bin/sh', '-c', 'umask 077; echo synthetic-avc-probe >/state/' + name)
        evidence['object_dac'] = command(runtime, args.machine, 'stat -c %a:%u:%g ' + shlex.quote(path))
        evidence['object_inode'] = int(command(runtime, args.machine, 'stat -c %i ' + shlex.quote(path)))
        evidence['object_name'] = name
        evidence['correct_object_label'] = label(runtime, args.machine, path)
        # Positive control: same UID/GID and DAC can read before changing only MCS.
        assert runtime.text('exec', target, 'cat', '/state/' + name) == 'synthetic-avc-probe'
        wrong_context = evidence['donor']['mount_label'] if args.denial_kind == 'mcs' else 'system_u:object_r:user_home_t:s0'
        command(runtime, args.machine, 'chcon ' + shlex.quote(wrong_context) + ' ' + shlex.quote(path))
        evidence['wrong_object_label'] = label(runtime, args.machine, path)
        assert command(runtime, args.machine, 'stat -c %a:%u:%g ' + shlex.quote(path)) == evidence['object_dac']
        runtime.text('exec', '--detach', target, '/bin/sh', '-c',
                     'echo $$ >/state/probe.pid; while test ! -e /state/trigger; do sleep .1; done; exec cat /state/' + name + ' >/state/probe.stdout 2>/state/probe.stderr')
        deadline = time.monotonic() + 5
        while True:
            pid = runtime.run('exec', target, 'cat', '/state/probe.pid', check=False)
            if pid.returncode == 0:
                container_pid = int(pid.stdout.strip())
                break
            if time.monotonic() > deadline:
                raise RuntimeError('denial process startup deadline')
            time.sleep(.1)
        processes = runtime.text('top', target, 'hpid', 'pid')
        host_pid = next(int(line.split()[0]) for line in processes.splitlines()[1:] if int(line.split()[1]) == container_pid)
        evidence['denial_process'] = {'container_pid': container_pid, 'vm_host_pid': host_pid,
            'uid': runtime.text('exec', target, 'id', '-u'), 'gid': runtime.text('exec', target, 'id', '-g'),
            'actual_context': runtime.text('exec', target, 'cat', '/proc/' + str(container_pid) + '/attr/current').rstrip('\x00')}
        assert evidence['denial_process']['uid'] == evidence['denial_process']['gid'] == '1000'
        audit_stat = evidence['capabilities']['audit_file']
        initial_inode, initial_size = (list(map(int, audit_stat['stdout'].split()[:2])) if audit_stat['returncode'] == 0 else (None, None))
        evidence['window_start_epoch'] = int(command(runtime, args.machine, 'date +%s'))
        runtime.text('exec', target, 'touch', '/state/trigger')
        deadline = time.monotonic() + 5
        while runtime.run('exec', target, '/bin/sh', '-c', 'kill -0 ' + str(container_pid), check=False).returncode == 0:
            if time.monotonic() > deadline:
                raise RuntimeError('denial process exit deadline')
            time.sleep(.1)
        evidence['denial_stderr'] = runtime.text('exec', target, 'cat', '/state/probe.stderr')
        assert name in evidence['denial_stderr'] and 'Permission denied' in evidence['denial_stderr']
        records = {}
        # Wait for existing auditd flush; no auditctl writes or auditd signals.
        for attempt in range(5):
            filters = 'grep -E ' + shlex.quote('pid=' + str(host_pid) + '([[:space:]]|$)|name="' + name + '"')
            scripts = {
                'ausearch': 'ausearch -m AVC,USER_AVC -p ' + str(host_pid) + ' -ts recent --raw 2>/dev/null | ' + filters + ' || true',
                'journal': 'journalctl -k --since "-2 minutes" --no-pager 2>/dev/null | ' + filters + ' || true',
                'dmesg': 'dmesg 2>/dev/null | ' + filters + ' || true',
            }
            if initial_inode is not None:
                current = command(runtime, args.machine, 'stat -c "%i %s" /var/log/audit/audit.log').split()
                evidence['audit_file_after'] = {'inode': int(current[0]), 'size': int(current[1])}
                if int(current[0]) == initial_inode and int(current[1]) >= initial_size:
                    scripts['audit_file_increment'] = 'tail -c +' + str(initial_size + 1) + ' /var/log/audit/audit.log | ' + filters + ' || true'
            for channel, script in scripts.items():
                text = command(runtime, args.machine, script)
                if text:
                    records[channel] = text.splitlines()
            if records:
                break
            if attempt != 4:
                time.sleep(1)
        evidence['window_end_epoch'] = int(command(runtime, args.machine, 'date +%s'))
        evidence['observed_records'] = records
        correlated = []
        for channel, lines in records.items():
            for line in lines:
                stamp = re.search(r'audit\((\d+)(?:\.\d+)?:\d+\)', line)
                if ('avc:' in line.lower() and 'denied' in line.lower() and
                    re.search(r'\bpid=' + str(host_pid) + r'\b', line) and name in line and
                    re.search(r'\bino=' + str(evidence['object_inode']) + r'\b', line) and
                    evidence['denial_process']['actual_context'] in line and evidence['wrong_object_label'] in line and
                    stamp and evidence['window_start_epoch'] <= int(stamp.group(1)) <= evidence['window_end_epoch']):
                    correlated.append({'channel': channel, 'record': line})
        evidence['correlated_avc'] = correlated
        # Restore this own file's original local label and demonstrate unchanged
        # DAC can read again, irrespective of whether audit records are exposed.
        command(runtime, args.machine, 'chcon ' + shlex.quote(evidence['correct_object_label']) + ' ' + shlex.quote(path))
        assert runtime.text('exec', target, 'cat', '/state/' + name) == 'synthetic-avc-probe'
        evidence['positive_after_local_relabel'] = True
        evidence['enforcement_after'] = machine(runtime, args.machine, 'getenforce; cat /sys/fs/selinux/enforce').stdout.strip()
        assert evidence['enforcement_after'].splitlines() == ['Enforcing', '1']
        if not correlated:
            evidence['status'] = 'unsupported'
            evidence['reason'] = 'Real new ' + args.denial_kind + ' denial reproduced, but existing audit channels exposed no AVC matching this PID, object, contexts and time window. Policy dontaudit or audit routing may suppress records; not changed or proven here.'
            print('UNSUPPORTED: ' + evidence['reason'])
            raise SystemExit(2)
        evidence['status'] = 'passed'
        evidence['audit_acceptance_passed'] = True
        evidence['mcs_avc_verified'] = args.denial_kind == 'mcs'
        evidence['verified_scope'] = args.denial_kind + '-denial AVC correlation; no other denial kind implied'
        print('PASS: new enforcing ' + args.denial_kind + ' denial has AVC correlated by host PID, unique object/inode, process/object contexts and event time')
    finally:
        runtime.close()
        evidence['remaining_owned_containers'] = len(json.loads(runtime.text('ps', '--all', '--filter', 'label=io.owlmail.verification-run=' + runtime.run_id, '--format', 'json')))
        evidence['remaining_owned_volumes'] = len(json.loads(runtime.text('volume', 'ls', '--filter', 'label=io.owlmail.verification-run=' + runtime.run_id, '--format', 'json')))
        output.write_text(json.dumps(evidence, indent=2) + '\n')
        assert not evidence['remaining_owned_containers'] and not evidence['remaining_owned_volumes']


if __name__ == '__main__':
    main()
