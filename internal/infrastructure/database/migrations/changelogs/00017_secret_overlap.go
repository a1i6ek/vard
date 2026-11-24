package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(17, upSecretOverlap, downSecretOverlap)
}

// upSecretOverlap keeps the secret a rotation replaced, hashed, until the
// overlap its administrator chose ends.
func upSecretOverlap(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE applications ADD COLUMN previous_secret_hash character varying(64) DEFAULT ''::character varying NOT NULL`,
		`ALTER TABLE applications ADD COLUMN previous_secret_expires_at timestamp with time zone`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("secret overlap: %w", err)
		}
	}
	return nil
}

// downSecretOverlap drops them: a secret still in its overlap stops working.
func downSecretOverlap(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE applications DROP COLUMN IF EXISTS previous_secret_expires_at`,
		`ALTER TABLE applications DROP COLUMN IF EXISTS previous_secret_hash`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("undo secret overlap: %w", err)
		}
	}
	return nil
}
