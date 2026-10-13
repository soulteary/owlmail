# Offline configuration and recovery diagnostics

`doctor` is available in the current `main` source, after the original
`v0.10.1` release. Build that source before following this guide.

Run `owlmail doctor` with the same flags, environment variables, and optional `-config` YAML/JSON file used for normal startup. Resolution retains the standard priority: command line, compatibility environment, OwlMail environment, configuration file defaults, then built-in defaults.

```sh
owlmail doctor -config /etc/owlmail/config.yaml
owlmail doctor -mail-directory /data/owlmail -web-user developer -web-password "$OWLMAIL_WEB_PASSWORD"
```

The command writes one JSON report with `ready`, `scope`, `checks`, and `relay_recovery`. Exit 0 means the offline checks passed; exit 1 means parsing or a check failed. Warning entries do not fail the command. Reports use fixed diagnostic messages and omit configuration values, credentials, URLs, and raw parser errors.

Checks include the normal startup value contract, complete Web authentication credentials, non-overlapping enabled listener addresses and ports, readable matching TLS/HTTPS key pairs and leaf certificate validity dates, compiled webhook configuration, Redis URL syntax, bounded relay-rule JSON, mail/index namespaces and existing file types, and durable relay recovery records. A missing mail directory or index parent is a warning because normal startup may create it. Automatic relay without an outgoing host fails diagnosis.

Recovery inventory uses the normal relay reader, validates the single-JSON-value contract, and counts queued/completed records, missing or unsafe source EML files, and duplicate pending sources. Expired completed records are excluded from the normal recovery capacity limit but remain in the inventory. It never removes expired jobs, repairs records, sends messages, acquires source leases, creates directories, or opens the SQLite index. Use a quiescent spool or a filesystem snapshot for consistent results; live writers may change files during inspection.

Doctor is offline. It does not bind listeners, connect to SMTP/Redis/S3, start health monitors, verify certificate trust/hostnames, or prove storage write access. A passing report is a local preflight result, not service readiness. Normal startup and the existing health endpoints still verify runtime readiness and recover durable delivery work.
