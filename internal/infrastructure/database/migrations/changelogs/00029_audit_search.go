package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	addOutsideTransaction(29, upAuditSearch, downAuditSearch)
}

// upAuditSearch lets the activity log's search use an index. The search
// matches a fragment anywhere in who acted, what they did, the address and
// the record's id (ILIKE '%term%'), which no btree serves; a trigram index
// over the four does, for any term of three letters or more.
//
// audit_logs is the largest table there is, so the index is built
// CONCURRENTLY: writes go on while it builds, which a plain CREATE INDEX
// would hold up for as long as it takes. That cannot run in a transaction,
// and on a large table it runs past the server's statement timeout, so it
// runs on one connection of its own with the timeout lifted for that
// connection alone. A build that fails part way leaves an INVALID index
// behind; the DROP first clears it, so running this again finishes the job.
func upAuditSearch(ctx context.Context, db *sql.DB) error {
	return onOneConnection(ctx, db, "audit search",
		`DROP INDEX CONCURRENTLY IF EXISTS idx_audit_logs_search`,
		`CREATE INDEX CONCURRENTLY idx_audit_logs_search ON audit_logs
			USING gin (actor_email gin_trgm_ops, action gin_trgm_ops, ip gin_trgm_ops, target_id gin_trgm_ops)`,
	)
}

// downAuditSearch drops the index; the search scans the table again.
func downAuditSearch(ctx context.Context, db *sql.DB) error {
	return onOneConnection(ctx, db, "undo audit search",
		`DROP INDEX CONCURRENTLY IF EXISTS idx_audit_logs_search`,
	)
}

// onOneConnection runs statements on a single connection with no statement
// timeout, set back before the connection returns to the pool.
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
