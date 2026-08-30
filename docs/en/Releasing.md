# Release process

This guide is for OwlMail maintainers. A release is identified by a signed or
annotated semantic-version tag such as `v0.5.0`. GitHub release binaries and
container images must be built from that same tag.

## Sources of truth

- `CHANGELOG.md` records user-visible changes.
- `docs/en/Release-X.Y.Z.md` is the curated English release body.
- `docs/zh-CN/Release-X.Y.Z.md` is the corresponding Chinese release note.
- `.github/workflows/release.yml` builds formal binaries, checksums, per-binary
  SBOMs, provenance, signatures, and multi-architecture release images from
  tags.
- `.github/workflows/docker.yml` publishes moving `main`, `latest`, and commit
  snapshots for the default branch only.

The release workflow prepends the curated English note to GitHub's generated
change list. It refuses manual runs for a missing tag or missing release-note
file.

## Pre-release checklist

- [ ] All feature, fix, dependency, report-card, and release-documentation PRs are merged.
- [ ] `CHANGELOG.md` and both language release notes describe the final diff from the previous tag.
- [ ] No release note contains unresolved placeholders or unsupported compatibility claims.
- [ ] Required checks on the exact `main` commit are green.
- [ ] `go test -race ./...`, `go vet ./...`, `go mod verify`, browser tests, and documentation tests pass.
- [ ] `govulncheck ./...` reports no reachable vulnerability, or every exception is documented.
- [ ] `.bun-version` and the release workflow's pinned Go, Bun, and
  `govulncheck` versions match the intended release toolchain.
- [ ] A multi-architecture Docker build succeeds.
- [ ] The complete mail directory has been backed up for any upgrade smoke test using persistent data.

For 0.5.0, also confirm the Go 1.27 dependency upgrade, repository-local Go
Report Card, Bun migration, and embedded webhook configurator are merged and
described by the release notes before tagging.

## Create the release tag

Update local `main`, record the exact commit, then create one annotated tag:

```bash
git switch main
git pull --ff-only
git status --short
git rev-parse HEAD
git tag -a v0.5.0 -m "OwlMail v0.5.0"
git push origin v0.5.0
```

Do not move or reuse a published release tag. If a release needs a correction,
make a new patch version.

The tag push starts the release workflow. A manual run is only a retry mechanism
and must execute at the same tag ref so its OIDC identity is tag-bound:

```bash
gh workflow run release.yml --ref v0.5.0 -f version=v0.5.0
```

The workflow rejects a manual run whose ref and requested version differ, then
checks out that tag before building. Before publishing assets, the job reruns
dependency verification, formatting, `go vet`, race-enabled Go tests,
`govulncheck`, and the Bun browser/documentation checks against the tag. It then
generates SPDX SBOMs, GitHub artifact attestations, and keyless Sigstore
signatures before publishing the release.

## Verify published artifacts

In addition to GitHub's automatic source archives, a release made with the
current workflow contains:

- `checksums.txt`
- `checksums.txt.sigstore.json`
- `owlmail-linux-amd64`
- `owlmail-linux-amd64.spdx.json`
- `owlmail-linux-arm64`
- `owlmail-linux-arm64.spdx.json`
- `owlmail-darwin-amd64`
- `owlmail-darwin-amd64.spdx.json`
- `owlmail-darwin-arm64`
- `owlmail-darwin-arm64.spdx.json`
- `owlmail-windows-amd64.exe`
- `owlmail-windows-amd64.exe.spdx.json`

Download all files into an empty directory and verify them:

```bash
sha256sum -c checksums.txt
gh attestation verify owlmail-linux-amd64 --repo soulteary/owlmail
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

The checksum manifest and GitHub provenance attestations cover all five
executables and all five adjacent `*.spdx.json` SBOMs.

Smoke-test at least one binary:

```bash
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64 -smtp 11025 -web 11080
curl --fail http://localhost:11080/healthz
curl --fail http://localhost:11080/api/v1/version
```

Stop the smoke-test process after both endpoints and one SMTP receipt succeed.

## Verify container publication

For `v0.5.0`, verify the `0.5.0`, `0.5`, `0`, and commit-SHA tags and both target
architectures. `main` and `latest` are moving default-branch tags and must not be
used to prove release reproducibility.

```bash
docker buildx imagetools inspect ghcr.io/soulteary/owlmail:0.5.0
cosign verify \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/soulteary/owlmail:0.5.0
docker run --rm \
  -p 127.0.0.1:11025:1025 \
  -p 127.0.0.1:11080:1080 \
  ghcr.io/soulteary/owlmail:0.5.0
```

Confirm the manifest lists `linux/amd64` and `linux/arm64`, then repeat the
health, version, and SMTP smoke tests against the container. The manifest also
has BuildKit SBOM and maximum-mode provenance attestations, a GitHub provenance
attestation, and explicit OCI source, revision, version, and MIT license labels.

## Post-release checklist

- [ ] GitHub marks `v0.5.0` as the latest non-prerelease release.
- [ ] The curated note appears before the generated pull-request list.
- [ ] Every binary, SBOM, checksum, signature bundle, and GitHub attestation is
  downloadable or discoverable and verified.
- [ ] Container version and commit tags resolve to the expected manifest.
- [ ] The release image's Cosign signature and OCI attestations verify.
- [ ] Go installation with `@v0.5.0` succeeds on the documented Go version.
- [ ] README and release-note installation commands resolve.
- [ ] Any release failure or known limitation is added to the release body and changelog.
