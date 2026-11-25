# CLAUDE.md

## Overview

An OAuth 2.0 authorization server and OpenID Connect provider in Go, with
two SvelteKit apps under `web/`:

| App           | `make dev`            | `make docker-local`   | What it is                                       |
| ------------- | --------------------- | --------------------- | ------------------------------------------------ |
| `web/id`      | http://localhost:5173 | http://localhost:8080 | Hosted sign-in pages and each user's own account |
| `web/console` | http://localhost:5174 | http://localhost:8081 | The admin panel                                  |

One Go binary is the API: `:8080` (provider and account API) and `:8081`
(admin API). Each app is a SvelteKit Node server in front of it that renders
its pages and passes the API paths it calls on to it, so each app is one
origin and the browser never reaches the API directly. Locally everything is
plain http; on a server, TLS and routing are the infrastructure's, in front
of the apps (`DEPLOY.md`).
Postgres holds everything; Redis is an optional cache. `README.md` is the
overview for people; update it when a change makes a sentence in it untrue.

## Commands

| What                               | Command                                                                                                                |
| ---------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| Run everything (hot reload)        | `make dev` — API, id (Vite :5173), console (Vite :5174); logs in `.logs/`                                              |
| Run everything (production builds) | `make start` — the API and both apps built, on the same ports; `env=dev` is `make dev`                                 |
| Run only the API                   | `make run`                                                                                                             |
| Lint and unit-test everything      | `make check` (`lint` + `test`; each also as `-go` / `-web`)                                                            |
| Format everything                  | `make fmt` (`lint` only reports, never rewrites)                                                                       |
| Go: tests against Postgres         | `make test-integration` (every `TestLive…`, throwaway DBs; fails with no database)                                     |
| Every check, locally               | `make ci` (`check`, `test-integration`, `docker-config`, `vuln`, `audit`); CI runs `check`, `vuln` and `docker-config` |
| Build                              | `make build` (`build-api` → `bin/vard`, `build-web` → `node build`)                                                    |
| Generated files                    | `make generate` (both apps' API types)                                                                                 |
| Migrations                         | `make migrate-new name=x`, `migrate-up`, `migrate-down`, `migrate-status`, `db-reset`                                  |
| Benchmarks, vulnerabilities        | `make bench`, `make vuln` (Go), `make audit` (both apps' packages; ignores listed in the Makefile with their reason)   |
| Rename the project                 | `make rename name="New Name" slug=newslug` (see A brand, below)                                                        |
| Production stack on this machine   | `make docker-local` → http://localhost:8080, http://localhost:8081                                                     |
| A server                           | `make docker-env`, `make docker-up`, `make db-backup` (DEPLOY.md)                                                      |
| Everything else                    | `make` lists every target                                                                                              |

Project commands in `.claude/commands/`: `/check` (every check), `/preview`
(the app in a browser on spare ports), `/endpoint` (add an admin endpoint),
`/migration` (write a migration).

## Architecture

```
cmd/vard/                the commands: serve (the default), migrate, and admin
                         create (the first administrator from the shell); each
                         parses its flags and calls internal/app
internal/                everything else of the server; nothing outside this
                         module may import it. DDD layers (layers_test.go):
  platform/              the shared kernel, below the domain
    crypto/                building blocks that know nothing of this server
                           (they import only each other):
      jose                     JWT signing and verifying, and Sealer (secrets at rest)
      totp, recovery           time-based one-time codes; recovery codes
    brand, config          shared by all: the brand (name, slug, favicon,
                           every name built from them; each app has its own,
                           lib/brand.ts); settings from .env
  domain/
    model/                 what the server is about, with no database or HTTP:
                           one file per table; every model listed in model.All
  infrastructure/        the adapters, depending on the domain alone
    store/                 every query in the project, one file per subject
    database/              the Postgres connection and the migration runner
      migrations/            All(): every migration, compiled in, run by goose's
                             Provider (database/migrate.go). setup/ is version 1
                             (schema/up.sql, frozen, and seed.go); changelogs/ one
                             file per change after it, 00002_x.go is version 2;
                             reset/ takes them all down and up again (make db-reset)
    cache/                 Redis, two databases: "cache" (what every page
                           reads) and "sessions" — despite its name, only
                           administrators, clients, grants and the rate-limit
                           counts; sessions themselves are always read from
                           Postgres
    mail, passkey,          sending email; the WebAuthn relying party; the
    signing, outbound      signing keys (loading, rotation, the JWKS); the one
                           client for supplied URLs (dial rules)
  application/           the provider's use cases (ADR 0003), layered of their
                         own, built in order by transport/http/services:
    authreq                what every part shares: the waiting authorization
                           request (pending, redirectError, grant), the
                           problems (errors.go), Client, paths and pages
    signin                 how users sign themselves in: session.go,
                           register.go, reset.go, verify.go, account.go,
                           second factors, passkeys, the throttle
    oauth                  authorize, PAR, tokens, the device flow, DPoP,
                           registration, introspection, Consent and Agreements
    oidc                   discovery, userinfo, logout (RP-initiated and
                           back-channel)
    federation             social sign-in and SSO (OIDC, SAML, domains)
    admin                  signing administrators in, and what they may do
                           (package auth, until its own stage renames it)
  transport/http/        HTTP: server.go, the engine and the route table
                         (registerPublicRoutes, registerAdminRoutes);
    <subject>/             one package per subject the API answers about
                           (handler.go, request.go, response.go, validation.go;
                           Handler, built by New(deps Deps))
    services/              builds the provider's services, in order; each
                           handler keeps the ones it calls
    respond, validate,     what every handler shares: respond (answers and the
    query, session, audit, problem catalog; shared.go: problems several
    cors, csrf, ratelimit, packages use), validate, query, session (who is
    middleware, signedin,    calling, route guards), audit, cors, csrf,
    spec                     ratelimit, middleware; signedin (what account,
                             profile, steps and twostep share); spec (holds
                             each answer struct to its schema, spec.Match)
  app/                   the composition root: New builds everything from the
                         configuration (database, Redis, the rows a fresh
                         installation needs, the services, both servers);
                         Run serves and stops in order: the servers, the
                         workers (the back-channel sender finishing its
                         posts), then the pools.
                         Migrate and CreateFirstAdmin open the database the
                         same way. Nothing imports it but cmd
    apptest/               starts the whole server on throwaway Postgres and
                           Redis, for tests/integration
tests/integration/       the tests that run the whole server over HTTP
                         (TestLive…, one file per subject; live_test.go: the
                         browser and fixture helpers), on app/apptest
api/openapi/             the HTTP API's description, OpenAPI 3.1, the source of
                         truth (ADR 0011): account.yaml (the account API and
                         the OAuth/OIDC endpoints) and admin.yaml, each split
                         by tag into account/*.yaml and admin/*.yaml with
                         schemas.yaml; common.yaml the problem details. The
                         Go package embeds and bundles them, served at
                         /api/v1/<account|admin>/openapi.json; bundle/ writes
                         them out for Spectral (make lint-api, in make
                         check) and openapi-typescript, which generates each
                         app's lib/api/schema.d.ts (make generate)
i18n/                    the English that ships, read with go-i18n: id/*.en.json
                         for the sign-in pages, console/*.en.json for the admin
                         API's problems; Catalog formats any language's text.
                         Outside internal/ because the sign-in app reads the
                         same files (web/id/src/lib/i18n/messages.ts)
web/<app>/src/hooks.server.ts  the app's server: API paths passed on
                         (lib/server/api.ts), page headers (lib/server/headers.ts)
web/                     the two apps and nothing else: console and id, each
                         a project of its own — package.json, bun.lock
                         (hoisted, bunfig.toml), node_modules, Prettier
                         (prettier.config.js), Stylelint
                         (stylelint.config.mjs), Dockerfile — sharing nothing
web/<app>/src/lib/styles/  the app's styles, plain CSS: themes.css (the
                         tokens — every scale and both palettes, kept by
                         hand in each app), fonts.css (the Inter and
                         JetBrains Mono faces),
                         base.css (reset, typography), components.css (the
                         panel's primitives, both apps) and layout.css, and
                         app.css, which the root layout imports once
web/<app>/src/lib/assets/fonts/  the app's typefaces, each with its licence:
                         Inter and JetBrains Mono, in the four alphabets
web/console/src/lib/     api/, query/, components/ui/, components/<feature>/
web/id/src/lib/          api/, i18n/, state/, components/ui/ (primitives),
                         components/auth|account|layout/, constants.ts (the
                         API's field limits)
web/<app>/src/lib/testing/  expectAccessible: axe on a mounted component
web/<app>/src/lib/brand.ts  the app's brand (name, slug, mark, favicon from
                         lib/assets/, cookie names; the console's links and
                         pageTitle)
scripts/                 what the Makefile runs: setup, start, docker-local,
                         docker-env, db, db-backup, rename;
                         lib.sh (load_env) is shared by all of them
compose.yaml             the Docker stack (make docker-up, make docker-local), read
                         with compose.env (compose.env.example); never .env, the
                         server's own development settings
Dockerfile, web/<app>/Dockerfile  the API's image and each app's
DEPLOY.md                a server, from DNS to backups
docs/clients.md          the integration guide for each client profile; kept
                         true when a profile changes
docs/threat-model.md     assets, trust boundaries, attackers, and which
                         invariant below answers each threat; kept true
docs/adr/                one record per significant decision (README.md:
                         the format); never rewritten, superseded instead
```

**Layering.** The layout is domain-driven design in Go, four layers under
`internal/` and a composition root: `platform` (the shared kernel — `crypto`,
`brand`, `config`) → `domain` (`model`: the aggregates, entities and value
objects, knowing no database or HTTP) → `infrastructure` (`store`,
`database` + `migrations`, `cache`, `mail`, `passkey`, `signing`, `outbound`)
→ `application` (the use cases) → `transport/http`, wired only by `app`
and run by `cmd`. Each layer imports only what is to its left;
`TestEachLayerImportsOnlyWhatIsBelowIt` (`layers_test.go`) fails otherwise,
and a new folder under `internal/` is added to its `layerOf`. The
application layer is the provider's, layered among themselves (ADR 0003):
`outbound`, `signing` and `authreq` → `signin` → `oauth` → `oidc` and
`federation`, neither of which imports the other. Nothing but `store`
writes a query; nothing but `transport/http` answers a request. A handler
reads a request, asks a service or the store, and answers. Rules live in
`validation.go` and the model. The layers are folders and the import rule,
not interfaces: a service calls the store it is given, with no repository
abstraction between them.

**The origins.** Each app's `hooks.server.ts` passes the paths in its
`API_PATHS` (`src/lib/server/api.ts`) on to `API_URL` — `/oauth2`,
`/.well-known` and `/api/v1/account` for `web/id`, `/api/v1/admin` for
`web/console` — with the caller's address in `X-Forwarded-For` and anything a
client sent there dropped. The same hook carries the browser's requests and
the ones a load makes while a page is rendered, in `make dev` and in
production alike. `TestEveryRouteIsReachableThroughItsApp` holds every route
to those lists. In `compose.yaml` only the apps publish ports, on the
host's loopback; Postgres and Redis are on an internal network the
apps are not on. Cookies are host-only and nothing is
cross-origin.

**The apps.** Rendered on the server (`adapter-node`, `node build`): every
page arrives with its content and its `<title>`, then hydrates. Loads call
the API with the load's `fetch` on the app's own origin, which works on the
server and in the browser. A load that needs the request itself (cookies,
`Accept-Language`) is a `+layout.server.ts`.

## Security invariants

This is an identity server; these are not style. Keep each one when changing
code near it, and add a test when adding a case.

- **Secrets are stored hashed or sealed, never plain.** Codes, refresh tokens,
  sessions and reset links are 256 random bits kept as a SHA-256
  (`model.NewSecret` / `HashSecret`). Provider secrets, SMTP passwords and TOTP
  seeds are sealed with `jose.Sealer`, always bound to where they are kept —
  `SealFor(model.X.YWhere(), …)`, table, column and row, as AEAD additional
  data — so a value moved to another column or row does not open; the row's
  id is given before the value is sealed. A new sealed column gets its
  `…Where` method in `internal/domain/model/sealed.go`. `VARD_SECRET_KEY` is the
  current key, and `VARD_OLD_SECRET_KEYS` the ones before it, which still
  open what they sealed (each value names its key's id) until the
  `keys reseal` command seals it all again under the current one (ADR 0007); every sealed
  column is in `store.SealedColumns`, held to the model's binding by
  `TestEverySealedColumnIsResealed`. Recovery
  codes, device user codes and throttle keys are keyed hashes (`Sealer.Tag`)
  under the installation's own tag key (`server_secrets`, sealed like the
  rest). The one
  secret also kept recoverable is a refresh token that has just replaced
  another: sealed on the replaced row (`successor_sealed`) for the grace
  below, and cleared within one more — when read past it, at the family's
  next rotation, and by `KeepSwept` every `model.RefreshReuseGrace`. Nothing
  else may keep a token it hands out. A client secret a rotation replaced is
  kept hashed only for the overlap its administrator chose, at most
  `model.MaxSecretOverlap` (a week), and refused from then on
  (`Application.CheckSecret`); a rotation with none ends it at once.
- **Single-use means one atomic `UPDATE … WHERE used_at IS NULL`** checked by
  `RowsAffected`, never read-then-write. A replayed code revokes what the
  first use issued; a replayed refresh token revokes its family — unless it
  was replaced less than `model.RefreshReuseGrace` (30 s) before, when it is
  answered with the very token that replaced it, so every copy a client holds
  stays good and a family never has two live tokens. The retry is checked
  like any refresh — client authentication, then DPoP key —
  before it is given anything.
- **Session cookies are read through `session.Token`, `session.UserToken` or
  `session.SignInState`**, which treat a duplicated cookie (planted from a
  sibling subdomain) as none. Never `c.Cookie(...)` for a session. Over https
  every session and sign-in cookie is written by `writeCookie` as a
  `brand.HostPrefix` (`__Host-`) one — Secure, `Path=/`, no Domain — which no
  sibling can set at all, and is read under that name alone
  (`session.CookieMode`, set per listener); plain-http development uses the
  bare names, which a browser requires there.
- **Outbound requests to admin- or user-supplied URLs** (social, SSO, SAML
  metadata, back-channel logout, a client's `jwks_uri`) go through
  `outbound.NewFederationClient` (`internal/infrastructure/outbound`), which refuses
  private, link-local and metadata addresses at dial time. A logout token
  follows no redirect. A provider's or a client's keys are fetched again
  only for a token naming a key not held (or none), and at most once a
  `outbound.KeyRefetchEvery` (a minute) per key set (`outbound.KeySets`;
  `verifyPublished`, `providerKeys`), and a social
  id_token is checked against its kind's published keys
  (`SocialSpec.JWKSURL`) wherever the kind names them.
- **A key from outside is between `jose.MinRSABits` (2048) and
  `jose.MaxRSABits` (4096) long.** A DPoP proof brings a key of the sender's
  choosing, and verifying under a large modulus costs the server; `ParseJWK`
  measures the modulus itself, never its encoding, so zero bytes in front of
  a short key do not pass it, and `Thumbprint` takes n and e in their
  minimal encoding, so one key has one `jkt`.
- **Redirect targets** are exact-matched redirect URIs
  (`Application.AllowsRedirect`: a desktop app's `127.0.0.1` or `[::1]` URI
  on any port, RFC 8252 §7.3 — never `localhost`, which a resolver may point
  elsewhere and a desktop app may not register — and nothing else loosened;
  Expo Go's `exp://` only for a mobile (`native`) app with
  `allow_development_redirects`), or a path checked by
  `safePath` (Go) / `safeNext` (`web/id`). Never redirect to a raw parameter.
  A `request` object or `registration` parameter is refused
  (`request_not_supported`, `registration_not_supported`), never ignored: a
  client must not believe a redirect URI or state it put there enforced.
- **A user's sign-in is throttled, never locked, and a second factor is
  guessed against the same count.** Every answer — password, second factor,
  current password — is counted before it is checked (`ReserveSignIn`, rows
  locked in key order) against the caller's network and the address typed,
  keyed by a keyed hash so an address nobody has counts the same; past the
  free failures each attempt waits longer (`model.ThrottleWait`). A right
  password does not clear the
  count while a second factor is still to come; only the right second factor
  does, and a success only gives the network its one attempt back. The
  waiting sign-in is named by an httpOnly cookie, never by anything the page
  or the address holds. Administrators are throttled the same way, on a keyed
  hash of the username typed, counted before it is looked up
  (`internal/application/admin/throttle.go`); nothing is ever locked. A browser that has
  signed in to the account before carries a known-browser cookie
  (`model.KnownBrowser`: a keyed hash of a random id, its issue time and the
  account, for 180 days) and is counted on a budget of its own instead of
  the address's or username's, so a stranger's failures at an account keep
  only strangers waiting; the cookie is issued by every finished sign-in
  (`openSession`, the panel's handlers) and buys nothing but that budget.
- **A flow that requires a second factor is met only by a session that
  passed one of the account's own** (`model.MethodSecondFactor` in its
  `Methods`, never an amr value; an emailed code is `otp` and does not
  count), checked by `sessionMeets` and again in `finish` before any code.
  A signed-in session without one steps up in place: the waiting sign-in
  names it (`step_up_session_id`) and is finished only from a browser whose
  session cookie names that session, for the same user, by one conditional
  UPDATE that rotates its token and keeps its id and expiry. A step-up
  renews `authenticated_at` (auth_time, `max_age`); the flow's lifetime runs
  from `created_at`, and the new cookie is kept as the old one was
  (`user_sessions.remember`). `amr` says `mfa` only with the marker, and a
  passkey's answer says only what its own flags do (`model.PasskeyAMR`):
  `swk` for one that may be synced (BE), `hwk` otherwise, `user` when the
  person was verified, never `pin` or `fpt`, which WebAuthn does not tell.
  A passkey is registered requiring a discoverable credential and user
  verification, since any may sign in alone.
  An account without a factor sets one up in the waiting sign-in
  (`twostep_enrol.go`, `enrolling`): only for a flow that requires one, only
  at a proved address (the verification email goes first), never for a
  sign-in holding an SSO link, and the setup endpoints refuse a bearer token.
  Every factor confirmed, there or on the account page, mails the account
  (`confirmFactor`, `mailFactorAdded`). A first factor set up while signing
  in is confirmed under a per-user advisory lock, only while the account has
  none (`ConfirmUserFactor(…, onlyFirst)`), so two setups at once keep one.
  Allowing a device (`AnswerDevice`) meets the device application's flow as
  authorize does (`step_up_required`, then `POST /api/v1/account/step-up`,
  whose waiting sign-in names the application, `flowOf`; its Agreements
  step too, accepted with the answer, `accept_terms`), and a refresh
  family that never passed a factor gets `invalid_grant` once its flow
  requires one. "Not now" (`LeaveSignIn`) spends the waiting sign-in and its
  authorization request, each by one conditional UPDATE, and answers the
  client `access_denied`; a device's step-up names its device
  (`device_authorization_id`), which leaving denies while unanswered.
  Single sign-on meets the requirement by the account's own factor
  (`second_factor = local`, the default), or by the provider's when the
  connection trusts it (`provider`) and its signed answer shows a listed
  value — an id_token's `amr` (`accepted_amr`, read before userinfo fills
  anything in) or the verified assertion's `AuthnContextClassRef`
  (`mfa_contexts`) — which gives the session only the marker, never an amr.
  Otherwise it falls back to the account's own factor, and the fallback is
  logged (`user.sso_second_factor_fallback`).
- **Only the person, in their own browser, accepts terms or allows an
  application.** The Agreements, Consent and device answers, and every change
  to two-step verification, refuse an application's access token. A Consent
  or Agreements page is answered (and read) only by the session `finish`
  sent there, having met the flow: `finish` records it on the request
  (`authorization_requests.session_id`, by one conditional UPDATE) and
  `pendingFor` refuses any other as `request_expired`, so a session the
  flow would not make cannot finish through the page what authorize refused.
- **Every one-time thing is taken with one conditional UPDATE**: a pushed
  request's `request_uri`, a device's approval and its tokens, a passkey
  sign-in's ceremony; a DPoP proof's jti is recorded once. Device user codes are short, so they are kept as a keyed hash
  and entering them is rate-limited.
- **A DPoP-bound access token is never taken as a bearer token.** Userinfo,
  the account API and the admin API take it only sent as DPoP with a proof
  from its key for that very request (`presented`), and a token sent as
  DPoP counts as a token wherever an application's token is refused
  (`session.Bearer`). A proof at the token, PAR and userinfo endpoints
  carries this server's nonce (`DPoPNonce`: stateless, a keyed MAC over the
  issuer and a two-minute step, taken for that step and the one before),
  checked last, before its jti is spent, so the client may retry with the
  nonce sent in `DPoP-Nonce`. A proof's key never holds private members
  (`ParseJWK`), and base64url is decoded strictly, so one key has one
  `jkt`. A client with `dpop_bound_access_tokens` (always an SPA, a
  mobile app and a machine) is refused any token request without a proof.
  A code bound to a key (`dpop_jkt`, or a pushed request's proof, RFC 9449
  §10; `DPoPKey`) is exchanged only with a proof from that key, checked
  after the code is claimed, so a code tried without it is spent; a pushed
  request whose proof and `dpop_jkt` disagree is refused.
- **A client assertion proves its client once, by the client's own key.** A
  `private_key_jwt` client (RFC 7523 §2.2) is authenticated by an assertion
  signed with a key it registered, inline (`jwks`) or at its `jwks_uri`,
  with ES256, PS256 or RS256; whose `iss` and `sub` are its client_id; whose
  `aud` is the issuer (or the token endpoint, or the endpoint it was sent
  to); which has not expired and expires within
  `model.ClientAssertionLifetime` (five minutes); and whose `jti` is taken
  once (`UseClientAssertion`), only after every other check, so a forged
  assertion cannot spend a real one's (`verifyAssertion`). A client
  authenticates one way per request, and one that signs assertions keeps
  no secret (`Application.Normalize`).
- **An application's profile decides what it may use, on every save**
  (`model.Profile`, ADR 0005): web, spa, native (mobile), desktop or
  machine. `Application.Normalize` applies it and `Validate` refuses what
  it forbids, so editing a setting never takes an application outside its
  profile: a public profile has no credential, a desktop app is sent back
  to loopback or its own scheme, a mobile app never to loopback, and a
  machine client uses `client_credentials` alone, by `private_key_jwt`
  alone, with no secret, for one named `resource`, with tokens bound to a
  DPoP key and living at most `model.MaxMachineTokenLifetime` (fifteen
  minutes). Dynamic registration reads the profile from the
  metadata and refuses a combination none allows (`profileOf`).
- **Every client uses PKCE** (OAuth 2.1, RFC 9700 §2.1.1), confidential
  ones included: an authorization request without an S256 `code_challenge`
  is refused, and a code is never exchanged without its verifier, nor at
  all if it was issued without a challenge. There is no setting to turn it
  off.
- **Introspection answers a token's own client, or the API's.** A
  confidential client may introspect its own tokens; another's access token
  only when an administrator listed it for an API in the token's `aud`
  (`api_introspectors`, never inferred from being allowed to request tokens
  for that API). A refresh token only its own client. A public client is
  refused introspection and cannot be listed.
- **A token is exchanged only as an administrator's rule says** (RFC 8693,
  `internal/application/oauth/exchange.go`). A `token_exchange_rules` row names the
  API, the confidential application acting and the application whose
  tokens it may present, with the scopes it may carry; nothing else allows
  an exchange, never a grant on the application or access to the API. The
  subject is an access token this server issued or an ID token, live and
  not revoked, for an active user. What is issued holds only scopes the
  subject token held and the rule lists (`model.ExchangeScopes`; the target
  API's roles still apply), expires no later than the subject, nests the
  actor in `act` up to `model.MaxDelegationDepth` actors, carries the
  subject's `sid` and `fid`, its ancestors' `jti`s (`exchanged_from`) and the
  subject's first client (`subject_client_id`), so revoking any of them ends
  it (`revocationKeys`). Every exchange is in the activity log
  (`token.exchanged`).
- **Registering a client takes an initial access token** (an admin API token
  with `applications.write`); `/oauth2/register` is never open.
- **Passwords are Argon2id** (`model.HashPassword`; `VerifyPassword` takes
  bcrypt too and asks for a rehash, done at sign-in with a conditional
  UPDATE). Lengths are `model.MinUserPasswordLength`,
  `MinAdminPasswordLength` and `MaxPasswordLength`, mirrored in both apps'
  `constants.ts` (`TestThePagesKnowThePasswordLengths`).
- **Unknown accounts cost the same as wrong passwords** (`CompareDummyPassword`),
  and sign-in failures give one answer, so timing and wording reveal nothing.
  Registering a taken address, in a flow that requires a verified one, is
  answered as a fresh account would be (check your email), in the same
  time — both send their mail after the answer (`afterAnswer`) — and mails
  the owner a notice instead of a link, at most once every
  `model.RegisteredAgainNoticeEvery` (`ClaimOnceEvery`); registering again
  answers the same either way (`registeredAgain`). A flow that signs a new
  account in at once cannot hide it and answers `email_taken`.
- **No other site calls the cookie-authenticated APIs.** CORS
  (`internal/transport/http/cors`) opens only the provider endpoints a browser app
  calls (token, PAR, userinfo, revocation, introspection, `.well-known`), to
  any origin and never with credentials; every other path tells every origin
  nothing, since each app shares its API's origin. CSRF (`internal/transport/http/csrf`)
  takes a state-changing request only from the app's own origin, compared
  exactly: no wildcards, no suffix matching. State-changing requests must be
  JSON.
  `web/id` turns SvelteKit's own CSRF check off (`csrf.trustedOrigins: ['*']`)
  because the provider takes form posts from other origins and from none
  (token endpoint, SAML ACS, Apple); so it never gets a form action or a
  `+server` route (`TestTheIDAppLetsTheProviderTakeFormPosts`).
- **A logout without `id_token_hint` asks first.** Anyone can link to
  `/oauth2/logout`, so a hint-less one from a signed-in browser goes to the
  `/logout` confirmation page, and only its JSON call to
  `/api/v1/account/logout` (`Confirmed`) signs out. A `logout_hint` naming
  anyone but the browser's user (`hintNames`: subject, or email in any
  case) signs nobody out, with or without `id_token_hint`.
- **A SAML response signs in only the browser that started it, once.** The
  ACS POST comes cross-site without cookies, so it only holds the response
  on the sign-in its `RelayState` names (never the state cookie's value) and
  answers 303; the GET that follows brings the state cookie and does every
  check. Only the HTTP-POST binding (a `SAMLart` would make the library fetch
  from the metadata outside `outbound.NewFederationClient`), no SHA-1 signatures
  (inside an encrypted assertion too, `checkEncryptedAlgorithms`), no
  RSA-1_5 key transport, a bearer confirmation, and each assertion ID once
  (`saml_assertions`). Every assertion, encrypted ones too, names its SAML
  `Issuer`, carries its own signature (not only the response's), and holds
  no `NameID` outside SAML's namespace, checked before `crewjam/saml` parses
  (`checkSAMLAssertions`; GO-2026-6909 has no fixed release, so `make vuln`,
  in CI too, ignores it by name, `VULN_IGNORE`; ADR 0009). A `NameID` is
  an address only when its format is `emailAddress`. Metadata read from an
  address is read again daily (`KeepMetadataFresh`); a read that fails, or
  names another entity ID, keeps the last good copy.
- **An OpenID Connect SSO answer names the provider it came from.** A
  callback whose `iss` (RFC 9207) is not the connection's issuer is refused,
  and so is one without it from a provider whose discovery promises it
  (`authorization_response_iss_parameter_supported`), before the code is
  exchanged (`completeOIDC`). A connection may authenticate there with an
  assertion signed by its own key (`client_auth = private_key_jwt`, the
  sealed `sp_key`), published at `/oauth2/sso/:slug/jwks`, for one
  minute and that token endpoint.
- **An SSO connection signs in only addresses it may**: its own domains, or —
  without domains — any address at a domain no other connection holds.
- **An OpenID Connect SSO provider vouches for an address only by saying
  so.** An `email_verified` of true proves it; false, or anything not a
  boolean, never does; a missing one only on a connection that turns
  `assume_email_verified` on (off by default, OIDC only) — `vouchedFor`
  in `internal/application/federation/sso_oidc.go`. It speaks for the `email` claim alone: an
  address mapped from any other (`upn`, `preferred_username`, …) has it
  unsaid (`verifiedClaim`). An address not vouched for is refused
  (`sso_no_email`) and makes or links no account.
- **An SSO connection takes over an existing account only at a proved
  domain.** Linking needs `matching = link`, listed domains (validation and
  `ssoAccountFor` both refuse it without), and an `sso_domain_verifications`
  row proved by its DNS TXT record.
  The first link to an account with a second factor waits for that factor:
  the identity is held on the pending sign-in (`link_connection_id`) and
  made only by `SubmitSecondFactor`, while the connection is still on
  (`linkHeld`), and the provider syncs nothing until then.
- **The first proof of an address ends what came before it.** A reset or
  verification link that first marks an account verified also deletes, in
  the same transaction (`clearBeforeProofIn`), its social and SSO
  identities, factors, recovery codes, consents and waiting sign-ins, and
  ends its sessions, refresh tokens, login codes and devices — whoever
  registered the address first may not have been its owner. Social sign-in
  never makes an account for an address the provider has not verified. A
  verification link proves the inbox, not who chose the password, so one
  that would give anything — a first proof, or a new address — is taken only
  with the account's password or from a browser signed in to it
  (`heldByTheirOwner`), checked before the link is spent and counted like a
  sign-in; the owner of an address somebody else registered proves it with a
  reset instead, which replaces the stranger's password.
- **A back-channel logout notice is owed from the moment the session
  ends** (ADR 0008). `revokeUserSessionsIn`, which every way of ending a
  session goes through, queues a `logout_deliveries` row per application in
  the same transaction; `oidc.KeepTelling` claims due rows with `FOR UPDATE
SKIP LOCKED` under a lease (`store.ClaimLogouts`), so instances never send
  one at once, and retries with `model.LogoutRetryAfter` for
  `model.LogoutRetryFor`. A refused or abandoned notice is written to the
  activity log in the transaction that gives it up. A session that expired
  on its own notifies nobody.
- **An access token ends with what it was issued in.** It carries `sid`
  and `fid`; whatever ends a session, a refresh family or a grant writes a
  `token_revocations` row in the same transaction (`revokeTokensIn`,
  `revokeUserRefreshIn`, `familiesRevokedIn`), and `verifyAccessToken` —
  userinfo, introspection, the account and admin APIs — refuses a token any
  of whose keys was revoked at or after its `iat`. Read from the database,
  never a cache. `aud` is never empty. A session ended on purpose — by its
  owner on the account page or by an administrator (`EndUserSession`) —
  takes the refresh families issued in it along (`revokeSessionRefreshIn`);
  a logout (`RevokeUserSession`) leaves an application's `offline_access`,
  which outlives the browser's session (OpenID Connect Core §11). Whatever
  ends a user's access as a whole — a password change or reset, a sign-out
  everywhere, turning the user off, deleting them — goes through
  `revokeUserRefreshIn`, which also writes `model.RevokedUser`, so every
  access token of theirs ends, a long-expired session's included, and
  turning the user on again brings none of it back.
- **An administrator's session is new once its second factor passes.**
  `PassAdminSessionMFA` ends the session the password opened and starts its
  successor in one transaction, and the handler sets the new cookie, so a
  token seen before the second factor is worth nothing after it. A session
  unused for `AdminSecurity.IdleTimeoutMinutes` is refused (`last_seen_at`,
  written at most once a `model.SeenEvery`). A passkey answers only the
  ceremony its session was offered (`passkey_ceremony`), spent on the first
  answer, and is counted against the lockout as a code is.
- **The panel is set up once.** While no administrator exists, the setup
  page makes the first, a super admin; `CreateFirstAdmin` counts them under
  an advisory lock in the transaction that makes it, so racing setups make
  one and any after is refused. Whoever reaches a fresh panel first sets it
  up, so an installation is set up as soon as it is up.
- **Nothing internal reaches a browser.** Every answer is a defined problem;
  anything else goes through `respond.Failure`, which logs it and answers 500.
  The request log records the path, never the query string.
- **An uploaded picture is what its bytes are.** The organisation's logo is
  decoded by `model.NewOrganizationLogo` — PNG or JPEG only, never SVG, which
  would run script on the apps' origin; at most 512 KB and 4096 pixels a
  side — and served with its stored type and `nosniff`.
- **Every response carries security headers** — the API's from
  `middleware.SecurityHeaders`, each app's (`Permissions-Policy` among them)
  from `src/lib/server/headers.ts`. HSTS is sent by both over https
  (`SecurityHeaders(https)`, `setPageHeaders(headers, url)`).
  A rendered page holds the reader's data, so every page is `no-store`.
- **The apps run no script they did not ship.** Their Content Security Policy
  is SvelteKit's `csp` option in each `vite.config.ts`, sent as a header:
  `script-src 'self'` plus a fresh nonce for SvelteKit's own scripts, and
  `frame-ancestors 'none'`. Never add `'unsafe-inline'` or
  `'unsafe-eval'` to scripts, an inline `<script>` to `app.html`, or an outside
  origin to `connect-src`; a new external resource is a deliberate change to
  that list.

## How things are added

**An endpoint.** The method in its package under `internal/transport/http/`, keeping the
four files; a package's `Handler` is built by `New(deps Deps)`, `Deps` naming
each thing it is given (the store, a service, the log); mounted in the one
route table, `internal/transport/http/server.go`, behind `session.Require` and
the permission it needs. A route under `managed` (`session.RequireAny`) also
takes an admin-cli token; a route for a person alone goes under `signedIn`.
Describe it in `api/openapi/<account|admin>/<tag>.yaml` (referenced from the
root file) in the same change: `TestTheDescriptionHasEveryRouteAndNoOther`
fails on a route the description lacks, or a path nothing serves. A new
answer struct adds its `spec.Match` pair to the package's `types_test.go`; a
new problem code goes in `common.yaml`'s `ProblemCode`
(`TestTheDescriptionNamesEveryProblem`). Breaking a v1 contract needs a new
version (ADR 0011). `/endpoint` walks through it.

**An error.** It is sent as problem details (RFC 9457,
`application/problem+json`): `type` (`<issuer>/problems/<code>`), `title`,
`status`, `detail` (the English sentence) and, as extensions, `code` and
`params`, which the apps translate. The OAuth endpoints (token, PAR,
revocation, introspection, device authorization, registration) keep RFC
6749's `error` and `error_description`. Never a sentence in Go. Define the
problem beside the handler —
`var taken = respond.Define(http.StatusConflict, "language_code_taken", respond.Admin)`
— or in `internal/transport/http/respond/shared.go` if several packages use it. Answer with
`respond.Fail(c, taken)` or return `taken.With("code", code)` (a write that
may hit a unique value: `respond.Taken`), and add
`error.language_code_taken` to `i18n/<app>/server.en.json` for each app
it is for (`Public` → `id`, `Admin` → `console`, `Both`). A parameter in the
sentence is listed in `TestErrorParametersAreSent`. A malformed id is answered
like a missing one: `*_not_found`. The one exception is `respond.Invalid`,
which passes on a model's own validation sentence.

**A model.** A struct in its own file in `internal/domain/model`, embedding `Base`,
registered in `model.All()` — bump the count in `TestAllListsEveryModel`. A
plain `string` column is `not null;default:''`, a `bool` or `int` is
`not null`, anything that may be absent is a pointer. Nothing is soft-deleted.
A column holding one of a set of values carries a named
`check:chk_<table>_<column>,<column> IN (…)` listing exactly the Go constants
(`TestEachCheckListsWhatGoAccepts`); a column naming another row has its
relation with a deliberate `constraint:OnDelete:` and an index; JSON is
`type:jsonb;serializer:json`. Anything with an `ExpiresAt` goes in the
sweep's `expiring` list (`TestEverythingThatExpiresIsSwept`).

**A migration.** `internal/infrastructure/database/migrations/setup` builds a fresh database from
`schema/up.sql`, written out once and frozen, and seeds it with named
columns (`seed.go`). Every change after it is a changelog, its own file in
`internal/infrastructure/database/migrations/changelogs` (`make migrate-new name=x`; `/migration`), whose
statements are written out, never read from the models, and whose `down`
undoes it. `TestLiveTheMigrationsMakeWhatTheModelsDescribe`
fails when a model and the migrations disagree — a column, a constraint or an
index — so a model change and its migration land together. Test the upgrade
of an existing database, not just a fresh one.

**A permission.** A constant and catalog entry in
`internal/domain/model/admin_permission.go`, routes guarded with `session.Can` or
`session.CanAnywhere`, and the name added to `AdminPermissionName` in
`web/console/src/lib/api/types.ts`. `store.EnsureSystemAPIs` makes it an
admin API scope on the next start.

**An activity log entry.** `h.audit.Record` / `RecordWith` in the handler, then
a sentence, icon and category in
`web/console/src/lib/components/activity/actions.ts`. If the log can name the
target, say who may see that name in `maySeeName`
(`internal/transport/http/activity/handler.go`).

**A setting the outside world sees.** Follow `internal/transport/http/organization/`: the
panel writes it, the discovery document publishes the agreements
(`internal/application/oidc/provider.go`), and the sign-in pages read the rest from
`GET /api/v1/account/organization`, which takes no session — so only what a
stranger may see goes in `PublicOrganization`.

**A setting that decides how people sign in.** Like the organisation's or
`admin_security`: a singleton row seeded from configuration on first start,
owned by the panel afterwards, read where it matters on each request.

**A social provider.** One entry in `model.SocialSpecs`. **A login step.** One
entry in `model.LoginStepSpecs`; `Implemented` says whether the server runs it.

**A cached read.** Only in the store, with `cached(ctx, s, group, field, load)`,
only for what every request asks for and almost nothing changes. A new group
is a constant in `internal/infrastructure/cache` listed in `cache.Groups`. Every write calls
`s.forget(ctx, group)` (or returns `s.forgetting(ctx, err, group)`) after it
commits. Cache a projection, never a model carrying a secret, and add it to
`TestCachedTypesSurviveJSON`. Never cache users, sessions, codes or refresh
tokens: a session is read from Postgres on every request, so ending one holds
on every server at once, even one that cannot reach Redis.
Everything must work with a nil cache.

**A brand.** Each application has its own: the server's in `internal/platform/brand`
(Go constants and `favicon.svg`), each app's in `src/lib/brand.ts` (`BRAND`,
the cookie names built from `BRAND.slug`; the console's Help links and
`pageTitle` too) with its favicon in `src/lib/assets/favicon.svg`. Each may
name the product, draw its mark and pick its favicon as it likes; the slug
must be the same in all three, because the server reads back cookies the apps
name, which `TestTheAppsBuildTheSameNames` checks. A new field goes in the
app's `BRAND` and is read as `BRAND.field`; a new asset goes in
`src/lib/assets/` and is imported there, so a build fingerprints it. A new
name for everything is `make rename name="New Name" slug=newslug`, which also
rewrites what has to spell it (go.mod, `cmd/<slug>`, the environment prefix,
scripts, the compose and Docker files, CI, docs). An installation's own name, app logo, terms
and privacy links are not a brand: an administrator sets them in Settings
(the logo is an uploaded file, `model.OrganizationLogo`, never an address),
and they take the brand's place in the console's header and on the sign-in
pages. Where the apps and the API are reached is
configuration (`VARD_ISSUER`, `VARD_ACCOUNT_URL`, `VARD_ADMIN_URL`).

**A language** (sign-in pages only; the panel is English). English is the
only one that ships, and the default on a fresh installation. Every other is
added on the panel's Languages page (Settings), translated there, and offered
when it is turned on; one of them may be made the default the sign-in pages
show before somebody chooses. Nothing but `*.en.json` goes in `i18n/`. The
server formats text with go-i18n (`i18n.Catalog`, `{name}` placeholders, as
svelte-i18n does on the pages); a group may not hold only keys go-i18n
reserves (`other`, `description`, `one`, …), which `TestEveryKeyIsRead` catches.

**A panel page.** A folder under `src/routes/(panel)/`, whose layout signs
the administrator in and draws the header and sidebar. `+page.ts` checks the
permission (`requirePermission` / `requireAnywhere`, from `#lib/api/load.js`)
and fetches through the typed client in `lib/api/admin.ts`, wrapped in
`fromApi` — an endpoint's path and answer are written there once, never in a
load; the answer's type in `lib/api/types.ts` is an alias of its schema in
`api/openapi/admin/schemas.yaml`, generated into `lib/api/schema.d.ts` by
`make generate` (openapi-typescript; never edited by hand, and `make
lint-api` fails when it is stale); the package's `types_test.go`
(`spec.Match`) holds the schema to the Go struct. The API enforces the same permission on every call. `+page.svelte` seeds
a TanStack query from it. Query options in `lib/query/` (key in
`lib/query/keys.ts`), components in `lib/components/<feature>/`. A list's
filters live in its address: `setSearchParams` and `debounce` from
`#lib/utils/url.js`. Add the route to `lib/components/layout/sidebar/sections.ts`
with an `allowed` check. Build from `#lib/components/ui`; a page never writes a
colour, a height or a hover state itself.

**A form in the panel.** It posts to a form action, enhanced with `use:enhance`.
Actions live by subject in `src/routes/actions/<subject>/+page.server.ts`, so a
dialog posts to the same one wherever it is mounted, and each is a method of
the typed client — `update: action(usersApi.update)` — or, where one save is
several calls, `act(event, …)` (`#lib/server/actions.js`). The form sends the
method's arguments as its `input` field, and `submit()` from `#lib/forms.js`
builds the `use:enhance` callback that sends them and applies the answer to
the page's state and its queries. `act` refuses a form from another origin
before it calls (SvelteKit's own check is off under its dev server); the call
goes through the request's fetch, with the administrator's cookies and this
origin, and the cookies the API sets come back with the action's answer. A
change made with one click (a remove, a toggle) and a search box stay as they
are: a call from the browser, and the address's filters. The sign-in pages have
no form actions, ever: SvelteKit's origin check is off there.

**A colour, size or font.** Each app owns its styles in `src/lib/styles`,
plain CSS kept by hand — there is no generator and no preprocessor; a
component's styles are a plain `<style>`, written in modern CSS (nesting
included). The brand colour is `#0064e0`, with a darker
hover and a light and a dark subtle tint, in both palettes. The dark
palette is built on a `#0b0b0b` background: surfaces stay neutral grey,
stepping up by elevation (`#131313` alt, `#161616` fields, `#121212`
popovers) with `#1f1f1f` hairlines, never pure black. Themes stay CSS
custom properties, switched by `<html data-theme>` with no script. Colours,
spacing, radii, font sizes, line heights and shadows come only from tokens:
Stylelint (`make lint`) refuses a raw colour anywhere, and a raw px in any of
those outside `themes.css`. A fine adjustment below the spacing scale is a
`--nudge-N`. Both apps draw their text in Inter, bundled in
`src/lib/assets/fonts` (`fonts.css`) at 400, 500 and 700: `--font-sans` is
`'Inter', var(--font-system-sans)`, and `--font-mono` is `'JetBrains Mono'`
over the platform stack. Text has three weights: 400 for what is read (rows,
links), 500 for a field's label and what is typed (a field's value, a search
box), 700 for what is pressed or heads something (buttons, the chosen
option, titles). Corners, the way Kraken draws them: controls and
fields round at `lg` 8px (`--radius-control/-field/-mark/-tile`), boxes
and popovers at `xl` 12px (`--radius-box/-popover`), surfaces — cards,
panels, dialogs, tables, the sign-in shell — at `2xl` 16px
(`--radius-surface`, `--app-radius`), and tags, badges and chips are
pills (`--radius-round`). `sm` 4px survives for tiny joints only — an
inline code mark, a calendar day cell, a focus ring (`--radius-check`
for a checkbox, `--radius-key` for kbd, `round` for what is a circle:
an avatar, a switch's knob, a bar's ends). A field is a filled box with no
border (`--color-field`, `-hover`, `-focus`, swapped inside menus and
dialogs), 52px (`--field-height`), its label floating inside it; focus is a
2px brand ring drawn inside its edge, and blame the same ring in red.

**A primitive both apps have** follows one spec, though each app keeps its
own copy. Anything shaped like a button wears `.control` from
`components.css`, with its size, variant (`solid`, `subtle`, `outline`,
`ghost`, `plain`, `link`) and palette taken from `control.ts`; a page never
styles a `<button>` of its own to look like one. `Button`, `LinkButton`,
`Spinner`, `Skeleton`, `Badge`, `Alert`, `Note`, `control.ts` and the four
token and style files are the same file in both apps: change one, copy it to
the other in the same change.

**An icon** (both apps). The Remix Icon font, as PocketBase uses it: the root
layout loads `remixicon/fonts/remixicon.css`, and an icon is its class on an
`<i>` — `<i class="ri-add-line" aria-hidden="true"></i>` (an icon that means
something on its own takes `role="img"` and an `aria-label` instead). Names are
on remixicon.com. Passed as a value an icon is that class, a string typed
`IconName` (`icon="ri-add-line"`, drawn `<i class={icon} …></i>`). Its size is
its font size: 16px in the panel and 18px on the sign-in pages (`--icon-size`, set in each app's `app.css`);
any other is `style:font-size`. `icons.spec.ts` fails on a class the font
lacks. `remixicon` stays pinned at 4.8.0, the last Apache-2.0 release: from
4.9.0 its licence forbids using an icon as a logo, which the sign-in pages'
shield mark does.

## Conventions

- **Text.** Sign-in pages: never a literal — `const t = useTranslator()` and
  `t('area.thing', { param })`, the key added to its group in `i18n/id/*.en.json`.
  Panel: the opposite — English in the markup; an error shown is the sentence
  the API sent (`messageOf` from `#lib/api/index.js`).
- **The project's own name is never spelled out.** It is written only in the
  brands: `internal/platform/brand` and each app's `src/lib/brand.ts`
  (`import { BRAND, pageTitle } from '#lib/brand.js'`).
  `TestNothingElseSpellsTheName` fails on it anywhere else in the code.
- **SvelteKit 3.** Imports from `src/lib` use `#lib` and name the file
  (`import { COOKIES } from '#lib/brand.js'`); there is no `$lib` and no
  `svelte.config.js` — configuration is `sveltekit({...})` in `vite.config.ts`.
  `resolve()` reads a leading slash as a route id, groups included
  (`resolve('/(panel)/users')`). `page.url` is read-only. Reload
  with `refreshAll()`, not `invalidateAll()`. Fetch in the page's `load`
  (`+page.ts`), never in `onMount`. Code that runs while a page renders runs
  on the server too: no `document`, `window` or `localStorage` outside an
  effect, an event handler or a `browser` check. What the server needs to do
  for the API is the API's job: a console form action only passes the form
  to it (A form in the panel). The theme is the
  reader's cookie, which `hooks.server.ts` writes into `<html data-theme>` as
  the page is rendered; no script applies it. A choice the reader makes (theme,
  language, the sidebar) is written and read through `#lib/utils/cookies.js`
  (`rememberChoice`, `readChoice`, `setChoice`): over https a `__Host-` cookie,
  read by its whole name, never matched by a pattern.
- **Comments** are few and short and say _why_ — a security reason, a race, a
  protocol rule — never what the next line does.
- **Names** are spelled out; identifiers use American spelling (`Normalize`,
  `Organization`), prose may be British.
- **Tests** are table-driven and named for what they prove ("a slug with
  spaces", not "TestCase3"). Tests needing Postgres are named `TestLive…` and
  skip without `VARD_TEST_DB_DSN`. Each has a database and a Redis prefix of
  its own, so a live test in `tests/integration` starts with `t.Parallel()` (eight at a time,
  `make test-integration`); only one that compares how long things take stays
  serial, because a loaded machine skews it, and one that changes anything
  package-wide (`model.SocialSpecs`, say), which another test would see. A new component that people use goes in
  the app's `components/a11y.spec.ts`, which runs axe on it in jsdom
  (`expectAccessible` from `#lib/testing/a11y.js`).
- **Formatting is enforced.** The hook in `.claude/hooks/format.sh` runs gofmt
  or the app's own Prettier on every file written; `./node_modules/.bin/prettier --write <path>` in the app
  fixes what `bun run lint` refuses.

## Gotchas and warnings

- **Never read `.env` or `compose.env`** — they hold the secret key and the
  stack's passwords. `.env.example` documents
  every variable.
- **Do not restart a running stack.** If anything answers on 8080, 8081, 5173
  or 5174, leave it alone; `/preview` raises a throwaway one on spare ports
  with its own database and Redis prefix, and takes it down afterwards.
- **Never edit a migration that has run anywhere.**
- **The browser opens the apps' ports, never the API's.** The API's URLs
  (issuer, admin URL) are the apps', and a path the API answers reaches it
  only if it is in that app's `API_PATHS`.
- **The local URLs are fixed by `scripts/start.sh`** (`make dev`, `make start`),
  which exports `VARD_ISSUER`, `VARD_ACCOUNT_URL`, `VARD_ADMIN_URL`,
  and `VARD_TRUSTED_PROXIES` over whatever `.env` says.
- **The API trusts the apps' servers and nobody else** to say who the
  caller is: loopback under `make dev`, the two containers' fixed addresses
  in `compose.yaml` — never a whole private range. The apps read the
  caller from the connection, or from `ADDRESS_HEADER` when the
  infrastructure in front sets one; the API's ports are never published. adapter-node's `BODY_SIZE_LIMIT` is the API's
  1 MiB, or it refuses a logo or a SAML response first.
- **The scripts read `.env` with `load_env` (`scripts/lib.sh`), the server
  with viper,** and the scripts export what they read, which the server then
  believes. A difference between the two is a different value depending on
  how the server was started — for `VARD_SECRET_KEY`, signing keys one of
  them cannot open. `TestTheScriptsReadDotEnvLikeTheServer` holds them
  together; change both or neither.
- **Postgres bounds every connection**: a statement past
  `VARD_DB_STATEMENT_TIMEOUT` (15 s) or a transaction idle past
  `VARD_DB_IDLE_IN_TRANSACTION_TIMEOUT` (60 s) is ended by the database.
  Migrations run under a session advisory lock, one instance at a time.
  `/readyz` is the orchestrator's probe (the database answers); `/healthz`
  says only that the process is up. With `VARD_DB_MIGRATE_DSN` (the
  schema owner's, as in `compose.yaml`) the server migrates as the
  owner and runs as `VARD_DB_DSN`'s role, which `MigrateAsOwner` makes and
  grants rows only — no DDL, not goose's table. A store query that needs more
  than SELECT, INSERT, UPDATE or DELETE fails there; it belongs in a
  migration.
- **Each app's address is built into it.** adapter-node no longer reads
  `ORIGIN` at run time: `vite.config.ts` takes `paths.origin` from `ORIGIN`
  when the app is built (compose passes `PUBLIC_URL` / `ADMIN_URL` as build
  arguments, `make start` the local ones). Built without it, an app guesses
  https from the headers and the console refuses its own forms over http;
  changing an address means building the images again.
- **Redis is optional.** Every path must work with a nil cache, and Redis is
  never the only copy of anything.
- **The shell is zsh.** An unquoted glob such as `--include=*.go` fails with
  "no matches found" — quote it.
- **TypeScript stays on 6** until SvelteKit, `svelte-check` and
  `typescript-eslint` support 7.

## Definition of done

A change is finished when all of this holds, and the summary says which checks
ran and their result — or which were skipped and why:

1. `make check` passes: formatting, go vet, staticcheck, both apps' lint and
   type-check, and every unit test.
2. If `compose.yaml`, `compose.env.example` or a Dockerfile changed: `make docker-config` passes.
3. If a migration, the store, hand-written SQL or a route changed:
   `make test-integration` passes.
4. Every new problem code is in the catalogs, every new key in every language,
   every new permission in `types.ts`.
5. New behaviour has a test; a security case has one without exception.
6. A user-visible change was looked at in a browser (`/preview`).
7. `README.md` and this file say nothing the change made untrue.

## Git

- **Never commit, push, branch, tag or open a pull request unless asked** in
  this conversation. Leave changes in the working tree and say what changed.
- When asked to commit: never `--amend`, `--force` or `--no-verify`, and never
  rewrite history that has been pushed. Branch off `main` first if on it.
- `.claude/settings.local.json` is personal and is not committed.
