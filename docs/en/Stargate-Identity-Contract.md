# Stargate identity contract for OwlMail

This is a source-level deployment contract review, not production acceptance.
OwlMail's public multiuser release gate remains closed. A stable subject, a
currently verified email address, and ownership of historical mail are three
separate requirements.

## Implemented upstream milestone; deployment acceptance still pending

[Stargate PR #229](https://github.com/soulteary/stargate/pull/229) was merged at
[`67d1e1c8dc458433a2cb8df41e4ce4d9081ae547`](https://github.com/soulteary/stargate/tree/67d1e1c8dc458433a2cb8df41e4ce4d9081ae547).
It implements `X-Auth-Email-Verified` for authenticated Warden-backed sessions
with successful, server-bound Herald **email-channel** challenge evidence matching
the explicit subject and exact canonical asserted email. Local parts and plus tags
remain exact; only the domain is normalized. Old sessions and password, SMS,
TOTP or header authentication do not acquire evidence from generic AMR.

Evidence is stored as session primitives, expires independently of sliding session
access, and is permanently cleared when directory refresh observes an email
change. The handler removes client-supplied verification headers before checking
authentication. Evidence is not a statement of historical address ownership, nor
does it enforce the user's ID non-reassignment policy.

The upstream PR's lint, race/unit coverage, Linux image packaging and Windows
startup checks passed. Its PR workflow is not a vulnerability-scan result. These
are source/component checks with test providers, not a real deployed
Stargate/Warden/Herald or Phorge notification acceptance result. Older upstream
versions do not gain the feature automatically. Deploy a version containing this
commit, configure the proxy's response-header allowlist and private peer boundary,
and perform [T02/T16/T17 acceptance](Multiuser-Release-Readiness.md) before any
multiuser gate decision. The normal OwlMail gate remains closed.

The following review describes the **initial pinned baseline** and explains why
this implementation was necessary; its missing-export statements apply to those
older revisions, not to the merged milestone above.

## Initial reviewed upstream revisions

- [Stargate `111c3f94bc1d7f76d6e31548918adda2f82b61e9`](https://github.com/soulteary/stargate/tree/111c3f94bc1d7f76d6e31548918adda2f82b61e9)
- [Warden `af23fee285e3cb2d4c01cac56d50db86ab9338dc`](https://github.com/soulteary/warden/tree/af23fee285e3cb2d4c01cac56d50db86ab9338dc)

These pins describe the inspected code. They do not imply a running Stargate,
Warden, or Herald deployment has passed OwlMail acceptance.

## Stable subject: an existing deployment option

Warden already supports `REQUIRE_EXPLICIT_USER_ID=true`. Its
[`Normalize` implementation](https://github.com/soulteary/warden/blob/af23fee285e3cb2d4c01cac56d50db86ab9338dc/internal/define/struct.go)
then leaves a missing `user_id` unresolved instead of deriving one from contact
information, allowing normal validation to reject the record. Configure the
authoritative user source with explicit, unique IDs and include `user_id` in
Warden's response fields. Stargate can forward that value as `X-Auth-User`.
This path does not require a new upstream identity API.

The operator must additionally guarantee that each ID belongs to one person,
survives email and phone changes, and is never assigned to another person.
`REQUIRE_EXPLICIT_USER_ID` prevents missing IDs; it cannot enforce those lifecycle
rules across operator edits or source replacements. Keep OwlMail's identity
namespace stable and distinct for each identity authority.

Without that configuration, Warden's
[identity derivation](https://github.com/soulteary/warden/blob/af23fee285e3cb2d4c01cac56d50db86ab9338dc/internal/define/identity.go)
hashes phone or email when an ID is absent. Stargate's
[login fallback](https://github.com/soulteary/stargate/blob/111c3f94bc1d7f76d6e31548918adda2f82b61e9/src/internal/handlers/login.go)
also generates an identifier from phone or email when Warden supplies no ID.
Changing the hash strategy does not make a reassigned address or phone number a
non-reassignable subject. A hash's repeatability is not an identity lifecycle
contract. Do not use these fallbacks as proof of that requirement.

## Initial baseline: missing exported email verification

Stargate's
[ForwardAuth configuration](https://github.com/soulteary/stargate/blob/111c3f94bc1d7f76d6e31548918adda2f82b61e9/src/internal/handlers/forwardauth.go)
exports user, email, name, scopes, role, and AMR headers. It does not configure a
verified-email declaration. Warden's
[response mapper](https://github.com/soulteary/warden/blob/af23fee285e3cb2d4c01cac56d50db86ab9338dc/internal/router/response.go)
returns directory attributes such as `mail` and `user_id`, without an
email-verification assertion. Directory membership or a listed `mail` value does
not prove that the current session verified control of that address.

There is a real email verification fact that a future upstream contract could
preserve: Stargate's
[challenge creation](https://github.com/soulteary/stargate/blob/111c3f94bc1d7f76d6e31548918adda2f82b61e9/src/internal/handlers/send_verify_code.go)
records server-side subject, channel, and destination; its login flow checks a
successful Herald response and the matching user ID. However, current session
storage retains user attributes and generic AMR, not an exported assertion
binding the verified email destination to the current subject and current
forwarded email.

An OTP-based upstream implementation would need to preserve that binding after successful
**email-channel** verification, reject missing or mismatched evidence, and
invalidate it when the asserted email changes. SMS verification, TOTP, password
login, a directory lookup/listed email without an explicit verification guarantee,
and a generic `otp`/AMR value do not establish
that the forwarded email address was verified. A proxy must not unconditionally
add `X-Auth-Email-Verified: true`, or derive it merely from one of those methods.

A controlled directory with an explicit guarantee of verified email ownership
could alternatively support a trusted declaration after its real contract is
accepted. Email OTP is one possible source of evidence, not a required exclusive
method. The plain Warden `mail` response reviewed here supplies no such explicit
verification guarantee.

There is therefore no existing end-to-end verified-email assertion to configure
at these pins. A valid explicit subject can be provided without changing
upstream code, but that alone does not satisfy automatic mailbox claiming.

## Current control does not establish historical ownership

Even a future genuine verified-email assertion proves current address control,
not ownership throughout the period when pending mail accumulated. Keep the
existing claim policies distinct:

- `verified-email` is for deployments that explicitly guarantee the address was
  not recycled or reassigned during that history window. A successful email OTP
  alone cannot supply this historical guarantee.
- `explicit-history` binds future delivery to the personal mailbox once trusted
  current-email evidence is available, while leaving older pending history
  isolated for an operator-confirmed migration. It does not automatically
  transfer an address already bound to another stable subject.
- Without verified-email evidence, OwlMail can resolve/create the stable
  subject's personal mailbox, but does not claim a pending mailbox or bind a new
  email route from an unverified email attribute. A new personal mailbox starts
  empty; this is not successful historical claiming.

Production acceptance must also establish trusted-proxy isolation and stripping
of client-supplied identity headers. Component tests with synthetic declarations
cannot substitute for these upstream and deployment facts. See the
[multiuser release-readiness checklist](Multiuser-Release-Readiness.md).

For the recommended topology, rotation and operational limits, see the
[deployment and operations contract](Stargate-Deployment.md).
