# OwlMail 0.9.0 release notes

OwlMail 0.9.0 focuses on reproducible integration testing and trustworthy
documentation for developers, CI pipelines, and coding agents. It turns the
JavaScript, Python, and Go examples into continuously exercised workflows,
strengthens cleanup behavior, and makes release metadata auditable from one
version source.

The 0.9.0 release is dated 2026-09-03. The commands below use the `v0.9.0` tag
and `0.9.0` container image; run them after the tag and artifacts are available.

## Highlights

### Runnable examples

The dependency-free JavaScript and Python examples and the Go example now run
end to end against an OwlMail server built from the same commit. The workflow
sends SMTP mail, waits for it through the REST API, verifies the message, and
removes it again. Manual dispatch remains available, while push and pull-request
triggers are limited to Go sources, module files, the examples, and the workflow
itself.

Failure handling is now explicit across all three languages. JavaScript retains
the original verification failure when cleanup also fails, rejects cleanup
redirects, normalizes repeated trailing slashes, and does not buffer response
bodies. Python cleans up after failed assertions and has focused failure-path
tests. Go supports IPv6 SMTP hosts, reports cleanup errors, drains bounded
non-success responses, and can recover the message ID after SMTP acceptance.

### Documentation contracts

Documentation tests now derive the current version, tag, image, release-note
paths, and examples from the root `VERSION` file. They validate executable shell
command structure instead of only looking for strings, cover every registered
machine-facing API route, check concrete MCP HTTP methods and prompt arguments,
and reject unpinned GitHub Actions references.

The seven root READMEs and the English and Chinese documentation indexes position
OwlMail as an AI-native integration-testing gateway. They continue to state the
security boundary clearly: MCP is optional, disabled by default, bounded,
read-only, and not required by the core SMTP or Web server.

### Release consistency

`VERSION` is the single source for the current documented stable release. The
release workflow checks the requested tag against it after checkout and refuses
to publish mismatched artifacts. Current release images, source installs, API
examples, issue templates, release-note links, and operator commands are updated
together.

The Go Report Card badge workflow also avoids triggering the full CI and image
build workflows for generated badge-only pushes while retaining complete pull
request validation.

## Upgrade notes

- There are no runtime API, SMTP protocol, configuration, or storage-format
  changes from 0.8.0; existing deployments can upgrade directly.
- MCP remains disabled by default and read-only when enabled.
- The MailDev and MailCatcher compatibility facades remain opt-in.
- CI configurations should pin `0.9.0` or a recorded manifest digest rather than
  moving `main` or `latest` tags.

## Included pull requests

- [#109](https://github.com/soulteary/owlmail/pull/109) AI-native README positioning
- [#110](https://github.com/soulteary/owlmail/pull/110) complete testing and AI-first documentation
- [#111](https://github.com/soulteary/owlmail/pull/111) release and integration guidance fixes
- [#112](https://github.com/soulteary/owlmail/pull/112) Go cleanup error reporting
- [#113](https://github.com/soulteary/owlmail/pull/113) IPv6 SMTP hosts in the Go example
- [#114](https://github.com/soulteary/owlmail/pull/114) complete MCP prompt arguments
- [#115](https://github.com/soulteary/owlmail/pull/115) Python failure cleanup
- [#116](https://github.com/soulteary/owlmail/pull/116) semantic documentation tests
- [#117](https://github.com/soulteary/owlmail/pull/117) badge-only build suppression
- [#118](https://github.com/soulteary/owlmail/pull/118) JavaScript error preservation
- [#119](https://github.com/soulteary/owlmail/pull/119) reliable integration cleanup fallback
- [#120](https://github.com/soulteary/owlmail/pull/120) complete machine-facing route coverage
- [#121](https://github.com/soulteary/owlmail/pull/121) badge-only CI suppression
- [#122](https://github.com/soulteary/owlmail/pull/122) live end-to-end example CI
- [#123](https://github.com/soulteary/owlmail/pull/123) concrete MCP HTTP methods
- [#124](https://github.com/soulteary/owlmail/pull/124) release metadata and CI scope alignment

## Install

```bash
docker pull ghcr.io/soulteary/owlmail:0.9.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.9.0
```

For repeatable deployment, record the published manifest digest and use
`ghcr.io/soulteary/owlmail@sha256:<digest>`.

## Release artifacts

- `checksums.txt`
- `checksums.txt.sigstore.json`
- `owlmail-linux-amd64` and `owlmail-linux-amd64.spdx.json`
- `owlmail-linux-arm64` and `owlmail-linux-arm64.spdx.json`
- `owlmail-darwin-amd64` and `owlmail-darwin-amd64.spdx.json`
- `owlmail-darwin-arm64` and `owlmail-darwin-arm64.spdx.json`
- `owlmail-windows-amd64.exe` and
  `owlmail-windows-amd64.exe.spdx.json`

Linux amd64 download, verification, and launch example:

```bash
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.9.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.9.0/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.9.0/checksums.txt.sigstore.json
grep ' owlmail-linux-amd64$' checksums.txt | sha256sum -c -
gh attestation verify owlmail-linux-amd64 --repo soulteary/owlmail
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64
```

```bash
cosign verify \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/soulteary/owlmail:0.9.0
```

## Known limitations

- MCP remains read-only and does not delete, mark, or relay messages.
- The MailCatcher facade does not implement MailCatcher's WebSocket event bus.
- Relay recovery is at-least-once rather than exactly-once.
- The exact GHCR `0.9.0` tag is immutable after publication; use a patch
  release instead of deleting and reusing published artifacts.
