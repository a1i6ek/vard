// Package apptest starts the whole server — both APIs, their routes and
// permission checks, the services, the store and the migrations — on
// throwaway Postgres (and Redis, with VARD_TEST_REDIS), for the integration
// tests in tests/integration. Without VARD_TEST_DB_DSN a test that starts
// one is skipped.
package apptest

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"

	"github.com/a1i6ek/vard/internal/api"
	"github.com/a1i6ek/vard/internal/api/services"
	"github.com/a1i6ek/vard/internal/cache"
	"github.com/a1i6ek/vard/internal/cache/cachetest"
	"github.com/a1i6ek/vard/internal/config"
	"github.com/a1i6ek/vard/internal/crypto/jose"
	"github.com/a1i6ek/vard/internal/database/databasetest"
	"github.com/a1i6ek/vard/internal/mail"
	"github.com/a1i6ek/vard/internal/store"
)

// AccountURL is where the provider sends browsers to sign in. Nothing is
// served there: a test reads the redirect and calls the account endpoints
// the page would.
const AccountURL = "http://account.test"

// Options says what a server starts on. With no DB it makes a throwaway
// database, and a cache when the tests have Redis.
type Options struct {
	DB       *gorm.DB
	DBConfig config.DB
	Cache    *cache.Redis
	Mailer   mail.Sender
	// Issuer is the address tokens name; "" is the public listener's own.
	Issuer string
	// Configure changes the configuration before the server is built.
	Configure func(*config.Config)
	// Seed writes what a fresh installation writes before serving: only the
	// first server on a database does.
	Seed bool
}

// Server is a running server.
type Server struct {
	// Public is the public listener's address; Admin the admin one's.
	Public, Admin string
	DB            *gorm.DB
	Cache         *cache.Redis
	// Store is the server's own, for a test that checks what it keeps.
	Store  *store.Store
	Config config.Config
}

// Start serves the public and admin APIs on listeners of their own, closed
// when the test ends.
func Start(t *testing.T, o Options) *Server {
	t.Helper()

	if o.DB == nil {
		o.DB, o.DBConfig = databasetest.Open(t)
		o.Cache = cachetest.Open(t)
	}
	if o.Configure == nil {
		o.Configure = func(*config.Config) {}
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// The issuer has to be the address the server answers on, which is only
	// known once it has a listener: so the listeners come first, and the
	// routers are built to fit them.
	var publicRouter, adminRouter http.Handler
	publicServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		publicRouter.ServeHTTP(w, r)
	}))
	adminServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminRouter.ServeHTTP(w, r)
	}))
	root := "http://" + publicServer.Listener.Addr().String()
	adminRoot := "http://" + adminServer.Listener.Addr().String()
	issuer := o.Issuer
	if issuer == "" {
		issuer = root
	}

	serverCfg := config.Config{
		DB:         o.DBConfig,
		Issuer:     issuer,
		AccountURL: AccountURL,
		AdminURL:   "http://admin.test",
		SecretKey:  "integration-test-secret-key-0123456789",
	}
	o.Configure(&serverCfg)

	// With VARD_TEST_REDIS the whole server runs with its cache, under a
	// prefix of its own, so every test also proves that a write is seen by
	// the next read through the cache.
	st := store.New(o.DB).WithCache(o.Cache)
	if o.Seed {
		seed(t, st, serverCfg)
	}

	sealer, err := jose.NewSealer(serverCfg.SecretKey)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := services.New(context.Background(), serverCfg, st, sealer, o.Mailer, log)
	if err != nil {
		t.Fatal(err)
	}
	publicEngine, err := api.NewPublic(serverCfg, log, provider, o.Cache)
	if err != nil {
		t.Fatal(err)
	}
	adminEngine, err := api.NewAdmin(serverCfg, st, sealer, log, provider, o.Cache)
	if err != nil {
		t.Fatal(err)
	}
	publicRouter, adminRouter = publicEngine, adminEngine

	publicServer.Start()
	adminServer.Start()
	t.Cleanup(publicServer.Close)
	t.Cleanup(adminServer.Close)

	return &Server{Public: root, Admin: adminRoot, DB: o.DB, Cache: o.Cache, Store: st, Config: serverCfg}
}

// seed writes what the server writes before serving that the tests rely on:
// the administrators' sign-in settings and the server's own APIs.
func seed(t *testing.T, st *store.Store, cfg config.Config) {
	t.Helper()

	ctx := context.Background()
	if err := st.EnsureAdminSecurity(ctx, cfg.AdminMFARequired); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureSystemAPIs(ctx); err != nil {
		t.Fatal(err)
	}
}
