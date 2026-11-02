#!/usr/bin/env python3
"""Build-only validation of this lab's cache after interrupted compilers."""
import pathlib
import subprocess
import sys

root = pathlib.Path(sys.argv[1])
if str(root) not in ('/build/lizardfs-build', '/build/ganesha-build'):
    raise SystemExit('refusing to clean any directory outside the two lab build caches')
removed = []
for path in root.rglob('*'):
    if path.suffix not in ('.o', '.a') or not path.is_file():
        continue
    result = subprocess.run(['readelf', '-h', '-S', str(path)], stdout=subprocess.DEVNULL,
                            stderr=subprocess.PIPE, timeout=10)
    if result.returncode or result.stderr:
        path.unlink()
        removed.append(str(path))
        print('Removed incomplete lab cache artifact: ' + str(path), flush=True)
print('Validated lab ELF cache; removed=' + str(len(removed)), flush=True)
