package signin

import (
	"testing"

	"github.com/a1i6ek/vard/internal/infrastructure/database/databasetest"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

// testStore is a store on a throwaway database of its own.
func testStore(t *testing.T) *store.Store {
	t.Helper()

	db, _ := databasetest.Open(t)

	return store.New(db)
}
