# 0004 Keep `store` and `model` whole

- **Status:** Accepted (2026-11-19)
- **Date:** 2026-11-19

## Context

The restructure's targets are no package over about 3,000 lines and no struct
over a few dozen methods, unless a record says why. Two packages miss both:

- `internal/store`: 5,715 lines in 36 files, one `Store` with 289 methods.
- `internal/model`: 5,475 lines in 47 files, 85 types.

### How the map was made

As for ADR 0003, the map is measured with type information (`go/packages`),
not read off:

- **Per declaration:** for each of the store's 316 declarations, the store
  functions it calls, whether it opens a transaction, and the tables it
  touches. Tables are counted by the model types it names and by the table
  names in its SQL, package-level statements included.
- **Callers:** for each `Store` method, the packages that call it.
- **Domains:** the 55 tables are grouped into seven domains, and each
  declaration is placed in the domain of most of the tables it touches.

| Domain     | Tables                                                                                                                                                    | Lines | Exported methods | Called mostly by                                       |
| ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ---------------- | ------------------------------------------------------ |
| accounts   | users, their fields, factors, recovery codes, sessions, throttles, resets, verifications, login codes, waiting sign-ins                                   | 863   | 59               | `signin` (46), `api/users`, `api/fields`               |
| roles      | user roles, their members, inheritance and API scopes                                                                                                     | 202   | 14               | `api/users`, `api/roles`, `federation`                 |
| oauth      | applications, APIs and scopes, introspectors, codes, requests, pushed requests, refresh tokens, revocations, consents, devices, DPoP proofs, signing keys | 1,151 | 65               | `oauth` (30), `api/apis`, `signin`, `api/applications` |
| federation | social providers, identities and logins; SSO connections, identities, logins, domains, SAML assertions                                                    | 466   | 39               | `federation` (21), `api/sso`, `api/social`             |
| admin      | administrators, their roles, sessions, security settings, MFA factors                                                                                     | 411   | 32               | `admin` (18), `api/admins`, `api/adminroles`           |
| settings   | organisation and logo, login flows, languages and translations, OTP and mail settings, server secrets                                                     | 552   | 30               | `api/languages`, `signin`, `api/mailsettings`          |
| audit      | the activity log                                                                                                                                          | 202   | 8                | `api/activity`                                         |

The rest (515 lines, 27 methods) touches no table itself: the cache helpers,
`translate`, the sweep's driver, and wrappers.

### What crosses domains

Calls cross domains freely, but what matters is **transactions**. 47 of the
57 transactions stay inside one domain. The other 10 cross domains. The first
six are what the security invariants rest on; the last four keep what is left
consistent when something is deleted:

| Transaction            | Domains                            | What it keeps true                                                                                       |
| ---------------------- | ---------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `VerifyEmail`          | accounts, oauth, federation, audit | "The first proof of an address ends what came before it" (`clearBeforeProofIn`)                          |
| `ResetPassword`        | accounts, oauth, federation, audit | the same, and "Whatever ends a user's access as a whole … goes through `revokeUserRefreshIn`"            |
| `ChangeUserPassword`   | accounts, oauth                    | "An access token ends with what it was issued in"                                                        |
| `SignOutUser`          | accounts, oauth                    | the same                                                                                                 |
| `revokeUserSessions`   | accounts, oauth                    | the same                                                                                                 |
| `DeleteUser`           | accounts, oauth, roles             | the same                                                                                                 |
| `DeleteApplication`    | admin, oauth, roles                | a deleted application takes along its roles and their grants, and the administrators' roles scoped to it |
| `DeleteAPI`, `SaveAPI` | oauth, roles                       | a removed scope leaves no role holding it                                                                |
| `DeleteLoginFlow`      | oauth, settings                    | the applications on a deleted flow fall back to the default one                                          |

Each is one `Transaction` composed from `…In(tx, …)` helpers that take the
transaction's `*gorm.DB`: `revokeTokensIn`, `revokeUserRefreshIn`,
`revokeUserSessionsIn`, `familiesRevokedIn` and `clearBeforeProofIn`. The
invariants say "in the same transaction", and they are true because those
helpers are unexported: nothing outside `store` can open a transaction, or
leave a revocation out of one.

### `model`

`model` is the vocabulary: types, their validation and the rules that are
part of what a thing is. Its 42 files reference each other's types 149
times. `model.go` (`Base` and the shared types) is referenced from 34 files,
`user.go` and `application.go` from 14 each. Splitting it by domain would put
`Application` in one package and the `AuthorizationRequest`, `Consent`,
`RefreshToken` and `UserRole` that name it in others, and the user and
session types in the middle of every one.

## Decision

**Keep `store` one package with one `Store`, and `model` one package**, as a
deliberate exception to the size targets.

- **What this records:** the exception, and why — the cross-domain
  transactions are the security invariants', and they hold because the
  helpers that compose them are private to one package.
- **Inside `store`:** the domains stay as files, one per table or subject, as
  today.
- **Owed:** a new cross-domain transaction is written as one more
  `…In(tx, …)` composition inside `store`, never as a transaction a caller
  assembles.

## Alternatives rejected

**(a) A subpackage per domain** (`store/accounts`, `store/oauth`, …, over a
shared `store/tx`). Each of the 10 transactions above then calls helpers from
several subpackages. Those helpers become exported functions taking a
`*gorm.DB`, so any package that imports them can open a transaction and choose
which revocations to put in it. Today only `store` can. That trades the
invariants' strongest guarantee for package size.

**(b) One package, one struct per domain** (`st.Accounts.UserByID`,
`st.OAuth.RefreshToken`, …), sharing the unexported helpers.

- **What it gains:** the transactions are untouched, and the largest struct
  drops to 65 methods.
- **What it costs:** 463 call sites in 74 files change.
- **What it doesn't do:** callers gain no compile-time boundary, since every
  domain is still reachable through one `*Store`.

It is churn for readability alone, and can be revisited later without
undoing anything here.

**(c) Splitting `model` by domain.** The cross-references above would become
import cycles, or would push most types into one shared package, which is
`model` again.

## Consequences

- **The restructure ends with Stage 6.** Nothing in `store` or `model` moves.
- **The size targets are met everywhere else.** `store`, `model` and `signin`
  (ADR 0003) are the recorded exceptions.
- **A reviewer can check one rule:** every transaction that spans more than
  one domain is in `store`, built from `…In` helpers, and named in this
  record's table or added to it.
- **If `store` grows a domain that never shares a transaction** (a new,
  self-contained subject), that domain can still become a package of its own
  without revisiting this record.
