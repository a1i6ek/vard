# CLAUDE.md

An OAuth 2.0 authorization server and OpenID Connect provider in Go, with two
SvelteKit apps. `README.md` is the overview for people: keep it true.

| App           | `make dev`            | `make docker-local`   | What                         |
| ------------- | --------------------- | --------------------- | ---------------------------- |
| `web/id`      | http://localhost:5173 | http://localhost:8080 | Sign-in pages and accounts   |
| `web/console` | http://localhost:5174 | http://localhost:8081 | The admin panel              |

One Go binary serves `:8080` (provider and account API) and `:8081` (admin
API). Each app is a SvelteKit Node server that renders its pages and passes
its `API_PATHS` (`src/lib/server/api.ts`) on to `API_URL`, so each app is one
origin and the browser never reaches the API. Postgres holds everything;
Redis is an optional cache.

## Commands

| What                    | Command                                                           |
| ----------------------- | ----------------------------------------------------------------- |
| Run (hot reload)        | `make dev` (logs in `.logs/`); production builds: `make start`    |
| Lint and unit tests     | `make check` (`lint` + `test`, each also `-go` / `-web`)          |
| Format                  | `make fmt` (`lint` only reports)                                  |
| Live tests              | `make test-integration` (every `TestLive…`, throwaway databases)  |
| Everything, locally     | `make ci` (`check`, `test-integration`, `docker-config`, `vuln`, `audit`) |
| API types for the apps  | `make generate`                                                   |
| Migrations              | `make migrate-new name=x`, `migrate-up`, `migrate-down`, `db-reset` |
| Production stack        | `make docker-local`; a server: `DEPLOY.md`                        |
| Rename the project      | `make rename name="New Name" slug=newslug`                        |

Project commands: `/check`, `/preview` (the apps on spare ports), `/endpoint`,
`/migration`.

## Layout

```
cmd/vard/             serve, migrate, admin create, keys reseal; calls internal/app
internal/             layered, each importing only what is left of it
                      (layers_test.go; a new folder goes in its layerOf):
  platform/             crypto (jose, totp, recovery), brand, config
  domain/model/         one file per table, every model in model.All()
  infrastructure/       store (every query), database (+ migrations), cache,
                        mail, passkey, signing, outbound
  application/          use cases, layered by ADR 0003: authreq → signin →
                        oauth → oidc | federation (never each other); admin
  transport/http/       server.go (both route tables) and one package per
                        subject: handler.go, request.go, response.go,
                        validation.go; New(deps Deps)
  app/                  builds everything and runs it; apptest for tests
tests/integration/    the whole server over HTTP (TestLive…)
api/openapi/          the API descriptions, the source of truth (ADR 0011)
i18n/                 the English that ships: id/ (sign-in), console/ (problems)
web/<app>/            an independent SvelteKit app each; nothing shared
docs/                 clients.md, threat-model.md, adr/ (never rewritten)
```

A handler reads a request, asks a service or the store, and answers. Only
`store` writes queries; only `transport/http` answers requests; rules live in
`validation.go` and the model. No repository interfaces: a service calls the
store it is given.

## Security invariants

An identity server: these are not style. Keep each one, and add a test for
every new case.

- **Secrets at rest.** Codes, refresh tokens, sessions and links are 256
  random bits stored as SHA-256 (`model.NewSecret`). Provider secrets, SMTP
  passwords, TOTP seeds and keys are sealed with `jose.Sealer`, bound to
  table, column and row (`SealFor(model.X.YWhere(), …)`; a new sealed column
  gets its `…Where` in `model/sealed.go` and joins `store.SealedColumns`).
  `VARD_OLD_SECRET_KEYS` open what older keys sealed until `keys reseal`
  (ADR 0007). Short codes and throttle keys are keyed hashes (`Sealer.Tag`).
  Nothing keeps a token it hands out, except a just-replaced refresh token's
  successor for the reuse grace.
- **Single use is one conditional UPDATE** (`… WHERE used_at IS NULL`, checked
  by `RowsAffected`): codes, refresh tokens, PAR handles, device approvals,
  passkey ceremonies, client assertion and DPoP `jti`s. A replayed code
  revokes what it issued; a replayed refresh token revokes its family, unless
  within `model.RefreshReuseGrace` (30 s), when it gets the same successor.
- **Session cookies** are read only through `session.Token`, `UserToken` or
  `SignInState` (a duplicated cookie is none), written `__Host-` over https.
  Sessions are read from Postgres on every request, never cached.
- **Outbound calls to supplied URLs** go through
  `outbound.NewFederationClient` (no private, link-local or metadata
  addresses). A provider's or client's keys are refetched at most once a
  minute per key set. Keys from outside are 2048–4096-bit RSA, measured.
- **Redirects** are exact-matched URIs (`Application.AllowsRedirect`; a
  desktop app's `127.0.0.1`/`[::1]` on any port, never `localhost`) or paths
  checked by `safePath` / `safeNext`. A `request` object or `registration`
  parameter is refused, never ignored.
- **Every client uses PKCE (S256)**, confidential ones too.
- **Client profiles** (web, spa, native, desktop, machine; ADR 0005) decide
  what an application may use, applied on every save (`Normalize`,
  `Validate`). Machines: `client_credentials`, `private_key_jwt` only,
  DPoP-bound tokens of at most 15 minutes, one `resource`.
- **Client assertions** (`private_key_jwt`) are verified fully, then their
  `jti` taken once, last. One authentication method per request.
- **DPoP.** A bound token is never accepted as bearer; proofs carry the
  server nonce, checked last; a key-bound code needs a proof from its key.
- **Sign-in is throttled, never locked**, counted before it is checked
  (`ReserveSignIn`) against the network and a keyed hash of the address typed;
  a known browser (`model.KnownBrowser`) has its own budget. Administrators
  likewise, by username. Unknown accounts cost a dummy hash
  (`CompareDummyPassword`); failures and taken addresses answer alike.
- **Second factors.** A flow requiring one is met only by the account's own
  (`model.MethodSecondFactor`), checked in `sessionMeets` and again in
  `finish`; a step-up rotates the session's token in one UPDATE. `amr` says
  `mfa` only then; a passkey's `amr` is what its flags say. Setting up a
  first factor while signing in needs a proved address. Every new factor
  mails the account.
- **Only the person, in their browser,** answers Consent, Agreements and
  device prompts or changes two-step settings: these refuse tokens, and a
  pending page answers only the session `finish` recorded.
- **Introspection** answers a token's own confidential client, or one listed
  for the token's API (`api_introspectors`). **Token exchange** only as a
  `token_exchange_rules` row allows, never widening scopes or lifetime.
- **Registration** of clients needs an admin token with `applications.write`.
  People register within an application's sign-in under its flow, or on the
  account page under the default flow.
- **Passwords** are Argon2id; lengths in `model` mirror both apps'
  `constants.ts`.
- **No other site calls cookie APIs.** CORS opens only the provider
  endpoints, without credentials. CSRF takes state changes only as JSON from
  the app's own origin. `web/id` turns SvelteKit's origin check off for the
  provider's form posts, so it never has form actions.
- **Logout without `id_token_hint`** asks first (`/logout`). A `logout_hint`
  naming someone else signs nobody out.
- **SAML** holds the cross-site POST and checks it on the browser's GET;
  HTTP-POST binding only, no SHA-1, signed assertions, each ID once.
- **SSO** signs in only its domains; takes over an existing account only at
  a DNS-proved domain; trusts `email_verified` only as a real `true`.
- **The first proof of an address** ends everything before it
  (`clearBeforeProofIn`).
- **Ending a session, family, grant or user** writes `token_revocations` in
  the same transaction; `verifyAccessToken` checks it, never a cache.
  Back-channel notices are queued in that transaction too (ADR 0008).
- **Administrators** get a new session after their second factor; idle ones
  expire. The first administrator is made once, under an advisory lock.
- **Nothing internal reaches a browser:** every error is a defined problem
  or `respond.Failure` (logged, 500). Logs never hold query strings.
- **Uploads** are what their bytes are: PNG or JPEG logos only.
- **Headers and CSP:** security headers on every response, HSTS over https,
  `no-store` pages; CSP `script-src 'self'` plus a nonce — never
  `unsafe-inline`, `unsafe-eval` or a new outside origin unasked.

## Adding things

- **An endpoint:** a method in its package, mounted in `server.go` behind
  `session.Require` and its permission, described in
  `api/openapi/<account|admin>/<tag>.yaml` in the same change
  (`TestTheDescriptionHasEveryRouteAndNoOther`). New answer structs get a
  `spec.Match` in `types_test.go`. `/endpoint` walks through it.
- **An error:** `respond.Define(status, "code", respond.Admin|Public|Both)`
  beside the handler (or `respond/shared.go`), its code in `common.yaml`'s
  `ProblemCode`, its sentence as `error.<code>` in `i18n/<app>/server.en.json`.
  Problem details (RFC 9457); OAuth endpoints keep RFC 6749's `error`.
- **A model:** its own file, embedding `Base`, in `model.All()` (bump
  `TestAllListsEveryModel`). Strings `not null;default:''`; enums carry a
  `check:chk_<table>_<column>` listing the Go constants; foreign keys have
  a deliberate `OnDelete` and an index; anything with `ExpiresAt` is swept.
- **A migration:** the setup (`migrations/setup`, version 1, `schema/up.sql`
  and `seed.go`) is never edited once released; a change is a changelog
  (`make migrate-new`), written out, with a `down`. An index on a growing
  table is `CONCURRENTLY` via `addOutsideTransaction`. The drift test
  (`TestLiveTheMigrationsMakeWhatTheModelsDescribe`) holds models and
  migrations together. The history was squashed on 2026-11-26: older
  databases are refused (`ErrSquashedHistory`). `/migration` walks through it.
- **A permission:** `model/admin_permission.go`, `session.Can`, and
  `AdminPermissionName` in the console's `types.ts`.
- **An activity entry:** `audit.Record`, then its sentence and icon in the
  console's `activity/actions.ts`; who may see its target in `maySeeName`.
- **A cached read:** only in the store, `cached(...)` for what every request
  reads and almost nothing changes; every write `forget`s its group. Never
  users, sessions, codes or tokens. `cache.Overview` alone ages out (30 s).
  Everything works with no Redis.
- **A setting people sign in under:** a singleton row seeded from
  configuration, owned by the panel after.
- **A language:** only English ships (`i18n/**/*.en.json`); others are added
  in the panel.
- **A panel page:** a folder in `src/routes/(panel)/` with `+page.ts`
  (`requirePermission`, the typed client in `lib/api/admin.ts`), query
  options in `lib/query/`, components in `lib/components/<feature>/`, and an
  entry in the sidebar's `sections.ts`. List filters live in the address.
- **A panel form:** posts to an action in `src/routes/actions/<subject>/`
  with `use:enhance` and `submit()`, or calls the typed client for one-click
  changes.

## Conventions

- **Text:** sign-in pages never hold a literal (`useTranslator()`, keys in
  `i18n/id/*.en.json`); the panel is English in the markup, showing the API's
  sentence for errors.
- **The project's name** appears only in the brands (`internal/platform/brand`,
  each app's `brand.ts`; `TestNothingElseSpellsTheName`).
- **Styles:** plain CSS in each app's `src/lib/styles`; colours, spacing,
  radii, sizes and shadows only from tokens (Stylelint). Buttons wear
  `.control`. `Button`, `LinkButton`, `Spinner`, `Skeleton`, `Badge`, `Alert`,
  `Note`, `Panel`, `EmptyState`, `control.ts` and the four style files are
  the same file in both apps: change both. Fonts are Source Sans 3 and JetBrains
  Mono; icons are Remix Icon 4.8.0 classes (`icons.spec.ts`).
- **SvelteKit 3:** `#lib/...` imports naming the file, no `svelte.config.js`,
  `resolve()` route ids, `refreshAll()`, data fetched in `load`, no browser
  globals while rendering.
- **Comments** are few and short, and say why.
- **Names** are spelled out; identifiers in American spelling.
- **Tests** are table-driven and named for what they prove. Live tests are
  `TestLive…`, start with `t.Parallel()`, and skip without
  `VARD_TEST_DB_DSN`. New components join the app's `a11y.spec.ts`.
- **Formatting** is enforced: gofmt and each app's Prettier (a hook runs
  them on every write).

## Gotchas

- **Never read `.env` or `compose.env`.** `.env.example` documents them.
- **Do not restart a running stack** (8080, 8081, 5173, 5174): `/preview`
  runs one on spare ports.
- **Never edit a migration that has run anywhere.**
- **The browser opens the apps' ports only**; a path reaches the API only if
  it is in that app's `API_PATHS` (`TestEveryRouteIsReachableThroughItsApp`).
- **The API trusts only the apps' servers** to name the caller
  (`VARD_TRUSTED_PROXIES`).
- **Scripts and the server read `.env` alike**
  (`TestTheScriptsReadDotEnvLikeTheServer`): change both or neither.
- **Postgres ends** statements past 15 s and idle transactions past 60 s; the
  runtime role has no DDL.
- **Each app's address is built in** (`ORIGIN` at build time).
- **The shell is zsh:** quote globs.
- **TypeScript stays on 6** until SvelteKit supports 7.

## Done means

1. `make check` passes.
2. `make docker-config` passes if compose or a Dockerfile changed.
3. `make test-integration` passes if a migration, the store, SQL or a route
   changed.
4. New problem codes, translation keys and permissions are everywhere they
   belong.
5. New behaviour has a test; a security case always does.
6. A visible change was looked at in a browser (`/preview`).
7. `README.md` and this file are still true.

The summary says which checks ran and how they ended.

## Git

- Never commit, push, branch, tag or open a pull request unless asked.
- When committing: never `--amend`, `--force` or `--no-verify`; branch off
  `main` first.
- `.claude/settings.local.json` is personal and never committed.
