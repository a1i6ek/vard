package changelogs

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	add(13, upDropFrontchannelLogout, downDropFrontchannelLogout)
}

// upDropFrontchannelLogout removes front-channel logout (OpenID Connect
// Front-Channel Logout 1.0): browsers block the third-party cookies its
// hidden frames need, so it failed silently, and back-channel logout is the
// one way applications are told. Applications lose their
// frontchannel_logout_uri, and logout pages waiting to be shown are dropped.
func upDropFrontchannelLogout(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`DROP TABLE IF EXISTS frontchannel_logouts`,
		`ALTER TABLE applications DROP COLUMN IF EXISTS frontchannel_logout_uri`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("drop front-channel logout: %w", err)
		}
	}
	return nil
}

// downDropFrontchannelLogout makes the table and the column again, as the
// first schema made them. What they held is not brought back: every
// application's frontchannel_logout_uri is empty, and no logout page waits.
func downDropFrontchannelLogout(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE applications ADD COLUMN frontchannel_logout_uri character varying(512) DEFAULT ''::character varying NOT NULL`,
		`CREATE TABLE frontchannel_logouts (
			id uuid NOT NULL,
			created_at timestamp with time zone NOT NULL,
			updated_at timestamp with time zone NOT NULL,
			handle_hash character varying(64) NOT NULL,
			frames jsonb NOT NULL,
			next character varying(2048) NOT NULL,
			language character varying(35) DEFAULT ''::character varying NOT NULL,
			expires_at timestamp with time zone NOT NULL,
			used_at timestamp with time zone
		)`,
		`ALTER TABLE frontchannel_logouts ADD CONSTRAINT frontchannel_logouts_pkey PRIMARY KEY (id)`,
		`CREATE INDEX idx_frontchannel_logouts_expires_at ON frontchannel_logouts USING btree (expires_at)`,
		`CREATE UNIQUE INDEX idx_frontchannel_logouts_handle_hash ON frontchannel_logouts USING btree (handle_hash)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("undo dropping front-channel logout: %w", err)
		}
	}
	return nil
}
