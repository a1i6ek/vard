package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(28, upDeviceOrigin, downDeviceOrigin)
}

// upDeviceOrigin keeps where a device asked to be signed in from, to show
// the person allowing it. A device already waiting has none.
func upDeviceOrigin(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE device_authorizations ADD COLUMN request_ip character varying(45) DEFAULT ''::character varying NOT NULL`,
		`ALTER TABLE device_authorizations ADD COLUMN request_agent character varying(255) DEFAULT ''::character varying NOT NULL`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("device origin: %w", err)
		}
	}
	return nil
}

// downDeviceOrigin forgets it.
func downDeviceOrigin(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE device_authorizations DROP COLUMN IF EXISTS request_agent`,
		`ALTER TABLE device_authorizations DROP COLUMN IF EXISTS request_ip`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("undo device origin: %w", err)
		}
	}
	return nil
}
