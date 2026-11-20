package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(24, upLogoutDeliveries, downLogoutDeliveries)
}

// upLogoutDeliveries makes the outbox of back-channel logout notices:
// written with the session's end, sent by a worker until delivered.
func upLogoutDeliveries(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`CREATE TABLE logout_deliveries (
			id uuid NOT NULL,
			created_at timestamp with time zone NOT NULL,
			updated_at timestamp with time zone NOT NULL,
			application_id uuid NOT NULL,
			session_id uuid NOT NULL,
			user_id uuid NOT NULL,
			email character varying(255) DEFAULT ''::character varying NOT NULL,
			status character varying(16) NOT NULL,
			tries bigint NOT NULL,
			next_try_at timestamp with time zone NOT NULL,
			last_error character varying(255) DEFAULT ''::character varying NOT NULL,
			delivered_at timestamp with time zone,
			expires_at timestamp with time zone NOT NULL,
			CONSTRAINT logout_deliveries_pkey PRIMARY KEY (id),
			CONSTRAINT chk_logout_deliveries_status CHECK (status IN ('pending','delivered','failed')),
			CONSTRAINT fk_logout_deliveries_application FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX idx_logout_deliveries_application_id ON logout_deliveries USING btree (application_id)`,
		`CREATE INDEX idx_logout_deliveries_due ON logout_deliveries USING btree (status, next_try_at)`,
		`CREATE INDEX idx_logout_deliveries_expires_at ON logout_deliveries USING btree (expires_at)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("make logout_deliveries: %w", err)
		}
	}
	return nil
}

// downLogoutDeliveries drops the outbox; notices still owed are lost.
func downLogoutDeliveries(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS logout_deliveries`); err != nil {
		return fmt.Errorf("drop logout_deliveries: %w", err)
	}
	return nil
}
