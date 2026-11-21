---
description: Add an admin API endpoint, the way this repository adds one
argument-hint: "<what it should do>"
---

Add an endpoint for: $ARGUMENTS

Follow the shape the other packages already have, and read the nearest one
first — `internal/api/fields/` is the smallest complete example.

1. The handler method in the package under `internal/api/` the subject belongs
   to, or a new package of the same four files: `handler.go`, `request.go`,
   `response.go`, `validation.go`, whose `Handler` is built by `New(deps Deps)`
   (`Deps`: the store, the services, the audit recorder and the log it is
   given). A handler reads the request, asks the store
   and answers; it decides no statuses of its own beyond the happy one, passing
   everything else to `respond.Failure`.
2. Queries in `internal/store` — never in the handler — returning models, and
   `store.ErrNotFound` / `store.ErrDuplicate` for what the caller has to know.
3. The rules in `validation.go`, or in the model when they are what the thing
   _is_ rather than what this request may say. A broken rule is a problem
   from `respond.Define`, with its sentence in `i18n/console/server.en.json`
   (see "An error" in CLAUDE.md) — never a sentence written in Go.
4. Mount it in `registerAdminRoutes` in `internal/api/server.go`, behind
   `session.Require` and the permission it needs, on the admin server unless it
   is genuinely public.
5. Describe it in `api/openapi/admin/<tag>.yaml` (and reference it from
   `api/openapi/admin.yaml`): its parameters, body and answer, with a schema
   in `admin/schemas.yaml` for a new answer and a `spec.Match` pair for it
   in the package's `types_test.go`; a new problem code in `common.yaml`'s
   `ProblemCode`. The tests fail on a route the description lacks.
6. `h.audit.Record(...)` for anything that changes something, and a line in
   `web/console/src/lib/components/activity/actions.ts` for the new action.
7. Tests: table-driven ones for the validation, and a `TestLive…` in
   `tests/integration/` (the subject's file) when the route, the store or a permission
   is what you want to prove.

Then run `/check`.
