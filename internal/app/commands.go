package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/database"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
	"github.com/a1i6ek/vard/internal/platform/config"
	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
)

// Migrate applies, rolls back, lists (each line to report) or resets the
// migrations. Migrating is the schema owner's: with a MigrateDSN it
// connects as the owner, and never as the runtime role, which may not exist
// yet and may not drop a table or read goose's.
func Migrate(cfg config.Config, log *slog.Logger, command string, report func(string)) error {
	ownerCfg := cfg.DB
	if cfg.DB.MigrateDSN != "" {
		ownerCfg.DSN = cfg.DB.MigrateDSN
	}
	owner, err := database.Open(ownerCfg, log)
	if err != nil {
		return err
	}
	defer database.Close(owner)

	switch command {
	case "up":
		if cfg.DB.MigrateDSN != "" {
			return database.MigrateAsOwner(cfg.DB, log)
		}
		return database.Migrate(owner, log)
	case "down":
		return database.MigrateDown(owner, log)
	case "status":
		states, err := database.MigrateStatus(owner)
		if err != nil {
			return err
		}
		for _, state := range states {
			report(fmt.Sprint(state))
		}
		return nil
	case "reset":
		if err := database.MigrateReset(owner, log); err != nil {
			return err
		}
		// The tables are new, so the runtime role is granted them again.
		if cfg.DB.MigrateDSN != "" {
			return database.MigrateAsOwner(cfg.DB, log)
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q: want up, down, status or reset", command)
	}
}

// CreateFirstAdmin makes the first administrator, a super admin, as the
// setup page would: only while there is none.
func CreateFirstAdmin(ctx context.Context, cfg config.Config, log *slog.Logger, created *model.Admin) error {
	db, err := database.Open(cfg.DB, log)
	if err != nil {
		return err
	}
	defer database.Close(db)

	return store.New(db).CreateFirstAdmin(ctx, created)
}

// Reseal seals every sealed value again under VARD_SECRET_KEY, opening each
// with whichever of it and VARD_OLD_SECRET_KEYS sealed it, and reports how
// many it changed in each column. It may be stopped and run again; once it
// has run through, the old keys open nothing any more and may be dropped.
func Reseal(ctx context.Context, cfg config.Config, log *slog.Logger, report func(string)) error {
	db, err := database.Open(cfg.DB, log)
	if err != nil {
		return err
	}
	defer database.Close(db)

	sealer, err := jose.NewSealer(cfg.SecretKey, cfg.OldSecretKeys...)
	if err != nil {
		return err
	}

	st := store.New(db)
	for _, column := range store.SealedColumns {
		changed, err := st.Reseal(ctx, column, sealer)
		if err != nil {
			return err
		}
		report(fmt.Sprintf("%s.%s: %d sealed again", column.Table, column.Column, changed))
	}
	return nil
}
