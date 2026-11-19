package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(15, upPrivateKeyJWT, downPrivateKeyJWT)
}

// upPrivateKeyJWT lets a client authenticate with a signed assertion
// (RFC 7523 §2.2): its public keys, inline or at an address of its own, and
// the assertions already used, each remembered until it would have expired.
func upPrivateKeyJWT(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE applications ADD COLUMN jwks jsonb`,
		`ALTER TABLE applications ADD COLUMN jwks_uri character varying(512) DEFAULT ''::character varying NOT NULL`,
		`ALTER TABLE applications DROP CONSTRAINT chk_applications_token_auth_method`,
		`ALTER TABLE applications ADD CONSTRAINT chk_applications_token_auth_method CHECK (token_auth_method IN ('client_secret_basic','client_secret_post','private_key_jwt','none'))`,
		`CREATE TABLE client_assertions (
			id uuid NOT NULL,
			created_at timestamp with time zone NOT NULL,
			updated_at timestamp with time zone NOT NULL,
			hash character varying(64) NOT NULL,
			expires_at timestamp with time zone NOT NULL
		)`,
		`ALTER TABLE client_assertions ADD CONSTRAINT client_assertions_pkey PRIMARY KEY (id)`,
		`CREATE INDEX idx_client_assertions_expires_at ON client_assertions USING btree (expires_at)`,
		`CREATE UNIQUE INDEX idx_client_assertions_hash ON client_assertions USING btree (hash)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("private_key_jwt: %w", err)
		}
	}
	return nil
}

// downPrivateKeyJWT drops the keys and the used assertions. A client that
// signed assertions is left on client_secret_basic with no secret, so it
// cannot authenticate until an administrator issues it one.
func downPrivateKeyJWT(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`DROP TABLE IF EXISTS client_assertions`,
		`UPDATE applications SET token_auth_method = 'client_secret_basic', client_secret_hash = '' WHERE token_auth_method = 'private_key_jwt'`,
		`ALTER TABLE applications DROP CONSTRAINT chk_applications_token_auth_method`,
		`ALTER TABLE applications ADD CONSTRAINT chk_applications_token_auth_method CHECK (token_auth_method IN ('client_secret_basic','client_secret_post','none'))`,
		`ALTER TABLE applications DROP COLUMN IF EXISTS jwks_uri`,
		`ALTER TABLE applications DROP COLUMN IF EXISTS jwks`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("undo private_key_jwt: %w", err)
		}
	}
	return nil
}
