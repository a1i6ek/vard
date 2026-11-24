package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(27, upAppAssociations, downAppAssociations)
}

// upAppAssociations keeps a mobile app's iOS app IDs and Android packages,
// which the sign-in origin publishes so the app may use its passkeys.
func upAppAssociations(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE applications ADD COLUMN ios_app_ids jsonb`,
		`ALTER TABLE applications ADD COLUMN android_apps jsonb`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("app associations: %w", err)
		}
	}
	return nil
}

// downAppAssociations drops them: the association files go unpublished.
func downAppAssociations(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE applications DROP COLUMN IF EXISTS android_apps`,
		`ALTER TABLE applications DROP COLUMN IF EXISTS ios_app_ids`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("undo app associations: %w", err)
		}
	}
	return nil
}
