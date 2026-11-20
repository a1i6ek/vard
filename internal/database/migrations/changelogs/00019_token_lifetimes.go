package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(19, upTokenLifetimes, downTokenLifetimes)
}

// upTokenLifetimes shortens access tokens to an hour at most, an
// application's and an API's alike, and gives refresh families an idle
// lifetime: fourteen days, or the family's whole lifetime when that is
// shorter. A refresh token keeps when it ends unused; one issued before has
// none, and ends as it would have.
func upTokenLifetimes(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`UPDATE applications SET access_token_lifetime = 3600 WHERE access_token_lifetime > 3600`,
		`UPDATE apis SET token_lifetime = 3600 WHERE token_lifetime > 3600`,
		`ALTER TABLE applications ADD COLUMN refresh_token_idle_lifetime bigint DEFAULT 1209600 NOT NULL`,
		`UPDATE applications SET refresh_token_idle_lifetime = LEAST(refresh_token_idle_lifetime, refresh_token_lifetime)`,
		`ALTER TABLE applications ALTER COLUMN refresh_token_idle_lifetime DROP DEFAULT`,
		`ALTER TABLE refresh_tokens ADD COLUMN idle_expires_at timestamp with time zone`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("token lifetimes: %w", err)
		}
	}
	return nil
}

// downTokenLifetimes drops the idle lifetime; the shortened access token
// lifetimes stay as they are, which the earlier limit allows.
func downTokenLifetimes(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS idle_expires_at`,
		`ALTER TABLE applications DROP COLUMN IF EXISTS refresh_token_idle_lifetime`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("undo token lifetimes: %w", err)
		}
	}
	return nil
}
