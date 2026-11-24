package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(22, upMachineTokens, downMachineTokens)
}

// upMachineTokens binds every machine client's tokens to a DPoP key and
// keeps them to fifteen minutes, as a service asks again rather than
// refreshes.
func upMachineTokens(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `UPDATE applications
		SET dpop_bound_access_tokens = true, access_token_lifetime = LEAST(access_token_lifetime, 900)
		WHERE profile = 'machine'`); err != nil {
		return fmt.Errorf("machine tokens: %w", err)
	}
	return nil
}

// downMachineTokens lets machine clients get bearer tokens again; their
// shortened lifetimes stay, which the earlier limits allow.
func downMachineTokens(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `UPDATE applications SET dpop_bound_access_tokens = false WHERE profile = 'machine'`); err != nil {
		return fmt.Errorf("undo machine tokens: %w", err)
	}
	return nil
}
