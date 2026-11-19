package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(16, upClientProfiles, downClientProfiles)
}

// upClientProfiles replaces an application's type with its profile (ADR
// 0005): web and spa stay; m2m becomes machine; a native app becomes desktop
// when it is sent back to loopback or signs in by the device grant alone,
// and stays native (mobile) otherwise. A machine client signs its
// assertions, so one that had a secret loses it and cannot authenticate
// until an administrator registers its keys.
func upClientProfiles(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE applications DROP CONSTRAINT chk_applications_type`,
		`ALTER TABLE applications RENAME COLUMN type TO profile`,
		`ALTER TABLE applications RENAME CONSTRAINT applications_type_not_null TO applications_profile_not_null`,
		`ALTER INDEX idx_applications_type RENAME TO idx_applications_profile`,
		`UPDATE applications SET profile = 'machine' WHERE profile = 'm2m'`,
		`UPDATE applications SET profile = 'desktop'
			WHERE profile = 'native' AND (
				COALESCE(jsonb_array_length(redirect_uris), 0) = 0
				OR EXISTS (SELECT 1 FROM jsonb_array_elements_text(redirect_uris) AS uri WHERE uri LIKE 'http://%')
			)`,
		`UPDATE applications SET allow_development_redirects = false WHERE profile <> 'native'`,
		`UPDATE applications SET token_auth_method = 'private_key_jwt', client_secret_hash = '', secret_hint = '', secret_created_at = NULL
			WHERE profile = 'machine' AND token_auth_method <> 'private_key_jwt'`,
		`ALTER TABLE applications ADD CONSTRAINT chk_applications_profile CHECK (profile IN ('web','spa','native','desktop','machine'))`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("client profiles: %w", err)
		}
	}
	return nil
}

// downClientProfiles goes back to types: desktop becomes native again and
// machine m2m. A machine client stays on private_key_jwt, which 00015 still
// takes; the secrets it lost are not brought back.
func downClientProfiles(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE applications DROP CONSTRAINT chk_applications_profile`,
		`UPDATE applications SET profile = 'native' WHERE profile = 'desktop'`,
		`UPDATE applications SET profile = 'm2m' WHERE profile = 'machine'`,
		`ALTER INDEX idx_applications_profile RENAME TO idx_applications_type`,
		`ALTER TABLE applications RENAME COLUMN profile TO type`,
		`ALTER TABLE applications RENAME CONSTRAINT applications_profile_not_null TO applications_type_not_null`,
		`ALTER TABLE applications ADD CONSTRAINT chk_applications_type CHECK (type IN ('web','spa','native','m2m'))`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("undo client profiles: %w", err)
		}
	}
	return nil
}
