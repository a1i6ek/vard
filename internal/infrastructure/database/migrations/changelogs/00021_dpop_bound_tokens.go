package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(21, upDPoPBoundTokens, downDPoPBoundTokens)
}

// upDPoPBoundTokens lets an application require a DPoP proof with every
// token request (RFC 9449 §5.2), and turns it on for the browser and mobile
// applications, which ADR 0005 requires it of.
func upDPoPBoundTokens(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE applications ADD COLUMN dpop_bound_access_tokens boolean DEFAULT false NOT NULL`,
		`UPDATE applications SET dpop_bound_access_tokens = true WHERE profile IN ('spa','native')`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("dpop bound tokens: %w", err)
		}
	}
	return nil
}

// downDPoPBoundTokens drops the requirement: those applications may get
// bearer tokens again.
func downDPoPBoundTokens(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE applications DROP COLUMN IF EXISTS dpop_bound_access_tokens`); err != nil {
		return fmt.Errorf("undo dpop bound tokens: %w", err)
	}
	return nil
}
