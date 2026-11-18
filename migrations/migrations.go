// Package migrations is every migration, in the order goose applies them:
// the setup (migrations/setup), which makes the whole schema and its first
// rows, then each changelog (migrations/changelogs). They are Go, compiled
// into the binary, so nothing is read from disk at run time.
package migrations

import (
	"github.com/pressly/goose/v3"

	"github.com/a1i6ek/vard/migrations/changelogs"
	"github.com/a1i6ek/vard/migrations/setup"
)

// All is the setup, then every changelog.
func All() []*goose.Migration {
	return append([]*goose.Migration{setup.Migration}, changelogs.All()...)
}
