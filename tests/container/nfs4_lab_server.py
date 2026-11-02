#!/usr/bin/env python3
"""Container-only native LizardFS/RichACL NFSv4.1 lab bootstrap.

Not a production service or mode gate. All persistent master/chunk/NFS recovery
state resides under /state. The orchestrator must isolate its container/network
and stop clients before SIGTERM; forced termination never earns a stopped marker.
"""
import grp
import hashlib
import ipaddress
import json
import os
import pathlib
import pwd
import signal
import socket
import subprocess
import time

ROOT = pathlib.Path('/state')
STOP = False


def atomic_json(path, value):
    temporary = path.with_name(path.name + '.tmp')
    with temporary.open('w') as out:
        json.dump(value, out, indent=2)
        out.write('\n')
        out.flush()
        os.fsync(out.fileno())
    temporary.replace(path)
    descriptor = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def signal_stop(_number, _frame):
    global STOP
    STOP = True


def identities():
    for name, uid in [('nfsalice', 1001), ('nfsbob', 1002)]:
        try:
            account = pwd.getpwnam(name)
        except KeyError:
            try:
                pwd.getpwuid(uid)
            except KeyError:
                with pathlib.Path('/etc/passwd').open('a') as out:
                    out.write(f'{name}:x:{uid}:{uid}:Synthetic NFS lab:/nonexistent:/usr/sbin/nologin\n')
            else:
                raise RuntimeError('synthetic UID already used by another account')
        else:
            if account.pw_uid != uid or account.pw_gid != uid:
                raise RuntimeError('synthetic account mapping conflict')
        try:
            group = grp.getgrnam(name)
        except KeyError:
            try:
                grp.getgrgid(uid)
            except KeyError:
                with pathlib.Path('/etc/group').open('a') as out:
                    out.write(f'{name}:x:{uid}:\n')
            else:
                raise RuntimeError('synthetic GID already used by another group')
        else:
            if group.gr_gid != uid:
                raise RuntimeError('synthetic group mapping conflict')
    pathlib.Path('/etc/idmapd.conf').write_text('[General]\nDomain = lab.test\n[Mapping]\nNobody-User = nobody\nNobody-Group = nogroup\n[Translation]\nMethod = nsswitch\n')


def configure():
    if os.geteuid() != 0 or ROOT.is_symlink():
        raise RuntimeError('container root and a real /state directory required')
    ROOT.mkdir(exist_ok=True)
    marker = ROOT / '.owlmail-nfs4-lab'
    if marker.exists():
        if marker.read_text() != 'native-lizardfs-lab-v1\n':
            raise RuntimeError('foreign state marker')
    elif any(ROOT.iterdir()):
        raise RuntimeError('refuse nonempty unmarked state')
    else:
        marker.write_text('native-lizardfs-lab-v1\n')
    for name in ['master', 'chunkmeta', 'chunks', 'config', 'logs', 'ganesha-recovery']:
        directory = ROOT / name
        if directory.is_symlink():
            raise RuntimeError('persistent lab paths must not be symlinks')
        directory.mkdir(exist_ok=True)
    for name in ['lab-ready.json', 'lab-stopped.json']:
        (ROOT / name).unlink(missing_ok=True)
    metadata = ROOT / 'master' / 'metadata.mfs'
    if not metadata.exists():
        # Official pinned src/data/metadata.mfs seed. Never reseed a restored
        # master that already has metadata/changelog files but lost metadata.
        if any((ROOT / 'master').iterdir()):
            raise RuntimeError('missing metadata in nonempty restored master')
        with metadata.open('xb') as out:
            out.write(b'MFSM NEW')
            out.flush()
            os.fsync(out.fileno())
    identities()
    candidates = socket.getaddrinfo(socket.gethostname(), None, socket.AF_INET, socket.SOCK_STREAM)
    native_ip = next((entry[4][0] for entry in candidates
                      if not ipaddress.ip_address(entry[4][0]).is_loopback
                      and not ipaddress.ip_address(entry[4][0]).is_unspecified), None)
    if native_ip is None:
        raise RuntimeError('private container IPv4 required for native LizardFS backend')
    files = {
        'master.cfg': '''WORKING_USER = root
WORKING_GROUP = root
NICE_LEVEL = 0
DATA_PATH = /state/master
EXPORTS_FILENAME = /state/config/exports.cfg
PERSONALITY = master
AUTO_RECOVERY = 0
MATOML_LISTEN_HOST = 127.0.0.1
MATOML_LISTEN_PORT = 9419
MATOCS_LISTEN_HOST = 127.0.0.1
MATOCS_LISTEN_PORT = 9420
MATOCL_LISTEN_HOST = 127.0.0.1
MATOCL_LISTEN_PORT = 9421
''',
        # Permission checking stays enabled. Only this same container's FSAL
        # may connect to native master; no mapall or nomasterpermcheck bypass.
        'exports.cfg': '127.0.0.1 / rw,alldirs,maproot=0:0\n',
        'chunk.cfg': '''WORKING_USER = root
WORKING_GROUP = root
NICE_LEVEL = 0
DATA_PATH = /state/chunkmeta
MASTER_HOST = 127.0.0.1
MASTER_PORT = 9420
BIND_HOST = 127.0.0.1
MASTER_RECONNECTION_DELAY = 1
CSSERV_LISTEN_HOST = 127.0.0.1
CSSERV_LISTEN_PORT = 9422
HDD_CONF_FILENAME = /state/config/hdd.cfg
HDD_LEAVE_SPACE_DEFAULT = 64MiB
''',
        'hdd.cfg': '/state/chunks\n',
        'ganesha.conf': '''NFS_Core_Param {
    Protocols = 4;
    NFS_Port = 2049;
    Bind_Addr = 0.0.0.0;
    Enable_NLM = false;
    Enable_RQUOTA = false;
    Plugins_Dir = /usr/local/lib/ganesha;
}
NFSv4 {
    Minor_Versions = 1;
    DomainName = lab.test;
    IdmapConf = /etc/idmapd.conf;
    UseGetpwnam = true;
    Allow_Numeric_Owners = true;
    Only_Numeric_Owners = true;
    Graceless = true;
    RecoveryBackend = fs;
    RecoveryRoot = /state/ganesha-recovery;
    RecoveryDir = v4recov;
    RecoveryOldDir = v4old;
}
LizardFS {
    PNFS_MDS = false;
    PNFS_DS = false;
}
EXPORT {
    Export_Id = 77;
    Path = /;
    Pseudo = /lab;
    Access_Type = RW;
    Squash = No_Root_Squash;
    Protocols = 4;
    Transports = TCP;
    SecType = sys;
    FSAL {
        Name = LizardFS;
        hostname = 127.0.0.1;
        port = 9421;
        mountpoint = owlmail-native-acl-lab;
        subfolder = /;
    }
}
LOG { Default_Log_Level = EVENT; }
''',
    }
    # LizardFS rejects a localhost master. Use this container's own address,
    # with no backend port publishing and an exact native export client address.
    files = {name: value.replace('127.0.0.1', native_ip) for name, value in files.items()}
    for name, value in files.items():
        (ROOT / 'config' / name).write_text(value)
    return native_ip, {name: hashlib.sha256(value.encode()).hexdigest() for name, value in files.items()}


def wait_port(processes, port, host, timeout=25):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline and not STOP:
        for name, process in processes.items():
            if process.poll() is not None:
                raise RuntimeError(name + ' exited during startup: ' + str(process.returncode))
        try:
            with socket.create_connection((host, port), timeout=.3):
                return
        except OSError:
            time.sleep(.1)
    raise RuntimeError('startup interrupted or readiness timeout on port ' + str(port))



def wait_export_ready(processes, log_path, start_offset, timeout=25):
    # Restored state includes old source logs. Only inspect this process's suffix.
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline and not STOP:
        for name, process in processes.items():
            if process.poll() is not None:
                raise RuntimeError(name + ' exited before export readiness')
        if log_path.exists():
            with log_path.open('rb') as stream:
                if os.fstat(stream.fileno()).st_size < start_offset:
                    raise RuntimeError('Ganesha log truncated during readiness')
                stream.seek(start_offset)
                fresh = stream.read().decode('utf-8', errors='replace')
            if any(text in fresh for text in ('Unable to mount LizardFS cluster',
                    'No export entries found', 'Could not create export for')):
                raise RuntimeError('Ganesha failed to initialize the native export')
            if ('OWLMAIL_NATIVE_EXPORT_READY LizardFS module export /.' in fresh
                    and 'NFS SERVER INITIALIZED' in fresh):
                return
        time.sleep(.1)
    raise RuntimeError('native export readiness interrupted or timed out')


def probe_native_root(native_ip):
    # A single bounded attempt, not a retry that could hide initialization errors.
    path = ROOT / 'logs' / 'native-readiness.log'
    with path.open('a') as log:
        result = subprocess.run(['/usr/local/bin/owlmail-native-probe', native_ip],
            stdout=log, stderr=subprocess.STDOUT, timeout=20, check=False)
    if result.returncode != 0:
        raise RuntimeError('native root GETATTR readiness failed: ' + str(result.returncode))


def terminate(name, process, timeout):
    forced = False
    if process.poll() is None:
        try:
            os.killpg(process.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        try:
            process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            forced = True
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait(timeout=2)
    return {'name': name, 'exit_code': process.returncode, 'forced': forced}


def main():
    signal.signal(signal.SIGTERM, signal_stop)
    signal.signal(signal.SIGINT, signal_stop)
    processes = {}
    logs = []
    success = False
    try:
        native_ip, hashes = configure()
        commands = {
            'master': ['/usr/local/sbin/mfsmaster', '-d', '-c', '/state/config/master.cfg', 'start'],
            'chunk': ['/usr/local/sbin/mfschunkserver', '-d', '-c', '/state/config/chunk.cfg', 'start'],
            'ganesha': ['/usr/local/bin/ganesha.nfsd', '-F', '-f', '/state/config/ganesha.conf', '-L', '/state/logs/ganesha.log', '-p', '/state/config/ganesha.pid'],
        }
        for name, port in [('master', 9421), ('chunk', 9422), ('ganesha', 2049)]:
            if name == 'ganesha':
                probe_native_root(native_ip)
                ganesha_log = ROOT / 'logs' / 'ganesha.log'
                ganesha_offset = ganesha_log.stat().st_size if ganesha_log.exists() else 0
            log = (ROOT / 'logs' / (name + '-console.log')).open('a')
            logs.append(log)
            processes[name] = subprocess.Popen(commands[name], stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
            wait_port(processes, port, native_ip if name != 'ganesha' else '127.0.0.1')
        wait_export_ready(processes, ganesha_log, ganesha_offset)
        time.sleep(.5)
        if any(process.poll() is not None for process in processes.values()):
            raise RuntimeError('a native service exited after socket readiness')
        atomic_json(ROOT / 'lab-ready.json', {'nativeFSAL': 'LizardFS', 'daemonAndPlugin': 'matching pinned Ganesha V4.3 source', 'ganeshaExecutable': '/usr/local/bin/ganesha.nfsd', 'pnfs': False, 'protocol': 'NFSv4.1',
            'domain': 'lab.test', 'pids': {name: process.pid for name, process in processes.items()},
            'ports': {'ganesha': 2049, 'master': 9421, 'chunk': 9422}, 'nativeBackendAddress': native_ip, 'configSHA256': hashes, 'nativeRootGETATTR': True, 'freshExportReady': True,
            'scope': 'numeric AUTH_SYS identity and native ACL lab; Only_Numeric_Owners and Graceless; named-domain, Kerberos and session recovery unverified'})
        print('native LizardFS/RichACL NFSv4.1 lab ready', flush=True)
        while not STOP:
            for name, process in processes.items():
                if process.poll() is not None:
                    raise RuntimeError(name + ' exited unexpectedly: ' + str(process.returncode))
            time.sleep(.1)
        success = True
    finally:
        if processes:
            (ROOT / 'lab-ready.json').unlink(missing_ok=True)
        results = []
        for name, timeout in [('ganesha', 6), ('chunk', 5), ('master', 12)]:
            if name in processes:
                results.append(terminate(name, processes[name], timeout))
        for log in logs:
            log.close()
        graceful = success and len(results) == 3 and all(item['exit_code'] == 0 and not item['forced'] for item in results)
        if graceful:
            atomic_json(ROOT / 'lab-stopped.json', {'graceful': True, 'services': results,
                'metadataBytes': (ROOT / 'master' / 'metadata.mfs').stat().st_size})
        else:
            raise RuntimeError('native lab not safely stopped: ' + json.dumps(results))


if __name__ == '__main__':
    main()
