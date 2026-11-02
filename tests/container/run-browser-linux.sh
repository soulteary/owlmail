#!/bin/sh
# Test-only Linux headed browser runner; preserve the caller's working directory.
set -eu
owlmail_runner_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
exec node "$owlmail_runner_dir/run-browser-linux.mjs" "$@"
