# 0010 The passkey RP ID, chosen once

- **Status:** Accepted (2026-11-21)
- **Date:** 2026-11-21

## Context

Every WebAuthn credential is bound to a relying party ID: a domain. A browser
offers a passkey only to pages on that domain or below it, and a platform
offers it to a native app only when that domain publishes the app
(`apple-app-site-association`, `assetlinks.json`). The ID can't be changed
afterwards: a passkey made for `id.example.com` will never be offered for
`example.com`, nor the other way round. Changing it means every user enrols
again.

Until now the ID was the sign-in pages' host, with no way to choose.

## Decision

**The sign-in host by default, or a domain above it, set once before launch.**

- **`VARD_PASSKEY_RP_ID`:** empty is the sign-in pages' host (the
  `VARD_ACCOUNT_URL` host). Set, it must be that host or a domain above it
  (`example.com` for `id.example.com`), and never a public suffix (`co.uk`,
  `com`): the server refuses to start otherwise.
- **The panel's passkeys** (administrators') stay on the panel's own host:
  separate credentials, never offered on the sign-in pages.
- **Ceremonies stay where they are:** they still run on the sign-in pages,
  whose origin is the only one accepted. A domain above them only widens
  where else a browser or app may _offer_ the passkey.
- **Mobile apps:** a mobile application in the panel may list its iOS app
  IDs and Android packages, which the sign-in origin publishes. Those files
  must be served at the RP ID's domain. With the default ID, that is the
  sign-in origin, and it serves them itself. With a parent domain, whoever
  runs that domain serves the same two files there (copy them from the
  sign-in origin).
- **Android app origins aren't accepted** (`android:apk-key-hash:…`):
  native apps sign in through the system browser (RFC 8252), on the sign-in
  pages. An app that runs its own WebAuthn ceremony against this server
  would need an API that doesn't exist; that is a later decision.

## Alternatives rejected

- **Always the parent domain.** It would share every user's passkeys with
  every site on the domain, which an installation should choose, not
  inherit.
- **Leave it unconfigurable.** An installation that later wants its passkeys
  usable on `example.com` would have to make every user enrol again.
- **Several RP IDs.** A credential has exactly one; Related Origin Requests
  (`/.well-known/webauthn`) are for letting other origins use _one_ ID, not
  for having several. They can come later on top of this.

## Consequences

- **Choose before launch:** DEPLOY.md and `.env.example` say so. After the
  first passkey, the setting must not change; changing it makes every
  passkey unusable, and the users sign in some other way and enrol again.
- **Parent-domain care:** with a parent domain, a compromised site anywhere
  under it could run ceremonies for the same ID (WebAuthn allows any origin
  under the RP ID). It can't produce an assertion without the user's
  authenticator, but it can ask for one. Use a parent domain only when
  every site under it is trusted.
