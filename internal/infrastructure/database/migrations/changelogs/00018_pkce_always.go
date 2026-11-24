package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(18, upPKCEAlways, downPKCEAlways)
}

// upPKCEAlways drops require_pkce: every client uses PKCE now (OAuth 2.1,
// RFC 9700 §2.1.1), so there is nothing left to choose.
func upPKCEAlways(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE applications DROP COLUMN IF EXISTS require_pkce`); err != nil {
		return fmt.Errorf("pkce always: %w", err)
	}
	return nil
}

// downPKCEAlways puts the column back, on for every application, which is
// what the server now does for all of them.
func downPKCEAlways(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE applications ADD COLUMN require_pkce boolean DEFAULT true NOT NULL`,
		`ALTER TABLE applications ALTER COLUMN require_pkce DROP DEFAULT`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("undo pkce always: %w", err)
		}
	}
	return nil
}
