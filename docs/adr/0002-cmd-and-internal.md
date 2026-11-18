# 0002 The Go code lives in `cmd/` and `internal/`

- **Status:** Accepted
- **Date:** 2026-11-19

## Context

On 2026-11-02 (cea44a2) the Go code moved from `internal/` to three
top-level folders, the way PocketBase lays itself out: `tools/` (building
blocks), `core/` (the server) and `api/` (HTTP), with `migrations/` and
`i18n/` beside them. PocketBase is laid out that way because it is a framework:
other programs import its packages, so they have to be importable.

This project is not a framework. It is a server binary, and nothing imports
it. The Go project's own guidance for that case (go.dev/doc/modules/layout,
"Server project") is `cmd/` for the commands and `internal/` for everything
else. Established Go servers follow it: Dex and Gitea keep their commands thin
and their packages under `internal/` or an equivalent folder that nothing
outside imports.

The module path was `vard`, which isn't an import path anyone could fetch. It
is now `github.com/a1i6ek/vard` (the first stage of this restructure).

## Decision

The Go code moves back under `internal/`, mechanically: `git mv` and import
paths, no identifier renamed and no code edited.

| From                   | To                                       |
| ---------------------- | ---------------------------------------- |
| `core/<package>`       | `internal/<package>`                     |
| `core/auth`            | `internal/admin`                         |
| `tools/<package>`      | `internal/crypto/<package>`              |
| `api`, `api/<subject>` | `internal/api`, `internal/api/<subject>` |
| `api/kit/<package>`    | `internal/api/<package>`                 |
| `migrations`           | `internal/database/migrations`           |

`i18n/` stays at the top of the repository: the one Go package outside
`cmd/` and `internal/`. It embeds the shipped English (`i18n/id/*.en.json`,
`i18n/console/*.en.json`), and the sign-in app reads the same files in its
tests, its dev server and its image (`web/id/src/lib/i18n/messages.ts`,
`web/id/vite.config.ts`, `web/Dockerfile`, `web/Dockerfile.dockerignore`). Go's
`embed` can't reach a parent folder, so moving the package moves the files,
and nothing under `web/` changes in this restructure. Moving it later is those
four paths in `web/`, a change of its own.

`layers_test.go` keeps the same layers and the same rules for the new folders.
`migrations` and `i18n`, which sat beside the layers, get layers of their own,
so the test now walks `migrations` too without allowing anything new.

`core/auth` keeps its package clause (`package auth`) for now: renaming it
changes every `auth.` selector, and the packages that import it have local
variables named `admin`. A later stage renames it with the code around it.

## Alternatives rejected

- **Keep `core/`.** Its reason, being importable by other programs, doesn't
  apply, and the Go toolchain enforces `internal/`: nothing outside this
  module can depend on a package by accident.
- **`pkg/` for the building blocks.** `pkg/` says "import me", which nothing
  should, and it's a name with no meaning of its own.
- **Split `core/oidc` in the same change.** A move that also changes code
  can't be reviewed as a move. That split is its own stage, after a call map
  and its own record.

## Consequences

- `git log --follow` and `git blame -M` follow every file through the move.
- Any branch touching Go code at the time conflicts on paths, so in-flight
  work is merged first.
- `CLAUDE.md`, `README.md`, the Makefile, the Dockerfile, the scripts and the
  Claude commands name the new paths. Each security invariant names its files
  by their new paths, word for word otherwise.
- Goose records migration versions, not file paths, so an existing database
  upgrades as before; the upgrade tests prove it.
