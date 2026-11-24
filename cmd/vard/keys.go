package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/a1i6ek/vard/internal/app"
	"github.com/a1i6ek/vard/internal/platform/config"
)

// keysCommand manages the key secrets are sealed with: keys reseal seals
// every stored secret again under VARD_SECRET_KEY, opening each with it or
// one of VARD_OLD_SECRET_KEYS.
func keysCommand(args []string) int {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if len(args) != 1 || args[0] != "reseal" {
		fmt.Fprintf(os.Stderr, "usage: %s keys reseal\n", os.Args[0])
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		log.Error("fatal", "error", err)
		return 1
	}

	if err := app.Reseal(context.Background(), cfg, log, func(line string) { fmt.Println(line) }); err != nil {
		log.Error("fatal", "error", err)
		return 1
	}
	return 0
}
