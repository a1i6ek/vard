package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(12, upSSOAssumeEmailVerified, downSSOAssumeEmailVerified)
}

// upSSOAssumeEmailVerified lets an OpenID Connect connection take an address
// its provider sends without email_verified as proved. Every existing
// connection is off: an address nobody vouched for no longer signs in.
func upSSOAssumeEmailVerified(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE sso_connections ADD COLUMN assume_email_verified boolean NOT NULL DEFAULT false`,
		`ALTER TABLE sso_connections ALTER COLUMN assume_email_verified DROP DEFAULT`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("sso assume email verified: %w", err)
		}
	}
	return nil
}

// downSSOAssumeEmailVerified drops the column. A connection that trusted its
// provider's unlabelled addresses forgets it, and the server below 00012
// takes them as proved again.
func downSSOAssumeEmailVerified(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE sso_connections DROP COLUMN IF EXISTS assume_email_verified`); err != nil {
		return fmt.Errorf("undo sso assume email verified: %w", err)
	}
	return nil
}
