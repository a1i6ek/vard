package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(23, upTokenExchange, downTokenExchange)
}

// upTokenExchange makes the rules for token exchange (RFC 8693): which
// application may exchange which application's tokens for a token to which
// API, with which scopes. It starts empty, so nothing may be exchanged until
// an administrator says so.
func upTokenExchange(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`CREATE TABLE token_exchange_rules (
			api_id uuid NOT NULL,
			application_id uuid NOT NULL,
			subject_application_id uuid NOT NULL,
			scopes jsonb NOT NULL,
			created_at timestamp with time zone NOT NULL,
			CONSTRAINT token_exchange_rules_pkey PRIMARY KEY (api_id, application_id, subject_application_id),
			CONSTRAINT fk_token_exchange_rules_api FOREIGN KEY (api_id) REFERENCES apis(id) ON DELETE CASCADE,
			CONSTRAINT fk_token_exchange_rules_application FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE,
			CONSTRAINT fk_token_exchange_rules_subject_application FOREIGN KEY (subject_application_id) REFERENCES applications(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX idx_token_exchange_rules_application ON token_exchange_rules USING btree (application_id)`,
		`CREATE INDEX idx_token_exchange_rules_subject_application ON token_exchange_rules USING btree (subject_application_id)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("make token_exchange_rules: %w", err)
		}
	}
	return nil
}

// downTokenExchange drops the rules: nothing may be exchanged, and what was
// allowed is lost.
func downTokenExchange(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS token_exchange_rules`); err != nil {
		return fmt.Errorf("drop token_exchange_rules: %w", err)
	}
	return nil
}
