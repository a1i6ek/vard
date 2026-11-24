package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(25, upSSOClientAuth, downSSOClientAuth)
}

// upSSOClientAuth lets an OpenID Connect connection authenticate to its
// provider with an assertion signed by its own key; every existing one goes
// on with its client secret.
func upSSOClientAuth(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE sso_connections ADD COLUMN client_auth character varying(16) DEFAULT 'client_secret'::character varying NOT NULL`,
		`ALTER TABLE sso_connections ADD CONSTRAINT chk_sso_connections_client_auth CHECK (client_auth IN ('client_secret','private_key_jwt'))`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("sso client_auth: %w", err)
		}
	}
	return nil
}

// downSSOClientAuth puts every connection back on its client secret.
func downSSOClientAuth(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE sso_connections DROP COLUMN IF EXISTS client_auth`); err != nil {
		return fmt.Errorf("undo sso client_auth: %w", err)
	}
	return nil
}
