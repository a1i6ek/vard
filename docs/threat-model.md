# Threat model

What Vard protects, where trust changes hands, who attacks it, and which rule
answers each threat. The rules are the **Security invariants** in
`CLAUDE.md`; each one named here is quoted by its opening words, so it can
be found there. A threat with no invariant is listed under
[Gaps](#gaps-threats-not-yet-answered), with the phase that answers it.

Keep this file true: a change that adds an asset, a boundary or an invariant
updates it.

## Assets

| Asset                                                                     | Where it lives                                                                     | Harm if lost                                                                    |
| ------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| `VARD_SECRET_KEY`                                                         | The API's environment                                                              | Opens every sealed value: provider secrets, SMTP password, TOTP seeds, tag key. |
| Token signing keys                                                        | Postgres, sealed (`signing_keys`)                                                  | Anyone can mint ID and access tokens for any user and any application.          |
| Users' passwords                                                          | Postgres, Argon2id                                                                 | Account takeover here and, through reuse, elsewhere.                            |
| Second factors: TOTP seeds, recovery codes, passkey credentials           | Postgres; seeds sealed, recovery codes keyed hashes, passkeys public keys          | A stolen password becomes enough.                                               |
| Sessions, codes, refresh tokens, reset and verification links             | Postgres, SHA-256 of 256 random bits                                               | Sign-in as the user without any credential.                                     |
| Access and ID tokens in flight                                            | Clients and APIs                                                                   | Calls to APIs as the user until expiry or revocation.                           |
| Administrators' accounts and API tokens                                   | Postgres                                                                           | Every user, every application, every setting.                                   |
| Application credentials: client secrets, redirect URIs, introspector list | Postgres                                                                           | Impersonate an application; send codes and tokens to an attacker.               |
| Federation settings: social and SSO providers, SAML metadata              | Postgres, secrets sealed                                                           | Sign in as anyone at a domain the connection holds.                             |
| Personal data: names, addresses, the activity log                         | Postgres (`users`, `user_fields`, `audit_logs`) and its backups (`make db-backup`) | Privacy breach; account enumeration.                                            |
| Availability of sign-in                                                   | Both apps, the API and Postgres (Redis is optional)                                | Nobody signs in to any application that depends on the server.                  |

## Trust boundaries

```
 Browser ──(1)──▶ Infrastructure (TLS, routing) ──(2)──▶ web/id, web/console (SvelteKit)
                                                              │
                                                             (3) API_PATHS only, X-Forwarded-For set by the app
                                                              ▼
 Client apps, APIs ──(1)──▶ … ──▶ web/id ──────────────▶ API :8080 (public) / :8081 (admin)
                                                              │
                                                             (4)
                                                              ▼
                                                   Postgres          Redis (optional cache)
                                                              │
                                                             (5) outbound.NewFederationClient
                                                              ▼
                       Social providers, SSO (OIDC, SAML), back-channel logout URIs, SMTP, Pwned Passwords
```

1. **The internet to the infrastructure.** Nothing on the far side is
   trusted. TLS ends here (`DEPLOY.md`).
2. **The infrastructure to the apps.** The apps trust the infrastructure
   only to say the caller's address, and only through `ADDRESS_HEADER` when it
   is configured.
3. **The apps to the API.** The API believes `X-Forwarded-For` only from the
   apps' own addresses (`VARD_TRUSTED_PROXIES`: loopback in development, the
   two containers' fixed addresses in `compose.yaml`). The app drops whatever
   the client sent in that header. Only the paths in each app's `API_PATHS`
   cross.
4. **The API to its data.** Postgres and Redis sit on an internal network the
   apps are not on. The API runs as a role that may only read and write rows.
   Redis holds nothing that is not also in Postgres, and no session, code or
   token is ever read from it.
5. **The API to the outside.** Every URL an administrator or user supplies
   is reached through `outbound.NewFederationClient`, which refuses private,
   link-local and metadata addresses when it dials.

## Attackers

| Attacker                                   | Can                                                                                                |
| ------------------------------------------ | -------------------------------------------------------------------------------------------------- |
| **A1** Anonymous on the internet           | Send any request to the apps; register accounts; run credential stuffing from many addresses.      |
| **A2** A malicious web page                | Make the user's browser send requests and form posts with its cookies; frame pages; open popups.   |
| **A3** A malicious or compromised client   | Hold a real `client_id`, or claim to; ask for any scope, resource and redirect URI.                |
| **A4** A network or token thief            | Read a token or code from a log, a referrer, browser history, a device or a proxy.                 |
| **A5** A sibling subdomain                 | Run code on another host under the same registrable domain and set cookies for the parent domain.  |
| **A6** A malicious identity provider       | Answer social and SSO sign-ins with any claims; point metadata or endpoints at internal addresses. |
| **A7** A user attacking another user       | Register an address first; know the victim's address and some of their history.                    |
| **A8** An administrator without full power | Use the panel within their role, and try to go beyond it.                                          |
| **A9** Someone with a database copy        | Read a backup or a replica: every row, but not the API's environment.                              |

Out of scope: an attacker with the API's environment (`VARD_SECRET_KEY`) or
code execution on the API, who owns the server; a compromised user device or
browser; denial of service by volume, which is the infrastructure's to absorb.

## Threats and what answers them

### Credentials and sign-in

| Threat                                                                    | Attacker | Answered by                                                                                                                |
| ------------------------------------------------------------------------- | -------- | -------------------------------------------------------------------------------------------------------------------------- |
| Password guessing and credential stuffing against one account             | A1       | "A user's sign-in is throttled, never locked …" — counted before checked, longer waits past the free failures.             |
| Locking a victim out by failing on purpose                                | A1, A7   | The same: never locked, and the known-browser cookie gives the owner a budget a stranger's failures do not spend.          |
| Guessing a second factor after the password                               | A1       | The same count covers the second factor; a right password does not clear it.                                               |
| Learning which addresses have accounts, by wording or timing              | A1       | "Unknown accounts cost the same as wrong passwords"; registering a taken address answers as a fresh one would.             |
| Offline cracking after a database leak                                    | A9       | "Passwords are Argon2id"; "Secrets are stored hashed or sealed, never plain."                                              |
| Pre-registering a victim's address and keeping access after they claim it | A7       | "The first proof of an address ends what came before it"; social sign-in never makes an account for an unverified address. |
| An administrator's password guessed                                       | A1       | Administrators are throttled the same way, on a keyed hash of the username.                                                |
| A token seen before an administrator's second factor used after it        | A4       | "An administrator's session is new once its second factor passes."                                                         |
| Racing two first-time setups to create two super admins                   | A1       | "The panel is set up once" — counted under an advisory lock.                                                               |

### Sessions and cookies

| Threat                                                        | Attacker | Answered by                                                                                                      |
| ------------------------------------------------------------- | -------- | ---------------------------------------------------------------------------------------------------------------- |
| Session fixation or tossing from a sibling subdomain          | A5       | "Session cookies are read through `session.Token` …" — `__Host-` cookies over https, duplicates treated as none. |
| Cross-site request forgery against the account and admin APIs | A2       | "Browsers are held to exact origin lists"; state-changing requests must be JSON.                                 |
| Clickjacking the Consent or Agreements page                   | A2       | "The apps run no script they did not ship" — `frame-ancestors 'none'`.                                           |
| Logging a user out by linking to the logout endpoint          | A2       | "A logout without `id_token_hint` asks first."                                                                   |
| A session or token living on after the user ends it           | A4       | "An access token ends with what it was issued in" — revocations read from Postgres, never a cache.               |
| Script injection stealing what a page shows                   | A2, A8   | Content Security Policy without `unsafe-inline`; "An uploaded picture is what its bytes are" (no SVG logos).     |

### Authorization and tokens

| Threat                                                                                    | Attacker | Answered by                                                                                                                  |
| ----------------------------------------------------------------------------------------- | -------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Open redirect, or a code sent to an attacker's redirect URI                               | A3       | "Redirect targets are exact-matched redirect URIs"; `request` and `registration` refused, never ignored.                     |
| A desktop app's any-port `localhost` redirect, which a resolver can point off the machine | A4       | Only `127.0.0.1` and `[::1]` match on any port (`AllowsRedirect`, RFC 8252 §8.3); `localhost` matches exactly or not at all. |
| Authorization code interception and injection                                             | A3, A4   | "Every client uses PKCE"; RFC 9207 `iss` on the response; "Single-use means one atomic `UPDATE` …".                          |
| Replaying a code or refresh token                                                         | A4       | "Single-use means one atomic `UPDATE` …" — replay revokes what was issued, or the family.                                    |
| Using a stolen DPoP-bound token without the key                                           | A4       | "A DPoP-bound access token is never taken as a bearer token"; a key-bound code needs a proof from that key.                  |
| Making the server verify a huge RSA key from a DPoP proof                                 | A1       | "A key from outside is between `jose.MinRSABits` (2048) and `jose.MaxRSABits` (4096) long."                                  |
| An application accepting terms or consent on the user's behalf                            | A3       | "Only the person, in their own browser, accepts terms or allows an application."                                             |
| Skipping a second factor the flow requires                                                | A3, A7   | "A flow that requires a second factor is met only by a session that passed one of the account's own."                        |
| One client reading another's tokens by introspection                                      | A3       | "Introspection answers a token's own client, or the API's."                                                                  |
| Registering a rogue client                                                                | A1       | "Registering a client takes an initial access token."                                                                        |
| Guessing a device user code                                                               | A1       | "Every one-time thing is taken with one conditional UPDATE" — user codes are keyed hashes and entering them is rate-limited. |

### Federation

| Threat                                                                                                                                                                                              | Attacker | Answered by                                                                                                                                                                                       |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| SSRF through a provider URL, SAML metadata or back-channel logout URI                                                                                                                               | A6, A8   | "Outbound requests to admin- or user-supplied URLs" — `outbound.NewFederationClient`; a logout token follows no redirect.                                                                         |
| Hammering a provider's key endpoint by naming unknown keys                                                                                                                                          | A1, A6   | The same: refetched at most once a minute per key set.                                                                                                                                            |
| An identity provider passing off an address it never verified (a missing `email_verified`, or an address read from a claim it does not cover)                                                       | A6, A7   | "An OpenID Connect SSO provider vouches for an address only by saying so."                                                                                                                        |
| A connection signing in addresses at domains it does not own                                                                                                                                        | A6       | "An SSO connection signs in only addresses it may."                                                                                                                                               |
| A connection taking over existing accounts                                                                                                                                                          | A6       | "An SSO connection takes over an existing account only at a proved domain."                                                                                                                       |
| A provider-signed assertion the SAML library mishandles: no `<Issuer>` (a panic) or a `<NameID>` in another namespace (a spoofed subject); `crewjam/saml` v0.5.1, GO-2026-6909, fixed in no release | A6       | "A SAML response signs in only the browser that started it, once" — an Issuer on every assertion and every NameID in SAML's namespace, checked before the library parses (`checkSAMLAssertions`). |
| SAML response replay, injection or login CSRF                                                                                                                                                       | A2, A6   | "A SAML response signs in only the browser that started it, once."                                                                                                                                |
| An identity provider claiming a second factor it did not perform                                                                                                                                    | A6       | The SSO part of the second-factor invariant: only when the connection trusts the provider, from a signed value.                                                                                   |

### Data at rest and in the logs

| Threat                                                        | Attacker | Answered by                                                                                                                            |
| ------------------------------------------------------------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| Reading provider secrets, seeds or SMTP passwords from a copy | A9       | "Secrets are stored hashed or sealed, never plain" — sealed under a key the copy does not hold.                                        |
| Moving a sealed value to another row or column to open it     | A9, A8   | The same: sealed with table, column and row as additional data.                                                                        |
| Internal details reaching the browser                         | A1       | "Nothing internal reaches a browser"; the request log records the path, never the query string.                                        |
| An administrator going beyond their role                      | A8       | Every admin route behind `session.Require` and the permission it needs (`How things are added`); the activity log records each change. |
| An administrator reading or changing cache entries directly   | A8       | The panel has no cache inspector (removed in Phase 1): Redis is reached only by the server, with no route to read or edit its keys.    |

## Gaps: threats not yet answered

| Gap                                                                                                                                                                                                                                                                                  | Phase that answers it |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------- |
| A leaked client secret is a full credential, and cannot be rotated without downtime.                                                                                                                                                                                                 | 2                     |
| DPoP proofs can be made ahead of time and replayed within their window (no server nonce).                                                                                                                                                                                            | 3                     |
| `VARD_SECRET_KEY` can never be rotated, so a suspected leak cannot be recovered from.                                                                                                                                                                                                | 3                     |
| An API checking access tokens locally does not see a revocation until the token expires: at most an hour (the default is 10 minutes).                                                                                                                                                | 4                     |
| A refresh family stays usable however long it sits unused.                                                                                                                                                                                                                           | 3                     |
| A back-channel logout that fails is not retried: the application keeps its session.                                                                                                                                                                                                  | 5                     |
| A SAML response with a signed `<Response>` but unsigned `<Assertion>` is accepted (`crewjam/saml` waives the assertion's signature then), although the metadata says `WantAssertionsSigned`. The response signature covers the assertion, so this is defence in depth, not a bypass. | 6                     |
| Signing-in, token and failure counts are not observable, so an attack in progress is not seen.                                                                                                                                                                                       | 9                     |

Each gap is checked again when its phase lands; a gap found to be wrong is
removed from this list with the reason in the pull request.
