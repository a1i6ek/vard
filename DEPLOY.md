# Deploying Vard on a server

One machine running Docker. The stack speaks plain HTTP; TLS and routing are
the infrastructure's, in front of it.

```
infrastructure ─▶ 127.0.0.1:3000  id       ─┐
               └▶ 127.0.0.1:3001  console  ─┤ edge network
                                   api ─────┘ (and out: mail, SSO, logout)
                                    │
                      postgres, redis   data network (internal, no way out)
```

- **Published:** only the two apps, on loopback. Postgres, Redis and the API
  publish nothing; the apps cannot reach Postgres or Redis.
- **The API runs as `vard_app`**, a database role that reads and writes rows
  and nothing more; it migrates as `vard`, the owner, and makes `vard_app`
  (with `POSTGRES_APP_PASSWORD`) on its first start.
- **The API believes only the two apps** about who is calling
  (`VARD_TRUSTED_PROXIES`, their fixed addresses).
- **Containers** run as unprivileged users with no capabilities, read-only
  filesystems (the apps and the API), `no-new-privileges`, memory and process
  limits, and capped logs.

## What you need

- A Linux server with Docker Engine and the Compose plugin, and git.
- Two DNS names pointing at it: one for the sign-in pages (the issuer, such as
  `id.example.com`) and one for the panel (`admin.example.com`).
- A firewall that lets in 22, 80 and 443 only:
  `sudo ufw allow OpenSSH && sudo ufw allow 80,443/tcp && sudo ufw enable`.

## First deployment

```sh
git clone https://github.com/a1i6ek/vard.git && cd vard

# compose.env, with new secrets — back it up: VARD_SECRET_KEY unlocks the
# signing keys, and without it every token and session is lost.
make docker-env public=https://id.example.com admin=https://admin.example.com
$EDITOR compose.env          # the VARD_SMTP_* settings, so mail is sent

make docker-up               # build and start; waits until everything is healthy
```

Route `PUBLIC_URL` to `ID_PUBLISH` and `ADMIN_URL` to `CONSOLE_PUBLISH`,
then open
`https://admin.example.com/new-super-admin` and make the first administrator
straight away: until someone has, whoever opens that page first can.
Every administrator sets up an authenticator app at their first sign-in:
`compose.env` says `VARD_ADMIN_MFA=required`, which a super admin can change
later in Administrators › Two-factor sign-in.

The server refuses to start on plain http addresses other than localhost.
`VARD_ALLOW_INSECURE_HTTP` lets plain http through for a test setup, with a
warning at every start; never set it on a real server. It never lets a weak
`VARD_SECRET_KEY` through: a key of hex digits alone under 64 characters, or
any other key under 43 characters or with fewer than 16 distinct characters,
always stops the server.

## Updating

```sh
git pull
make db-backup               # first, always
make docker-up               # rebuilds what changed, migrates the database on start
```

### Upgrading past 1.1.0

Back up the database first (`make db-backup`). The server applies these as it
starts, each with a down:

| Migration                         | What it changes                                                                                                           |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `00011_request_session`           | the session an authorization request was finished by, so only that session answers its Consent or Agreements page         |
| `00012_sso_assume_email_verified` | an OIDC SSO connection's `assume_email_verified` (off for every existing one)                                             |
| `00013_drop_frontchannel_logout`  | drops front-channel logout; back-channel is the one way applications are told                                             |
| `00014_drop_facebook`             | deletes Facebook providers (their identities and waiting sign-ins with them)                                              |
| `00015_private_key_jwt`           | an application's public keys (`jwks`, `jwks_uri`), and the client assertions already used                                 |
| `00016_client_profiles`           | an application's `type` becomes its `profile`; `m2m` becomes `machine` and loses its secret; see below                    |
| `00017_secret_overlap`            | the secret a rotation replaced, kept hashed while its overlap lasts                                                       |
| `00018_pkce_always`               | drops `require_pkce`: every client uses PKCE                                                                              |
| `00019_token_lifetimes`           | access tokens at most an hour; each application's refresh idle lifetime (14 days, or less); a refresh token's idle expiry |
| `00020_id_token_alg`              | each application's `id_token_signed_response_alg` (RS256 for every existing one)                                          |
| `00021_dpop_bound_tokens`         | `dpop_bound_access_tokens` per application, on for every SPA and mobile app                                               |

What changes for applications:

- **Machine clients authenticate with `private_key_jwt` alone.** Every
  former `m2m` application, `admin-cli` included, loses its secret and
  cannot get a token until its public key is registered: in the panel
  (Applications → the application → Credentials → a key set or an
  address), or through the admin API. The client then signs a short-lived
  assertion for each request (RFC 7523; `docs/adr/0005-client-profiles.md`).
- **A native app becomes `desktop`** when it is sent back to loopback, or
  signs in by the device grant alone; any other stays `native` (mobile). A
  desktop app may not register `http://localhost/…`: use
  `http://127.0.0.1/…`, which matches on any port. A mobile app may no
  longer register a loopback redirect.
- **Dynamic registration** (`/oauth2/register`) refuses what no profile
  allows, such as a client using `client_credentials` alone without
  `private_key_jwt`, instead of registering it with a secret.
- **`audience` is now `resource`** (RFC 8707), at the authorization, PAR,
  token and device endpoints; `audience` is no longer read. One resource per
  request: more than one, or one no API answers to, is `invalid_target`. An
  API's identifier must be an absolute URI (`https://…` or `urn:…`).
- **Every client uses PKCE**, confidential ones too: a request without an S256
  `code_challenge` is refused. `require_pkce` is gone.
- **Access tokens live an hour at most**; longer settings are brought down.
  Refresh families also end after 14 days unused (per application), and device
  codes after five minutes.
- **ID tokens may be signed with PS256 or ES256** per application
  (`id_token_signed_response_alg`); RS256 stays the default.
- **`VARD_REQUIRE_PAR=true`** requires every client to push its requests.
- **DPoP proofs carry the server's nonce** at the token, PAR and userinfo
  endpoints: a client gets one in `DPoP-Nonce` with every answer, and a proof
  without it is answered `use_dpop_nonce` (RFC 9449 §8). DPoP client
  libraries handle this. **An SPA's and a mobile app's every token request
  needs a proof** (`dpop_bound_access_tokens`), and the account and admin APIs
  now take a DPoP-bound token sent as DPoP with its proof.
- **An SSO connection's issuer is compared exactly**: a trailing slash the
  provider's discovery document does not have, or the other way round, is now
  refused. Type the issuer as the provider publishes it.
- **A rotated secret may overlap its successor**: rotating asks how long
  the old one keeps working (at once, an hour, a day or a week).
- **Front-channel logout and Facebook sign-in are gone**, and
  `VARD_CORS_ORIGINS` is no longer read.

### Upgrading to 1.1.0

Back up the database first (`make db-backup`). The server applies six
migrations as it starts, each with a down (`make migrate-down` steps back
one at a time):

| Migration                   | What it adds                                                                                      |
| --------------------------- | ------------------------------------------------------------------------------------------------- |
| `00005_dpop_jkt`            | `dpop_key` on authorization requests and codes, for a code bound to a DPoP key                    |
| `00006_step_up`             | a login flow's `require_second_factor` (off for every existing flow); a step-up's session         |
| `00007_remember_session`    | whether a session was "stay signed in" (existing sessions: no), so a step-up keeps its cookie     |
| `00008_sign_in_application` | the application a waiting sign-in with no authorization request is for (a device being allowed)   |
| `00009_sign_in_device`      | the device a step-up was started to allow, which "Not now" denies                                 |
| `00010_sso_second_factor`   | an SSO connection's `second_factor` (`local` for every existing one) and what its provider proves |

What changes for applications and the people signing in:

- **`amr` says `mfa` only after one of the account's own second factors**
  (an authenticator app, a recovery code or a passkey). A password and an
  emailed code is `pwd` and `otp`, no longer `mfa`: the emailed code proves
  only the mailbox a password reset also goes to. An application that
  reads `mfa` to decide anything gets it less often; one that needs it
  should sign in through a login flow that requires a second factor.
- **A login flow can require a second factor** (the Two-step verification
  step, "Require it"; off for every existing flow). Someone without one
  sets one up while signing in, once their address is confirmed; someone
  already signed in without one is asked for it in the session they have
  (`prompt=none` gets `interaction_required`), and may leave it with "Not
  now", which tells the application `access_denied`. Allowing a device is
  held to the same flow.
- **Refresh families end under a requiring flow.** Turning the requirement
  on for a flow in use refuses, at the next refresh, every refresh token
  whose sign-in never passed a second factor (`invalid_grant`, a retry
  within the 30-second grace included): the application signs the person
  in again. Nothing is revoked at once, and families that passed a factor
  carry on.
- **Single sign-on** under a requiring flow asks for the account's own
  factor (each connection's `local`, the default), or, for a connection set
  to trust its provider, takes the provider's own when its signed answer
  shows one of the values the connection lists (OpenID Connect `amr`,
  default `mfa`; SAML authentication contexts, which you list yourself).
  Otherwise it falls back to the account's own factor and the activity log
  records it.
- **Every second factor added** from the account page or while signing in
  emails its account a notice ("Two-step verification turned on" on the
  Mail page, where its text can be changed). With SMTP not configured it
  only reaches the log, like the other emails.
- **`dpop_jkt`** binds an authorization code to a DPoP key (RFC 9449 §10),
  as does pushing the request (PAR) with a DPoP proof. A bound code is
  exchanged only with a proof from that key; tried without it, the code is
  spent. `dpop_jkt` in a plain authorization request crosses the browser
  and can be stripped, so a client that needs the binding pushes its
  request with a proof.
- **RSA keys from outside are capped at 4096 bits.** A DPoP proof, or any
  key the server is handed, with a longer modulus is refused: verifying
  under a huge one costs the server. Keys this server makes are unchanged.
- **Social sign-in checks the provider's signature** on its id_token where
  the provider publishes keys (Apple, Google), fetched through the same
  guarded client as SSO. A provider's keys are fetched again only for a
  token naming a key not held, at most once a minute per key set, so a
  stream of bad tokens cannot make the server hammer the provider.
- The social and SSO callbacks are rate limited like their start routes,
  and a redirect URI's own query comes back byte for byte.

### Upgrading to 1.0.0

1.0.0 refuses to start with a weak `VARD_SECRET_KEY`, whatever else is set: a
key made only of hex digits needs at least 64 characters, and any other key at
least 43 characters and 16 distinct ones. A key made with `openssl rand -base64
32` (what `make docker-env` writes) or `openssl rand -hex 32` passes. The key
cannot be changed: under a new one nothing stored opens, so an installation
with a weak key starts again with a fresh database.

An installation from before the migrations were squashed into one setup
(`internal/database/migrations/setup`) cannot be stepped forward: the server refuses its
database and says so. Back it up and start on a new, empty database; its
rows are not carried over.

## Backups

`make db-backup` writes `backups/vard-<time>.dump` and keeps the newest
14 (`KEEP=30 make db-backup` for more). Run it from cron, and copy the dumps
off the machine: they hold every user and every sealed secret.

```
15 3 * * * cd /srv/vard && make db-backup >>/var/log/vard-backup.log 2>&1
```

Restoring is in the comment at the top of `scripts/db-backup.sh`.

## Checking it

```sh
make docker-ps                         # every container healthy
make docker-logs service=api           # the API's log
curl -sI https://id.example.com/.well-known/openid-configuration   # 200, with HSTS
```

## Settings

Everything is in `compose.env`; `compose.env.example` explains each. The ones
a server must not change: `VARD_SECRET_KEY` and `POSTGRES_PASSWORD`, which
the stored keys and the database were made with, and `POSTGRES_APP_PASSWORD`,
which `vard_app` was made with. `ID_PUBLISH` and
`CONSOLE_PUBLISH` stay on loopback: Docker opens a published port past ufw.
