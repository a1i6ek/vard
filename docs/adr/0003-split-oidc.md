# 0003 Split `internal/oidc` by what it does

- **Status:** Proposed. Waiting for the user's approval before any code moves.
- **Date:** 2026-11-19

## Context

`internal/oidc` is one package: 40 files and 10,171 lines, with one `Service`
of 226 methods and 15 fields. It holds the authorization server, OpenID
Connect, how a person signs in, social and SSO federation, key rotation and the
outbound HTTP client. 17 packages under `internal/api` hold a
`provider *oidc.Service` and call 79 of its methods.

### How the map was made

The call map below is measured, not read off. A small analyzer (Go's own
`go/parser`, run outside the module) records, for each of the package's 549
top-level declarations, the declarations of the package it uses and the
`Service` fields it reads. It counts a method only where it is called on the
receiver (`s.X`), so that `s.store.AnswerDevice` isn't mistaken for
`s.AnswerDevice`, and skips names after an imported package (`http.Client`).
Each declaration is then assigned to a proposed package, and every use that
crosses packages is an edge.

### What the first cut shows

Assigning whole files to the packages the plan suggested gives cycles
everywhere. Most of them are artifacts of where small helpers happen to sit:

- **`New`** (`provider.go`) builds everything, and it is the composition root's
  job (Stage 4), not an edge between packages.
- **A shared vocabulary** that every part uses:
  - the error catalog (`problem`, 43 codes in `errors.go`, `ErrRequestExpired`
    used by four parts);
  - who is calling (`Client`);
  - the provider's addresses (`Page…` and `Path…`, spread over ten files);
  - `withQuery`, `Issuer`, the activity log (`record`), `Organization`, and
    client lookup (`ApplicationByClientID`).
- **Every symbol that crosses a boundary is small, 3 to 34 lines**, using at
  most the store, the clock, the issuer and the account URL.

Separating those leaves three real cycles:

| Cycle               | One way                                                                                                           | The other way                                                                                                                 |
| ------------------- | ----------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| oauth ⇄ signin      | `Session` (the type), `holdStepUp`, `waitForStepUp`, `sendVerification`, `CodeChallenge`, `Registration`, `proof` | **`pending`** (reading the waiting authorization request, 8 callers), `redirectError`, the `grant` type, `ErrBrowserRequired` |
| federation ⇄ signin | `admit`, `hold`, `flowFor`, `startSession`, `startFederatedSession`, `sendVerification`                           | `ssoRequiredFor` (13 lines), `linkSSOIdentity` (11), the `ssoPerson` and `SSOButton` types                                    |
| federation ⇄ oauth  | `Continue`, `pending`, `confirmation`                                                                             | `refused` (3 lines)                                                                                                           |

Sign-in never calls `finish` or `Continue` itself: the API handler continues
the authorization once a person is signed in. Only federation's
`SocialLanding` calls `Continue`. So the "authorize and sign-in call each
other" problem is, in the code, **sign-in reading the waiting authorization
request**, and **authorize holding a sign-in for a step-up**.

## Options

**(a) `signin` depends on `oauth`; `oauth` defines a small interface for what
it needs from sign-in.** `oauth` uses `Session` as a type throughout
(`sessionMeets`, `accessClaims`, the Consent and device answers), and an
interface can't hold a type, so `Session` would still have to move below both.
`oauth` would then call `HoldStepUp`, `WaitForStepUp` and `SendVerification`
through an interface that only `signin` implements: an indirection with one
implementation, which the plan's rule 4 advises against.

**(b) A lower package owns the authorization request's lifecycle, and both
depend on it.** `pending`, `redirectError` and the `grant` projection move down,
with the shared vocabulary. Then `signin` no longer needs `oauth`, and `oauth`
uses `signin` directly: authorize calls sign-in, which is the direction the
protocol has. No interfaces.

**(c) Leave `oauth` and `signin` in one package**, splitting only `outbound`,
`signing`, `oidc` and `federation` out. That's simpler, but keeps a package of
about 5,000 lines with the two concerns the security invariants most need to
keep apart.

## Proposed decision: (b)

```
           crypto · brand · config · model · store · cache · mail · i18n
                                   │
         ┌──────────────┬──────────┴──────────┬─────────────────┐
     outbound        signing               authreq                │
   (dial rules)   (key set, JWKS,   (the authorization request:  │
                    rotation)         pending, redirectError,     │
                                      grant; the vocabulary:      │
                                      problems, Client, pages,    │
                                      paths, Issuer, record,      │
                                      Organization, clients)      │
                                           │                      │
                                        signin ◄──────────────────┤
                         (sessions, flows, register, reset,       │
                          verify, codes, second factors,          │
                          passkeys, throttle, account; and        │
                          ssoRequiredFor / linkSSOIdentity)       │
                                           │                      │
                                         oauth                    │
                    (authorize, PAR, token, device, DPoP,         │
                     registration, introspection, revocation,     │
                     Consent and Agreements)                      │
                         ┌─────────────────┴───────┐              │
                       oidc                    federation ─────────┘
          (discovery, ID tokens, userinfo,   (social; SSO over OIDC and
           logout, back- and front-channel)   SAML; SSO domains)
```

Arrows point from a package to what it imports. Every edge goes downward:
`oidc` imports `oauth` (ID token and DPoP algorithms), `signin` (logout ends
sessions), `signing`, `outbound` and `authreq`; `federation` imports `oauth`
(`Continue`), `signin`, `outbound` and `authreq`. Neither top package imports
the other. The simulation of exactly this assignment reports **no cycles**.

What moves to break the cycles, nothing else changing:

| Symbol                                                                                                                   | From                                                                                                    | To        | Size        |
| ------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------- | --------- | ----------- |
| `pending`, `redirectError`                                                                                               | `authorize.go`                                                                                          | `authreq` | 19, 8 lines |
| `grant`, `confirmation`                                                                                                  | `token.go`                                                                                              | `authreq` | 23, 7       |
| `refused`, `ssoRefusal`                                                                                                  | `sso.go`                                                                                                | `authreq` | 3, 3        |
| `ErrBrowserRequired`                                                                                                     | `twostep_setup.go`                                                                                      | `authreq` | 1           |
| `ssoRequiredFor`, `SSORequired`, `SSOButton`, `ssoPerson`                                                                | `sso.go`                                                                                                | `signin`  | 13, …       |
| `linkSSOIdentity`                                                                                                        | `sso_account.go`                                                                                        | `signin`  | 11          |
| errors, `Client`, `Page…`/`Path…`, `withQuery`, `Issuer`, `record`, `errorPage`, `Organization`, `ApplicationByClientID` | `errors.go`, `provider.go`, `organization.go`, `application.go`, and the files declaring a page or path | `authreq` |             |

The name `authreq` is the plan's. It holds more than the authorization
request, so a better name may suit it (`flow`, `provider`); see the questions.

### What each package holds (from the simulation)

| Package      | Lines (declarations) | Methods | `Service` fields it uses                                                                      |
| ------------ | -------------------- | ------- | --------------------------------------------------------------------------------------------- |
| `outbound`   | 49                   | 0       | none                                                                                          |
| `signing`    | 339                  | 4       | `keys`, `log`                                                                                 |
| `authreq`    | 295                  | 8       | `store`, `now`, `issuer`, `accountURL`, `log`                                                 |
| `signin`     | 2,756                | 100     | `store`, `now`, `mail`, `sealer`, `webauthn`, `pwnedURL`, `breachClient`, `accountURL`, `log` |
| `oauth`      | 2,179                | 46      | `store`, `now`, `keys`, `sealer`, `issuer`, `accountURL`, `log`                               |
| `oidc`       | 471                  | 13      | `store`, `now`, `keys`, `backchannel`, `issuer`, `accountURL`, `log`                          |
| `federation` | 2,224                | 55      | `store`, `now`, `sealer`, `social`, `ssoCache`, `lookupTXT`, `issuer`, `accountURL`, `log`    |

Each gets its own `Service` with only the fields in its row.

### `signin` stays whole, at about 100 methods

The plan asks for no struct over a few dozen methods. The obvious seam in
`signin` is second factors (`twostep*.go`, `passkeys.go`, `passkey_login.go`:
47 methods), but splitting there makes a real cycle. Sign-in holds a sign-in
for its second factor (`holdForSecondFactor`); the second factor then opens
the session and is counted against the same throttle (`openSession`,
`reserve`, `admit`). That shared count and shared session-opening is what two
security invariants rest on: "a second factor is guessed against the same
count", and "a flow that requires a second factor is met only by a session
that passed one". So `signin` stays one package, an exception this record
makes deliberately.

## Consequences

- The 17 handler packages take the services they use instead of one
  `*oidc.Service`, and `api/server.go`'s wiring builds each. That is the
  largest part of the change outside `internal/oidc`.
- `layers_test.go` gains `outbound`, `signing`, `authreq`, `signin`, `oauth`,
  `oidc` and `federation`, in the order above.
- `CLAUDE.md`'s invariants name functions that move (`newFederationClient`,
  `sessionMeets`, `pendingFor`, `ssoAccountFor`, …); each is updated to its new
  package, word for word otherwise.
- No function changes what it does. The moves above are relocations; if one
  needs more than that to compile, the pull request says so.

## Questions for the user

1. Approve (b)?
2. Accept `signin` at about 100 methods for the reason above, or split its
   `Service` into two structs inside the one package (sign-in and second
   factors, sharing the throttle and session code), at the cost of more
   wiring?
3. The low package's name: `authreq` (the plan's), or a name for what it holds
   (`flow`)?
4. Front-channel logout (`frontchannel.go`) and two social providers are due to
   be removed in the hardening plan's Phase 1. Do that before this split, so
   less code moves?
