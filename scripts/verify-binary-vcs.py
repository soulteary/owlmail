#!/usr/bin/env python3
"""Reject binaries with missing, dirty, or unexpected source provenance."""

import subprocess
import sys


def main():
    if len(sys.argv) < 2:
        raise SystemExit("Usage: verify-binary-vcs.py BINARY [BINARY ...]")
    revision = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    expected = {"vcs": "git", "vcs.revision": revision, "vcs.modified": "false"}
    for binary in sys.argv[1:]:
        info = subprocess.check_output(["go", "version", "-m", binary], text=True)
        settings = {}
        for line in info.splitlines():
            fields = line.split(None, 1)
            if len(fields) == 2 and fields[0] == "build" and "=" in fields[1]:
                key, value = fields[1].split("=", 1)
                settings[key] = value
        for key, value in expected.items():
            if settings.get(key) != value:
                raise SystemExit(
                    f"{binary}: expected {key}={value}, got {settings.get(key)!r}"
                )
        print(f"{binary}: clean VCS metadata at {revision}")


if __name__ == "__main__":
    main()
