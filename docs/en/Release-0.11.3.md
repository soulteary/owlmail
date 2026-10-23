# OwlMail 0.11.3

Released: 2026-10-23

This patch releases the merged `github.com/getkin/kin-openapi` update from 0.149.0 to 0.151.0 and the CodeQL Action update to 4.39.0. It retains Go 1.27.3. No application API, configuration, or storage format changes are introduced.

Back up the complete mail directory before deployment. Existing security settings remain unchanged; review [0.11.2](Release-0.11.2.md) for the previous release.

## Install

```bash
docker pull ghcr.io/soulteary/owlmail:0.11.3
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.11.3
```

For repeatable deployment, record the published manifest digest and use
`ghcr.io/soulteary/owlmail@sha256:<digest>`.

## Release artifacts

- `checksums.txt` and `checksums.txt.sigstore.json`
- `owlmail-linux-amd64` and `owlmail-linux-amd64.spdx.json`
- `owlmail-linux-arm64` and `owlmail-linux-arm64.spdx.json`
- `owlmail-darwin-amd64` and `owlmail-darwin-amd64.spdx.json`
- `owlmail-darwin-arm64` and `owlmail-darwin-arm64.spdx.json`
- `owlmail-windows-amd64.exe` and `owlmail-windows-amd64.exe.spdx.json`
- `owlmail-licenses.tar.gz`

Linux amd64 download, verification, and launch example:

```bash
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.11.3/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.11.3/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.11.3/checksums.txt.sigstore.json
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.11.3/owlmail-licenses.tar.gz
grep ' owlmail-linux-amd64$' checksums.txt | sha256sum -c -
grep ' owlmail-licenses.tar.gz$' checksums.txt | sha256sum -c -
gh attestation verify owlmail-linux-amd64 --repo soulteary/owlmail
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
tar -xzf owlmail-licenses.tar.gz
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64
```

Keep the extracted `owlmail-licenses/` directory with redistributed binaries.
The bundle covers OwlMail and its Go dependencies; image operating-system
packages retain their own terms.

```bash
cosign verify \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/soulteary/owlmail:0.11.3
```
