package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	"github.com/a1i6ek/vard/internal/app"
	"github.com/a1i6ek/vard/internal/platform/brand"
	"github.com/a1i6ek/vard/internal/platform/config"
)

// serveCommand runs the server until it is told to stop.
func serveCommand() int {
	// The level is the configuration's, which is read inside run: until then,
	// and for a configuration that does not load, it is info.
	level := new(slog.LevelVar)
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})).
		With("service", brand.Slug)

	// Gin defaults to debug mode, which a bare binary under systemd would run
	// in production; GIN_MODE=debug still turns it on.
	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	if err := run(log, level); err != nil {
		log.Error("fatal", "error", err)
		return 1
	}
	return 0
}

func run(log *slog.Logger, level *slog.LevelVar) error {
	log.Info("starting", "version", version)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	level.Set(cfg.LogLevel)

	server, err := app.New(context.Background(), cfg, log)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return server.Run(ctx)
}
