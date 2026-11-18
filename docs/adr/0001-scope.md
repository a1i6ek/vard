# 0001 Scope: which standards Vard speaks

- **Status:** Proposed
- **Date:** 2026-11-17

## Context

An identity server is judged by what it refuses as much as by what it does.
Every grant, parameter and response mode a server accepts is code to keep
correct and an interaction an attacker can try. RFC 9700 (OAuth 2.0 Security
Best Current Practice) and the OAuth 2.1 draft have already retired several
parts of OAuth 2.0 because they could not be made safe.

Vard serves five kinds of client — a server-side web app (BFF), a browser app
without a backend (SPA), a native mobile app, a desktop or command-line tool,
and a machine or agent with no user — and nothing else. A standard is in scope
only when one of those five needs it.

## Decision

### Supported today

What the server implements now, checked against `internal/oidc/provider.go`
(discovery), `internal/model/application.go` and the token endpoint.

| Standard                                                     | Notes                                                                                                                   |
| ------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------- |
| OAuth 2.0 (RFC 6749)                                         | `authorization_code`, `refresh_token`, `client_credentials`. `response_type=code`, `response_mode=query` only.          |
| Bearer tokens (RFC 6750)                                     | Refused for a DPoP-bound token.                                                                                         |
| PKCE (RFC 7636)                                              | `S256` only; always for a client without credentials, optional for a confidential one (`RequirePKCE`).                  |
| Token revocation (RFC 7009)                                  |                                                                                                                         |
| Token introspection (RFC 7662)                               | A token's own client, or an API's listed introspectors.                                                                 |
| Authorization server metadata (RFC 8414), OIDC Discovery 1.0 |                                                                                                                         |
| JWT access tokens (RFC 9068)                                 | `typ: at+jwt`, `aud` never empty.                                                                                       |
| JOSE (RFC 7515, 7517, 7518, 7519)                            | ID tokens RS256 only. RSA keys from outside 2048–4096 bits.                                                             |
| Dynamic client registration (RFC 7591)                       | Only with an initial access token.                                                                                      |
| Device authorization grant (RFC 8628)                        |                                                                                                                         |
| Pushed authorization requests (RFC 9126)                     | Optional per request; no server-wide requirement yet.                                                                   |
| DPoP (RFC 9449)                                              | ES256, RS256, PS256 proofs; code binding with `dpop_jkt`. No server nonces yet.                                         |
| Authorization server issuer identification (RFC 9207)        | `iss` on every authorization response.                                                                                  |
| OAuth for native apps (RFC 8252)                             | Loopback redirects on any port; private-use schemes. `localhost` is still accepted (see Phase 1).                       |
| OpenID Connect Core 1.0                                      | Authorization code flow only. `prompt` `none`, `login`, `consent`; `max_age`; `login_hint`. Public subject identifiers. |
| OIDC RP-Initiated Logout 1.0                                 | Without `logout_hint` or `ui_locales` yet.                                                                              |
| OIDC Back-Channel Logout 1.0                                 | Sent at the moment the session ends, not yet retried durably.                                                           |
| OIDC Front-Channel Logout 1.0                                | Proposed for removal (Phase 1).                                                                                         |
| Authentication method reference values (RFC 8176)            | `amr`.                                                                                                                  |
| TOTP (RFC 6238), WebAuthn Level 2                            | Second factors and passwordless passkeys.                                                                               |
| SAML 2.0 Web Browser SSO, as a service provider              | Inbound single sign-on only, HTTP-POST binding.                                                                         |
| OAuth 2.0 / OIDC as a relying party                          | Social sign-in (see the providers below) and OIDC single sign-on.                                                       |
| `audience` request parameter                                 | Not a standard. Renamed to RFC 8707 `resource` outright in Phase 3, with no alias: nothing is in production.            |

### Social sign-in providers

Decided 2026-11-17 by the user:

| Provider               | Decision                                                                                                                                                                                                                                                                   |
| ---------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Google, Apple          | Kept.                                                                                                                                                                                                                                                                      |
| Generic OpenID Connect | Kept: any provider with a discovery document and a signed id_token.                                                                                                                                                                                                        |
| Yandex, VK             | Kept.                                                                                                                                                                                                                                                                      |
| Facebook               | Removed (Phase 1).                                                                                                                                                                                                                                                         |
| Generic OAuth 2.0      | Removed (Phase 1): it signs people in from a userinfo response with no signed id_token and no `email_verified`, so it can never prove an address; generic OpenID Connect covers the providers worth keeping.                                                               |
| Microsoft (Entra ID)   | Added in its own pull request after Phase 1. It keys an identity on `tid` + `oid`, never creates or links an account by the `email` claim (nOAuth), takes an address as proved only when `xms_edov` is true, and checks the multi-tenant issuer against the token's `tid`. |

### Added by the planned phases

| Standard                                                 | Phase | For which client profile                    |
| -------------------------------------------------------- | ----- | ------------------------------------------- |
| `private_key_jwt` (RFC 7523 §2.2, OIDC Core §9)          | 2     | Web (BFF), machine and agent                |
| PKCE for every client (RFC 9700 §2.1.1, OAuth 2.1)       | 3     | All                                         |
| DPoP server nonces, `dpop_bound_access_tokens`           | 3     | SPA, native                                 |
| Resource indicators (RFC 8707), replacing `audience`     | 3     | All that call an API; required for machines |
| `require_pushed_authorization_requests` (RFC 9126 §6)    | 3     | Web (BFF)                                   |
| ES256 and PS256 ID tokens                                | 3     | All                                         |
| Token exchange (RFC 8693)                                | 4     | Machine and agent                           |
| JWT introspection responses (RFC 9701)                   | 4     | APIs that check tokens                      |
| Durable back-channel logout; `logout_hint`, `ui_locales` | 5     | Web (BFF)                                   |
| Problem details (RFC 9457) outside the OAuth endpoints   | 7     | The apps                                    |
| OpenAPI 3.1 description of the HTTP API                  | 7     | Integrators                                 |

### Undecided: the user decides

These are open until a record of their own accepts or rejects them.

- **Mutual TLS (RFC 8705)**: `tls_client_auth` and certificate-bound
  tokens. TLS ends at the infrastructure in front of the apps, so the
  certificate would arrive in a header that only a trusted proxy may set.
- **Workload identity federation**: the JWT bearer grant (RFC 7523 §2.1) from
  outside issuers such as Kubernetes or GitHub Actions.
- **FAPI 2.0 Security Profile**: decides how strict PAR becomes, and whether
  JAR is needed (FAPI 2.0 Message Signing).

### Deliberately not supported

| Standard                                                            | Why not                                                                                                                                                                                                           |
| ------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Implicit grant, `response_type=token`                               | Puts the access token in the URL fragment, where history, logs and other scripts can read it, and binds it to nothing. Removed by RFC 9700 §2.1.2 and OAuth 2.1. The SPA profile uses code + PKCE + DPoP instead. |
| Hybrid flows (`code id_token`, `code token`, `code id_token token`) | Only ever needed to protect a code that PKCE and the `iss` parameter now protect. Adds a second token path through the front channel for no gain.                                                                 |
| `response_mode=fragment`, `form_post`                               | No supported flow needs them: `query` carries only a code, which is useless without its verifier.                                                                                                                 |
| Resource owner password credentials grant                           | Teaches people to type their password into the client, cannot do a second factor, passkeys or SSO, and is removed by RFC 9700 §2.4 and OAuth 2.1.                                                                 |
| CIBA (Client-Initiated Backchannel Authentication)                  | Needs an out-of-band channel to the user's device that Vard does not have; the device grant serves the clients without a browser.                                                                                 |
| Acting as a SAML identity provider                                  | A second token format, signing pipeline and metadata surface to keep safe. Vard consumes SAML from enterprise identity providers; applications speak OIDC to it.                                                  |
| JAR (RFC 9101) `request` and `request_uri` by value or reference    | PAR already moves the request off the front channel, authenticated. `request` and `registration` are refused, never ignored (`CLAUDE.md`). Revisited only if FAPI 2.0 Message Signing is chosen.                  |
| `client_secret_jwt`                                                 | A shared secret used as an HMAC key gives none of `private_key_jwt`'s benefit (no secret on the server) and all of a client secret's risk.                                                                        |
| Pairwise subject identifiers                                        | No supported client needs unlinkable identifiers across applications of one organisation; adds a mapping table to every token.                                                                                    |
| The `claims` request parameter, Aggregated and Distributed Claims   | Scopes say what an application gets, in a form a person can read on the Consent page.                                                                                                                             |
| Self-issued OpenID Provider, OpenID for Verifiable Credentials      | A different trust model, and no supported client needs it.                                                                                                                                                        |
| Session Management 1.0 (`check_session_iframe`)                     | Depends on third-party cookies in an iframe, which browsers now block.                                                                                                                                            |
| Front-Channel Logout 1.0                                            | Proposed for removal in Phase 1 for the same reason; kept, labelled best effort, if the user decides so.                                                                                                          |

## Alternatives rejected

- **Supporting everything certified OpenID providers support**, for
  compatibility. Every one of the rejected items above has a safer
  replacement already in scope, and a client that needs a rejected item is a
  client this server does not mean to serve.
- **Keeping implicit and ROPC behind a per-client flag.** A flag that is easy
  to flip is the danger this project avoids; the code would still have to be
  correct.

## Consequences

- Discovery publishes only what is in the first two tables, and each phase
  updates it as it lands.
- Asking for something in the last table is answered with the standard error
  (`unsupported_response_type`, `unsupported_grant_type`,
  `request_not_supported`, …), never silently ignored.
- A client written for a legacy flow has to move to code + PKCE first.
- Adding anything from the last table needs a new record superseding this one.
