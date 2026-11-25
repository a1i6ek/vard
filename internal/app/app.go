// Package app is the composition root: it builds every part of the server
// from the configuration, runs it, and stops it in order. It is the one
// package that knows every other; nothing imports it but cmd.
package app

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/a1i6ek/vard/internal/application/admin"
	"github.com/a1i6ek/vard/internal/infrastructure/cache"
	"github.com/a1i6ek/vard/internal/infrastructure/database"
	"github.com/a1i6ek/vard/internal/infrastructure/mail"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
	"github.com/a1i6ek/vard/internal/platform/config"
	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
	"github.com/a1i6ek/vard/internal/transport/http"
	"github.com/a1i6ek/vard/internal/transport/http/services"
)

// App is the server, built and ready to run.
type App struct {
	cfg config.Config
	log *slog.Logger

	db       *gorm.DB
	shared   *cache.Redis
	store    *store.Store
	provider *services.Provider
	servers  map[string]*http.Server
}

// New connects to the database and Redis, makes sure the rows a fresh
// installation needs exist, loads the keys, and builds both servers. What
// it opened is closed again if a later step fails.
func New(ctx context.Context, cfg config.Config, log *slog.Logger) (_ *App, err error) {
	// Each plain http address would have stopped the server without
	// VARD_ALLOW_INSECURE_HTTP; said on every start, so a test setup's
	// escape never quietly reaches production.
	for _, reason := range cfg.Insecure {
		log.Warn("INSECURE: serving plain http because VARD_ALLOW_INSECURE_HTTP=true; passwords and sessions cross the network in the clear — never in production", "reason", reason)
	}

	a := &App{cfg: cfg, log: log}
	defer func() {
		if err != nil {
			a.close()
		}
	}()

	a.db, err = database.Connect(cfg.DB, log)
	if err != nil {
		return nil, err
	}

	log.Info("database connected")

	// A configured Redis that does not answer stops startup; none configured
	// runs without one.
	a.shared, err = cache.Open(ctx, cfg.Redis, log)
	if err != nil {
		return nil, err
	}

	if a.shared != nil {
		if err := a.shared.BumpAuthorityGroups(ctx); err != nil {
			log.Warn("could not bump the cache's authority groups at startup", "error", err)
		}

		log.Info("redis connected", "addr", cfg.Redis.Addr(), "cache_db", cfg.Redis.CacheDB,
			"session_db", cfg.Redis.SessionDB, "prefix", cfg.Redis.Prefix)
	} else {
		log.Info("redis is not configured; reading every page from the database")
	}

	a.store = store.New(a.db).WithCache(a.shared)

	sealer, err := jose.NewSealer(cfg.SecretKey, cfg.OldSecretKeys...)
	if err != nil {
		return nil, err
	}

	if err := seed(ctx, cfg, a.store, sealer); err != nil {
		return nil, err
	}

	// Whoever opens the setup page first makes the first administrator, so
	// a fresh installation should be set up as soon as it is up.
	exists, err := a.store.AdminsExist(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		log.Warn("no administrator exists yet: open the setup page to make the first one",
			"url", cfg.AdminURL+auth.SetupPage)
	}

	mailer := mail.New(mail.FromStore(a.store, sealer), log)

	// Load or create signing keys before serving, so a wrong VARD_SECRET_KEY
	// stops startup here.
	a.provider, err = services.New(ctx, cfg, a.store, sealer, mailer, log)
	if err != nil {
		return nil, err
	}

	public, err := api.NewPublic(cfg, log, a.provider, a.shared)
	if err != nil {
		return nil, err
	}

	adminEngine, err := api.NewAdmin(cfg, a.store, sealer, log, a.provider, a.shared)
	if err != nil {
		return nil, err
	}

	a.servers = map[string]*http.Server{
		"public": newServer(cfg.Addr, public),
		"admin":  newServer(cfg.AdminAddr, adminEngine),
	}

	return a, nil
}

// seed makes the singleton rows a fresh installation starts from, each from
// the configuration, and leaves any that exist alone.
func seed(ctx context.Context, cfg config.Config, st *store.Store, sealer *jose.Sealer) error {
	if err := st.EnsureAdminSecurity(ctx, cfg.AdminMFARequired); err != nil {
		return err
	}
	if err := ensureMailSettings(ctx, st, sealer, cfg.Mail); err != nil {
		return err
	}
	if err := st.EnsureOTPSettings(ctx); err != nil {
		return err
	}
	return st.EnsureSystemAPIs(ctx)
}

func ensureMailSettings(ctx context.Context, st *store.Store, sealer *jose.Sealer, cfg config.Mail) error {
	settings, password := mail.Seed(cfg)

	// Its id before it is stored, so the password is sealed to it.
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	settings.ID = id

	if password != "" {
		sealed, err := sealer.SealFor(settings.PasswordWhere(), []byte(password))
		if err != nil {
			return err
		}
		settings.Password = sealed
	}

	return st.EnsureMailSettings(ctx, settings)
}

// Run serves both listeners and runs the background workers until ctx ends
// or a listener fails, then stops everything in order (runInOrder).
func (a *App) Run(ctx context.Context) error {
	return runInOrder(ctx, a.log, a.servers, []func(context.Context){
		a.provider.Signing.MaintainKeys,
		func(ctx context.Context) { a.store.KeepSwept(ctx, a.cfg.AuditRetention, a.log) },
		a.shared.KeepSettling,
		a.provider.OIDC.KeepTelling,
		a.provider.Federation.KeepMetadataFresh,
	}, a.close)
}

// runInOrder runs the servers and workers, and stops them in order, each
// within shutdownGrace: the servers, so no request starts anything new; the
// workers, the back-channel sender among them, which finishes its posts
// under way; and last closeAll, the database and Redis they all used.
func runInOrder(ctx context.Context, log *slog.Logger, servers map[string]*http.Server, workers []func(context.Context), closeAll func()) error {
	working, stopWorkers := context.WithCancel(context.Background())
	var running sync.WaitGroup
	for _, work := range workers {
		running.Go(func() { work(working) })
	}

	err := serve(ctx, log, servers)

	stopWorkers()
	stopped, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if !waitFor(stopped, &running) {
		log.Warn("a background worker did not stop in time")
	}

	closeAll()

	return err
}

// close lets go of the database and Redis.
func (a *App) close() {
	if a.shared != nil {
		_ = a.shared.Close()
	}
	if a.db != nil {
		_ = database.Close(a.db)
	}
}

func waitFor(ctx context.Context, running *sync.WaitGroup) bool {
	done := make(chan struct{})
	go func() {
		running.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}
