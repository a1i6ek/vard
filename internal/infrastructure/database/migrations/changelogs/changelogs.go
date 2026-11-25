// Package changelogs holds every change to the schema after the setup, one
// file each, numbered in the order they run: 00002_x.go is version 2. Each
// adds itself from its init function with add; make migrate-new name=x
// writes the next one. A changelog that has run anywhere is never edited.
//
// There are none yet: the history before 2026-11-26 was squashed into the
// setup, and the next change is version 2.
package changelogs

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"github.com/pressly/goose/v3"
)

var all []*goose.Migration

// add registers a changelog under its file's number. Both directions run in
// one transaction, so a change that fails half way leaves nothing behind.
//
//lint:ignore U1000 the next changelog registers itself with it
func add(version int64, up, down func(context.Context, *sql.Tx) error) {
	all = append(all, goose.NewGoMigration(version, &goose.GoFunc{RunTx: up}, &goose.GoFunc{RunTx: down}))
}

// addOutsideTransaction registers a changelog that cannot run in a
// transaction: CREATE INDEX CONCURRENTLY, which builds an index without
// holding up writes to a table that grows. Each statement must be safe to
// run again, since a failure part way leaves what came before it in place.
//
//lint:ignore U1000 a changelog that builds an index CONCURRENTLY uses it
func addOutsideTransaction(version int64, up, down func(context.Context, *sql.DB) error) {
	all = append(all, goose.NewGoMigration(version, &goose.GoFunc{RunDB: up}, &goose.GoFunc{RunDB: down}))
}

// All is every changelog, oldest first.
func All() []*goose.Migration {
	sorted := slices.Clone(all)
	slices.SortFunc(sorted, func(a, b *goose.Migration) int { return int(a.Version - b.Version) })
	return sorted
}

// onOneConnection runs statements outside a transaction on a single
// connection with no statement timeout, set back before the connection
// returns to the pool: CREATE INDEX CONCURRENTLY on a large table runs past
// the server's limit. Drop an index CONCURRENTLY IF EXISTS before building
// it, since a build that fails leaves an INVALID one behind.
//
//lint:ignore U1000 a changelog that builds an index CONCURRENTLY uses it
func onOneConnection(ctx context.Context, db *sql.DB, what string, statements ...string) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SET statement_timeout = 0`); err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), `RESET statement_timeout`) //nolint:errcheck // the connection is closed next if this fails

	for _, statement := range statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
	}
	return nil
}
