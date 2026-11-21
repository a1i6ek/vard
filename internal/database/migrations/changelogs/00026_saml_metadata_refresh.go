package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(26, upSAMLMetadataRefresh, downSAMLMetadataRefresh)
}

// upSAMLMetadataRefresh records when a SAML connection's metadata was last
// read from its address, and why that failed, for the daily refresh.
func upSAMLMetadataRefresh(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE sso_connections ADD COLUMN metadata_checked_at timestamp with time zone`,
		`ALTER TABLE sso_connections ADD COLUMN metadata_error character varying(512) DEFAULT ''::character varying NOT NULL`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("saml metadata refresh: %w", err)
		}
	}
	return nil
}

// downSAMLMetadataRefresh forgets when metadata was read; it is read only
// by hand again.
func downSAMLMetadataRefresh(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE sso_connections DROP COLUMN IF EXISTS metadata_error`,
		`ALTER TABLE sso_connections DROP COLUMN IF EXISTS metadata_checked_at`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("undo saml metadata refresh: %w", err)
		}
	}
	return nil
}
