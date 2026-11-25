// Package changelogs holds every change to the schema after the setup, one
// file each, numbered in the order they run: 00002_x.go is version 2. Each
// adds itself from its init function with add; make migrate-new name=x
// writes the next one. A changelog that has run anywhere is never edited.
package changelogs

import (
	"context"
	"database/sql"
	"slices"

	"github.com/pressly/goose/v3"
)

var all []*goose.Migration

// add registers a changelog under its file's number. Both directions run in
// one transaction, so a change that fails half way leaves nothing behind.
func add(version int64, up, down func(context.Context, *sql.Tx) error) {
	all = append(all, goose.NewGoMigration(version, &goose.GoFunc{RunTx: up}, &goose.GoFunc{RunTx: down}))
}

// addOutsideTransaction registers a changelog that cannot run in a
// transaction: CREATE INDEX CONCURRENTLY, which builds an index without
// holding up writes to a table that grows. Each statement must be safe to
// run again, since a failure part way leaves what came before it in place.
func addOutsideTransaction(version int64, up, down func(context.Context, *sql.DB) error) {
	all = append(all, goose.NewGoMigration(version, &goose.GoFunc{RunDB: up}, &goose.GoFunc{RunDB: down}))
}

// All is every changelog, oldest first.
func All() []*goose.Migration {
	sorted := slices.Clone(all)
	slices.SortFunc(sorted, func(a, b *goose.Migration) int { return int(a.Version - b.Version) })
	return sorted
}
