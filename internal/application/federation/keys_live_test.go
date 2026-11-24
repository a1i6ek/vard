package federation

import (
	"testing"
	"time"

	"github.com/a1i6ek/vard/internal/infrastructure/database/databasetest"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

// testStore is a store on a throwaway database of its own.
func testStore(t *testing.T) *store.Store {
	t.Helper()

	db, _ := databasetest.Open(t)

	return store.New(db)
}

// clock is a time the test moves by hand.
type clock struct{ at time.Time }

func (c *clock) now() time.Time { return c.at }

func (c *clock) advance(d time.Duration) { c.at = c.at.Add(d) }
