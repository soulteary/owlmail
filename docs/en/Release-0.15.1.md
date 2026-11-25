# owlmail v0.15.1

Release date: 2026-11-26 (Asia/Shanghai).

Refresh SMTP, SQLite, S3/Redis and toolchain dependencies already on main, and include HTTP/SMTP listener error reporting and lifecycle regression coverage. Standalone remains supported; normal binaries retain the existing multiuser startup gate. No new production multiuser acceptance is claimed.

## Kit dependencies

- `github.com/soulteary/health-kit/v4`: `v4.0.9` → `v4.0.10`.
- `github.com/soulteary/logger-kit/v3`: `v3.0.7` → `v3.0.8`.
- `github.com/soulteary/version-kit/v4`: `v4.0.6` → `v4.0.7`.

## Upgrade and rollback

Retain the previous exact program/image and back up configuration before rollout. Stop affected writers during replacement; validate the actual configured user path in staging. Restore the complete previous dependency lockfiles when rebuilding an older source version.

Full changes: https://github.com/soulteary/owlmail/compare/v0.15.0...v0.15.1
