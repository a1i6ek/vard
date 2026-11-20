package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(20, upIDTokenAlg, downIDTokenAlg)
}

// upIDTokenAlg lets each application choose the algorithm its ID tokens and
// logout tokens are signed with; every existing one keeps RS256.
func upIDTokenAlg(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE applications ADD COLUMN id_token_signed_response_alg character varying(8) DEFAULT 'RS256'::character varying NOT NULL`,
		`ALTER TABLE applications ADD CONSTRAINT chk_applications_id_token_signed_response_alg CHECK (id_token_signed_response_alg IN ('RS256','PS256','ES256'))`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("id token alg: %w", err)
		}
	}
	return nil
}

// downIDTokenAlg drops the choice: every ID token is RS256 again.
func downIDTokenAlg(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE applications DROP COLUMN IF EXISTS id_token_signed_response_alg`); err != nil {
		return fmt.Errorf("undo id token alg: %w", err)
	}
	return nil
}
