package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/a1i6ek/vard/internal/app"
	"github.com/a1i6ek/vard/internal/platform/config"
	"github.com/a1i6ek/vard/internal/infrastructure/database/migrations"
)

// migrateCommand applies, rolls back, lists and resets the database
// migrations, and writes the next changelog.
func migrateCommand(args []string) int {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := migrate(log, args); err != nil {
		log.Error("fatal", "error", err)
		return 1
	}
	return 0
}

const migrateUsage = "usage: %s migrate up|down|status, reset -yes, or new <name>"

func migrate(log *slog.Logger, args []string) error {
	// Writing a new, empty changelog needs no database and no configuration.
	if len(args) == 2 && args[0] == "new" {
		path, err := migrations.NewChangelog(migrations.ChangelogsDir, args[1])
		if err != nil {
			return err
		}
		fmt.Println("created", path)
		return nil
	}

	command := ""
	if len(args) > 0 {
		command = args[0]
	}
	switch {
	case len(args) == 1 && command != "reset":
	case len(args) == 2 && command == "reset" && args[1] == "-yes":
	case command == "reset":
		return errors.New("reset deletes every row in the database; run it as migrate reset -yes")
	default:
		return fmt.Errorf(migrateUsage, os.Args[0])
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	return app.Migrate(cfg, log, command, func(line string) { fmt.Println(line) })
}
