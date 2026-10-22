# OwlMail 0.11.2

发布日期：2026-10-22

本次维护版本将构建工具链升级至 Go 1.27.3，更新兼容的 Go 依赖及已发布的 Kit 版本，并使用经过验证的质量报告 Action。公开 API、配置和存储格式保持兼容。

部署前请备份完整邮件目录，原有安全设置保持不变；上一功能版本说明见 [0.11.1](Release-0.11.1.md)。

## Install

```bash
docker pull ghcr.io/soulteary/owlmail:0.11.2
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.11.2
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
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.11.2/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.11.2/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.11.2/checksums.txt.sigstore.json
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.11.2/owlmail-licenses.tar.gz
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
  ghcr.io/soulteary/owlmail:0.11.2
```
