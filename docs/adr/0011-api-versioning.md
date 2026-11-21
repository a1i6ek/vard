# 0011 The HTTP API's description, and its versions

- **Status:** Accepted (2026-11-21)
- **Date:** 2026-11-21

## Context

The account and admin APIs were described only by the code and by each
app's hand-written types, held together by a test per answer. An outside
client had nothing to read, and a change could break one without anything
saying so. Errors were a home-grown shape.

## Decision

**OpenAPI 3.1 is the source of truth, and the API is versioned under
`/api/v1`.**

- **The description:** it lives in `api/openapi/`. `account.yaml` covers the
  account API and the OAuth and OpenID Connect endpoints; `admin.yaml` covers
  the admin API. Each is split into a file per tag, with schemas in
  `schemas.yaml` and the shared problem details in `common.yaml`.
- **Built in and served:** the binary embeds the files and serves each
  document bundled into one, at `/api/v1/account/openapi.json` and
  `/api/v1/admin/openapi.json`.
- **Tests hold the code to it:**
  - every route is in it, and every path in it is a route;
  - every answer struct has exactly its schema's fields (`spec.Match`);
  - every problem code is in `ProblemCode`.
- **Errors are problem details** (RFC 9457) except at the OAuth endpoints,
  which keep RFC 6749's `error` as their standards require.

**What counts as breaking**, and so needs `/api/v2` for that resource
rather than a change to v1:

- removing or renaming a path, a method, a field of an answer, or a value
  an enum may take;
- making an optional request field required, or adding a required one;
- narrowing what a field accepts (a shorter limit, fewer values), or
  changing its type or meaning;
- changing a success status, or the media type an answer is sent as;
- refusing, for every caller, something that used to be accepted.
  The exception is closing a security hole, which never waits for a
  version: it is written in DEPLOY.md's upgrade notes instead.

**Not breaking**, and so made in v1: new paths, new optional request
fields, new answer fields (a client ignores fields it does not know), new
problem codes, and new enum values in an answer marked as open.

**A deprecated spelling** stays for two minor releases or six months after
its replacement ships, whichever is longer. Throughout that time it is
marked `deprecated: true` in the description and named in DEPLOY.md's
upgrade notes, and the server sends `Deprecation` and `Sunset` headers
(RFC 9745, RFC 8594) on every answer that uses it. The two apps never use a
deprecated spelling, since they ship with the server.

## Alternatives rejected

- **Generate the description from the Go code.** Go has no single place
  that pairs a route with its request and answer types, and annotations
  would be a second, unchecked source. The tests above check the
  description against the code instead.
- **Version by header** (`Accept: …; version=2`). A path is visible in
  every log, and every proxy and tool understands it.
- **No versioning while the only clients are our apps.** The admin API is
  for automation too (`admin-cli`), so it has outside clients already.

## Consequences

- **Changing an endpoint** means changing its description in the same
  change; the tests fail otherwise. `/endpoint` and CLAUDE.md say so.
- **The apps' types are generated** from the description
  (`openapi-typescript`), not written by hand.
- **A breaking change is visible:** it shows as a changed schema in review.
