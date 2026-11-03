# Multiuser release readiness

## Published, with the gate retained

[0.14.0-alpha.1](https://github.com/soulteary/owlmail/releases/tag/v0.14.0-alpha.1) is a published infrastructure preview. Its [release workflow](https://github.com/soulteary/owlmail/actions/runs/41459011723), [CI](https://github.com/soulteary/owlmail/actions/runs/41457095521), and [Docker verification](https://github.com/soulteary/owlmail/actions/runs/41457095598) succeeded. Normal release binaries still reject multiuser startup. Synthetic proxy fixtures are test components, not a supported production entry point.

The remaining scope is independent personal inboxes. Gorge uses its existing SMTP support with system credentials and recipient routing; no new adapter or per-user SMTP password is required.

## Stable standalone release scope

[0.14.0](Release-0.14.0.md) supports standalone operation; the normal binary keeps the multiuser gate. Artifact publication and verification are recorded from the release workflow, separately from completing the acceptance below or enabling multiuser mode. The original Phorge PHP adapter and Gorge SMTP transport have isolated component coverage, not a complete notification deployment or real Stargate contract.

## Upstream implementation is available; real acceptance is pending

[Stargate #229](https://github.com/soulteary/stargate/pull/229) implements exact
verified-email session evidence at commit
`67d1e1c8dc458433a2cb8df41e4ce4d9081ae547`. See the
[updated identity contract](Stargate-Identity-Contract.md). Passing upstream CI
closes the missing source implementation, not T02 or T16: no isolated real
identity/notification deployment was available for this round. Keep both items
open, keep the normal runtime gate, and do not inject a constant verified header.

## Remaining acceptance before opening multiuser mode

1. **T02 — Real identity and historical ownership.** Validate the actual Stargate stable subject/namespace, trusted verified-email declaration, and proxy header stripping. Reject forged assertions and deny claiming with an unverified email; a valid stable subject without verified email can still obtain an empty personal inbox. A currently verified address does not establish historical ownership. Document address reuse and continuous historical ownership; use `explicit-history` when continuity cannot be guaranteed, without exposing old history automatically. Synthetic identities do not satisfy this item.
2. **T16 — Real notification path.** Exercise Phorge notifications for two identities through the deployed Gorge SMTP configuration and Stargate. Verify delivery before first login, later claiming, cross-user denial, TLS/domain restrictions, an SMTP system account's inability to read mail, and failure behavior without unintended real-mail fallback. Preserve redacted configuration and results.
3. **T17 — Normal binary and release decision.** After T02/T16, test the exact candidate's normal binary for accepted and rejected startup/configuration, real login/claim/isolation, polling, restart recovery and standalone compatibility. Review gate removal separately, only after blocking findings close. A successful gated Alpha release does not itself authorize this change.

Beta and RC releases are optional sequencing choices. Team administration, shared spaces, production outbound mail, a fixed soak-test duration, production NFS support, two-host recovery and Kerberos are not added as universal prerequisites. Validate the storage, backup and identity boundaries actually used by the intended deployment; the isolated NFS lab does not certify SQLite-on-NFS.

For detailed evidence and historical failures, see the [implementation progress](../zh-CN/Multiuser-Progress.md), [implementation plan](../zh-CN/Multiuser-Implementation-Plan.md), and [Alpha release notes](Release-0.14.0-alpha.1.md). Historical unexecuted checks remain historical records rather than current release status.
