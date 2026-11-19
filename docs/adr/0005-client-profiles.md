# 0005 Client profiles

- **Status:** Accepted (2026-11-19)
- **Date:** 2026-11-19

## Context

The server is built to serve five kinds of client well: a server-side web
application (a BFF), a browser application with no backend, a mobile app, a
desktop or command-line app (or a TV), and a service with no user. Each needs
a different, small set of settings, and each must never have some others:
a browser app a secret, a mobile app a loopback redirect, a service a user
flow.

Until now an application had a `type` (web, spa, native, m2m). It mixed two
of the kinds: `native` covered both mobile and desktop apps, so a mobile app
could register a loopback redirect, which only a desktop app needs. And a
service could still authenticate with a secret.

## Decision

**An application has a `profile`, replacing `type`**, with five values:

| Profile   | Client       | Authenticates with                               | Grants                                           | Sent back to                                                                                               |
| --------- | ------------ | ------------------------------------------------ | ------------------------------------------------ | ---------------------------------------------------------------------------------------------------------- |
| `web`     | confidential | `private_key_jwt`, or a secret while it migrates | code (and refresh, device, `client_credentials`) | `https`, or `http` on loopback, matched exactly                                                            |
| `spa`     | public       | nothing: PKCE                                    | code, refresh                                    | `https`, or `http` on loopback, matched exactly                                                            |
| `native`  | public       | nothing: PKCE                                    | code, refresh, device                            | a claimed `https` address or a private-use scheme; Expo Go's `exp://` while in development; never loopback |
| `desktop` | public       | nothing: PKCE                                    | code, refresh, device                            | `127.0.0.1` or `[::1]` on any port (RFC 8252 §7.3), or a private-use scheme; never `localhost` or `https`  |
| `machine` | confidential | `private_key_jwt` alone                          | `client_credentials` alone                       | nowhere                                                                                                    |

- **Enforced on every save.** `Application.Normalize` applies what a profile
  implies, and `Validate` refuses what it forbids, every time an
  application is saved. Editing one setting never takes an application
  outside its profile.
- **Fixed once created.** A profile cannot be changed: copies of the app in
  use depend on it.
- **The console** starts a new application from its profile.
- **Dynamic registration (RFC 7591)** has no word for a profile, so it is
  read from the metadata (`profileOf`). A combination no profile allows is
  refused with `invalid_client_metadata`, never quietly changed:
  - a service with a secret;
  - a native client with a secret;
  - a native client using only `client_credentials`.
- **`admin-cli`**, the server's own machine client, is no exception. It is
  made with no key, and an administrator registers the public key of the
  tool that uses it.

**The upgrade (migration 00016):**

- `m2m` becomes `machine`, and loses any secret it had.
- A `native` app that is sent back to loopback, or signs in by the device
  grant alone, becomes `desktop`; any other stays `native`.

## Alternatives rejected

- **A profile beside the type.** That means two columns that must always
  agree, and every check written against one of them.
- **Machine clients keeping secrets.** A secret is a shared credential that
  travels with every request. A service is exactly the client that can hold
  a private key, so it does.
- **An exception for `admin-cli`.** It holds the most powerful credential on
  the server, the admin API. The strongest method belongs there more than
  anywhere.

## Consequences

- **Breaking for existing clients:** a machine client must register keys
  before it can authenticate again. So must `admin-cli`, on an upgraded
  installation.
- **DPoP is not yet a requirement** of any profile. Making it required for
  SPA and mobile clients is Phase 3's work, and goes in this table when it
  lands.
- **mTLS is not one of the methods** (ADR 0006).
