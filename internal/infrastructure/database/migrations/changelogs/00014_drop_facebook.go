package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(14, upDropFacebook, downDropFacebook)
}

// upDropFacebook removes the Facebook social provider kind. Facebook names no
// claim saying an address is verified, so its addresses could only ever be
// assumed proved. Its providers are deleted, and with them (ON DELETE
// CASCADE) the identities and the waiting sign-ins made through them; the
// accounts themselves stay.
func upDropFacebook(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`DELETE FROM social_providers WHERE kind = 'facebook'`,
		`ALTER TABLE social_providers DROP CONSTRAINT chk_social_providers_kind`,
		`ALTER TABLE social_providers ADD CONSTRAINT chk_social_providers_kind CHECK (kind IN ('google','apple','yandex','vk','oidc'))`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("drop facebook: %w", err)
		}
	}
	return nil
}

// downDropFacebook lets the kind be stored again. The providers, identities
// and waiting sign-ins deleted going up are not brought back.
func downDropFacebook(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE social_providers DROP CONSTRAINT chk_social_providers_kind`,
		`ALTER TABLE social_providers ADD CONSTRAINT chk_social_providers_kind CHECK (kind IN ('google','apple','facebook','yandex','vk','oidc'))`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("undo dropping facebook: %w", err)
		}
	}
	return nil
}
