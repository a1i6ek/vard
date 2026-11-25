// Package setup is the first migration: the whole schema in one step,
// schema/up.sql, and the rows an installation starts with (seed.go). A later
// change is a changelog (internal/infrastructure/database/migrations/changelogs),
// never an edit here: a migration that has run anywhere says the same thing
// forever.
package setup

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Version is the setup's place in goose_db_version; every changelog comes
// after it.
const Version = 1

// Migration builds the database from nothing, and its down takes it all
// away again.
var Migration = goose.NewGoMigration(Version, &goose.GoFunc{RunTx: upSchema}, &goose.GoFunc{RunTx: downSchema})

var (
	//go:embed schema/up.sql
	schemaUp string
	//go:embed schema/down.sql
	schemaDown string
)

// gormTx wraps the transaction goose hands a migration in GORM, so a
// migration can use the migrator instead of writing every statement by hand.
// Everything it does runs in goose's transaction, so a migration that fails
// half way leaves nothing behind.
func gormTx(tx *sql.Tx) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: tx}), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("wrap transaction: %w", err)
	}
	return db, nil
}

// upSchema builds the whole database: every table, constraint and index, the
// search index where the database allows it, and the few rows an installation
// starts with.
func upSchema(_ context.Context, tx *sql.Tx) error {
	db, err := gormTx(tx)
	if err != nil {
		return err
	}

	if err := db.Exec(schemaUp).Error; err != nil {
		return fmt.Errorf("create the schema: %w", err)
	}

	if err := searchIndexes(db); err != nil {
		return err
	}

	return seed(db)
}

// downSchema drops everything again.
func downSchema(_ context.Context, tx *sql.Tx) error {
	if _, err := tx.Exec(schemaDown); err != nil {
		return fmt.Errorf("drop the schema: %w", err)
	}
	return nil
}

// The trigram indexes, which find a fragment anywhere in a text: the users
// page's search over an address, a name or a field, and the activity log's
// over who acted, what they did, the address and the record's id. They need
// pg_trgm, which a managed database may not let us install; without it both
// searches still work, by scanning.
func searchIndexes(db *gorm.DB) error {
	if err := db.Exec("SAVEPOINT trigram").Error; err != nil {
		return err
	}

	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error; err != nil {
		return db.Exec("ROLLBACK TO SAVEPOINT trigram").Error
	}

	for name, statement := range map[string]string{
		"idx_users_search": `CREATE INDEX idx_users_search ON users USING gin (
			LOWER(email || ' ' || COALESCE(first_name, '') || ' ' || COALESCE(last_name, '') || ' ' || COALESCE(data::text, ''))
			gin_trgm_ops)`,
		"idx_audit_logs_search": `CREATE INDEX idx_audit_logs_search ON audit_logs
			USING gin (actor_email gin_trgm_ops, action gin_trgm_ops, ip gin_trgm_ops, target_id gin_trgm_ops)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
	}

	return nil
}
