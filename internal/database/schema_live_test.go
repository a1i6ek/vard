package database_test

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/a1i6ek/vard/internal/config"
	"github.com/a1i6ek/vard/internal/database"
	"github.com/a1i6ek/vard/internal/database/databasetest"
	"github.com/a1i6ek/vard/internal/model"
)

// The migrations are written out by hand and the models are not, so nothing
// else stops a model changing without the migration that changes every
// existing database with it. Each column, foreign key and model-declared index
// a fresh database would get from the models is the same one the migrations
// make; the migrations may add indexes a struct tag cannot describe.
func TestLiveTheMigrationsMakeWhatTheModelsDescribe(t *testing.T) {
	migrated, _ := databasetest.Open(t)
	described, _ := databasetest.Empty(t)
	if err := described.AutoMigrate(model.All()...); err != nil {
		t.Fatal(err)
	}

	for _, part := range []struct {
		name  string
		query string
		// subset is true where the migrations may have more than the models.
		subset bool
	}{
		{name: "columns", query: `SELECT table_name || '.' || column_name || ' ' || data_type
			|| COALESCE('(' || character_maximum_length || ')', '')
			|| CASE WHEN is_nullable = 'NO' THEN ' not null' ELSE '' END
			|| COALESCE(' default ' || column_default, '')
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name <> 'goose_db_version'`},
		{name: "constraints", query: `SELECT conrelid::regclass || ' ' || conname || ' ' || pg_get_constraintdef(oid)
			FROM pg_constraint
			WHERE connamespace = 'public'::regnamespace AND conrelid::regclass::text <> 'goose_db_version'`},
		{name: "indexes", subset: true, query: `SELECT indexdef FROM pg_indexes
			WHERE schemaname = 'public' AND tablename <> 'goose_db_version'`},
	} {
		t.Run(part.name, func(t *testing.T) {
			made, want := rows(t, migrated, part.query), rows(t, described, part.query)
			for _, line := range want {
				if !slices.Contains(made, line) {
					t.Errorf("the models describe what no migration makes: %s", line)
				}
			}
			if part.subset {
				return
			}
			for _, line := range made {
				if !slices.Contains(want, line) {
					t.Errorf("a migration makes what no model describes: %s", line)
				}
			}
		})
	}
}

// Going down, one step at a time, takes away everything the migrations
// made, and up builds it again.
func TestLiveTheMigrationsGoDownAndUpAgain(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	applied := func() int64 {
		var version int64
		if err := db.Raw("SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied").Scan(&version).Error; err != nil {
			t.Fatal(err)
		}
		return version
	}
	for applied() > 0 {
		if err := database.MigrateDown(db, log); err != nil {
			t.Fatalf("down from %d: %v", applied(), err)
		}
	}
	if left := rows(t, db, `SELECT tablename FROM pg_tables
		WHERE schemaname = 'public' AND tablename <> 'goose_db_version'`); len(left) != 0 {
		t.Errorf("down left %v", left)
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up again: %v", err)
	}
	var roles int64
	if err := db.Table("admin_roles").Count(&roles).Error; err != nil || roles == 0 {
		t.Errorf("up again seeded %d admin roles (%v), want the starting ones", roles, err)
	}
}

// A reset leaves what a new installation starts with: every row added since
// is gone, and the seeded ones are back.
func TestLiveAResetStartsTheDatabaseAgain(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := db.Exec("UPDATE organizations SET name = 'Renamed'").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateReset(db, log); err != nil {
		t.Fatalf("reset: %v", err)
	}

	if names := rows(t, db, "SELECT name FROM organizations"); slices.Contains(names, "Renamed") || len(names) != 1 {
		t.Errorf("after a reset the organizations are %v, want the one a new installation has", names)
	}
	states, err := database.MigrateStatus(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range states {
		if !state.Applied {
			t.Errorf("after a reset migration %d is pending, want every one applied", state.Version)
		}
	}
}

func rows(t *testing.T, db *gorm.DB, query string) []string {
	t.Helper()
	var out []string
	if err := db.Raw(query).Scan(&out).Error; err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

// The database refuses what Go never writes, so a bug or a hand-run query
// cannot store it either. Each row is whole but for the one thing wrong.
func TestLiveTheDatabaseRefusesWhatGoNeverWrites(t *testing.T) {
	db, _ := databasetest.Open(t)

	user := &model.User{Email: "ada@example.com", IsActive: true}
	app := &model.Application{
		Name: "Shop", ClientID: "shop", Profile: model.ProfileWeb, TokenAuthMethod: model.AuthNone, IsEnabled: true,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(app).Error; err != nil {
		t.Fatal(err)
	}
	nowhere := uuid.New()

	for _, tt := range []struct {
		name, refusedBy string
		row             any
	}{
		{"an administrator status no code knows", "chk_admins_status",
			&model.Admin{Username: "a@example.com", Email: "a@example.com", Status: "root"}},
		{"an administrator's address in capitals", "chk_admins_email_lower",
			&model.Admin{Username: "b@example.com", Email: "B@example.com", Status: model.StatusActive}},
		{"a second organization", "idx_organizations_one_row",
			&model.Organization{Name: "Another", Slug: "another", Timezone: "UTC"}},
		{"an application profile no code knows", "chk_applications_profile",
			&model.Application{Name: "X", ClientID: "x", Profile: "mainframe", TokenAuthMethod: model.AuthNone}},
		{"a refresh token naming a session that never was", "fk_refresh_tokens_session",
			&model.RefreshToken{TokenHash: "h", FamilyID: uuid.New(), ApplicationID: app.ID, UserID: user.ID, SessionID: &nowhere}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := db.Omit(clause.Associations).Create(tt.row).Error; err == nil || !strings.Contains(err.Error(), tt.refusedBy) {
				t.Errorf("= %v, want it refused by %s", err, tt.refusedBy)
			}
		})
	}

	// Users' addresses are lower-cased by the model on the way in; the
	// check is for a write that goes round it.
	err := db.Exec("UPDATE users SET email = 'Ada@Example.com' WHERE id = ?", user.ID).Error
	if err == nil || !strings.Contains(err.Error(), "chk_users_email_lower") {
		t.Errorf("an address written in capitals = %v, want it refused by chk_users_email_lower", err)
	}
}

// Instances started together, as replicas are, migrate one at a time: the
// second waits for the first and finds nothing left to do.
func TestLiveInstancesStartingTogetherMigrateOnce(t *testing.T) {
	db, _ := databasetest.Empty(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	errs := make(chan error, 4)
	var started sync.WaitGroup
	for range 4 {
		started.Go(func() { errs <- database.Migrate(db, log) })
	}
	started.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("an instance's migration = %v, want it to wait its turn", err)
		}
	}
}

// The limit is Postgres's own, so a query that runs past it is ended by the
// database even if nothing in Go is waiting to cancel it.
func TestLiveAQueryPastTheStatementTimeoutIsEnded(t *testing.T) {
	_, cfg := databasetest.Empty(t)
	cfg.StatementTimeout = 100 * time.Millisecond

	db, err := database.Open(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(db) })

	err = db.Exec("SELECT pg_sleep(1)").Error
	if err == nil || !strings.Contains(err.Error(), "statement timeout") {
		t.Errorf("a one-second query under a 100ms limit = %v, want it ended", err)
	}
}

// With the owner's DSN to migrate, the server runs as a role made for it that
// reads and writes rows and nothing else: a flaw in the running server cannot
// change the schema, whatever SQL it is tricked into.
func TestLiveTheServerRunsAsARoleThatCannotChangeTheSchema(t *testing.T) {
	role := fmt.Sprintf("app_test_%d", time.Now().UnixNano())
	server, err := database.Open(config.DB{Driver: "postgres", DSN: os.Getenv(databasetest.Env), TimeZone: "UTC"},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() {
		_ = server.Exec(`DROP ROLE IF EXISTS "` + role + `"`).Error
		_ = database.Close(server)
	})

	owner, cfg := databasetest.Empty(t)
	runtime, _ := url.Parse(cfg.DSN)
	runtime.User = url.UserPassword(role, "runtime-password")
	cfg.MigrateDSN, cfg.DSN = cfg.DSN, runtime.String()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for range 2 { // the second start finds the role already made
		if err := database.MigrateAsOwner(cfg, log); err != nil {
			t.Fatalf("migrate as the owner: %v", err)
		}
	}

	db, err := database.Open(cfg, log)
	if err != nil {
		t.Fatalf("connect as the runtime role: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close(db)
		_ = owner.Exec(`DROP OWNED BY "` + role + `"`).Error
	})

	if err := db.Exec("UPDATE organizations SET name = 'Renamed'").Error; err != nil {
		t.Errorf("writing a row as the runtime role = %v, want it allowed", err)
	}
	for _, statement := range []string{
		"CREATE TABLE stolen (id int)",
		"ALTER TABLE users ADD COLUMN stolen text",
		"DROP TABLE audit_logs",
		"DELETE FROM goose_db_version",
	} {
		if err := db.Exec(statement).Error; err == nil || !strings.Contains(err.Error(), "permission denied") &&
			!strings.Contains(err.Error(), "must be owner") {
			t.Errorf("%s as the runtime role = %v, want it refused", statement, err)
		}
	}
}

// A fresh installation starts with no runtime role: the server connects as
// the owner first, which makes it, and only then as the role — in that
// order, or it can never start.
func TestLiveAFreshInstallationStartsAsItsRuntimeRole(t *testing.T) {
	role := fmt.Sprintf("app_fresh_%d", time.Now().UnixNano())
	server, err := database.Open(config.DB{Driver: "postgres", DSN: os.Getenv(databasetest.Env), TimeZone: "UTC"},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() {
		_ = server.Exec(`DROP ROLE IF EXISTS "` + role + `"`).Error
		_ = database.Close(server)
	})

	owner, cfg := databasetest.Empty(t)
	runtime, _ := url.Parse(cfg.DSN)
	runtime.User = url.UserPassword(role, "runtime-password")
	cfg.MigrateDSN, cfg.DSN, cfg.Migrate = cfg.DSN, runtime.String(), true

	db, err := database.Connect(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("a fresh installation could not start: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close(db)
		_ = owner.Exec(`DROP OWNED BY "` + role + `"`).Error
	})

	var user string
	if err := db.Raw("SELECT current_user").Scan(&user).Error; err != nil || user != role {
		t.Errorf("connected as %q (%v), want the runtime role %q", user, err, role)
	}
}

// A database the migrations from before the squash made says by its goose
// versions that it is up to date; it is refused rather than started on.
func TestLiveADatabaseFromBeforeTheSquashIsRefused(t *testing.T) {
	db, _ := databasetest.Empty(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, statement := range []string{
		"CREATE TABLE goose_db_version (id serial PRIMARY KEY, version_id bigint NOT NULL, is_applied boolean NOT NULL, tstamp timestamp DEFAULT now())",
		"INSERT INTO goose_db_version (version_id, is_applied) VALUES (0, true), (1, true), (6, true)",
		"CREATE TABLE users (id uuid PRIMARY KEY)",
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := database.Migrate(db, log); !errors.Is(err, database.ErrSquashedHistory) {
		t.Errorf("migrating it = %v, want ErrSquashedHistory", err)
	}
}

// An installation upgraded to the DPoP binding keeps the requests and codes
// it had, each bound to no key, which is what they were; going back down
// keeps them too, without the column.
func TestLiveTheDPoPBindingUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 4)
	if has := rows(t, db, `SELECT table_name FROM information_schema.columns
		WHERE column_name = 'dpop_key' AND table_name IN ('authorization_requests', 'authorization_codes')`); len(has) != 0 {
		t.Fatalf("before the binding, dpop_key is on %v", has)
	}

	user := &model.User{Email: "ada@example.com", IsActive: true}
	app := &model.Application{Name: "Shop", ClientID: "shop", Profile: model.ProfileWeb, TokenAuthMethod: model.AuthNone, IsEnabled: true}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	writeOldApplication(t, db, app)
	later := time.Now().Add(time.Hour)
	request := &model.AuthorizationRequest{HandleHash: "request", ApplicationID: app.ID, RedirectURI: "https://shop.example/cb", Scope: "openid", ExpiresAt: later}
	code := &model.AuthorizationCode{CodeHash: "code", ApplicationID: app.ID, UserID: user.ID, RedirectURI: "https://shop.example/cb", Scope: "openid", AuthenticatedAt: time.Now(), ExpiresAt: later}
	for _, row := range []any{request, code} {
		// Columns added after version 4 (the request's session_id, 00011) are not there yet.
		if err := db.Omit(clause.Associations, "DPoPKey", "SessionID").Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to the binding: %v", err)
	}
	for _, table := range []string{"authorization_requests", "authorization_codes"} {
		if keys := rows(t, db, "SELECT dpop_key FROM "+table); !slices.Equal(keys, []string{""}) {
			t.Errorf("%s after the upgrade holds keys %q, want the one row, bound to none", table, keys)
		}
	}

	migrateDownTo(t, db, log, 4)
	if has := rows(t, db, `SELECT table_name FROM information_schema.columns
		WHERE column_name = 'dpop_key' AND table_name IN ('authorization_requests', 'authorization_codes')`); len(has) != 0 {
		t.Errorf("down left dpop_key on %v", has)
	}
	for _, table := range []string{"authorization_requests", "authorization_codes"} {
		if left := rows(t, db, "SELECT id::text FROM "+table); len(left) != 1 {
			t.Errorf("down left %d rows in %s, want 1", len(left), table)
		}
	}
}

// migrateDownTo takes the migrations down, one at a time, until version is
// the last one applied.
func migrateDownTo(t *testing.T, db *gorm.DB, log *slog.Logger, version int64) {
	t.Helper()

	applied := func() int64 {
		var at int64
		if err := db.Raw("SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied").Scan(&at).Error; err != nil {
			t.Fatal(err)
		}
		return at
	}
	for applied() > version {
		if err := database.MigrateDown(db, log); err != nil {
			t.Fatalf("down from %d: %v", applied(), err)
		}
	}
	if at := applied(); at != version {
		t.Fatalf("the database is at %d, want %d", at, version)
	}
}

// An installation from before a flow could require a second factor upgrades
// with every flow requiring none and every waiting sign-in its own; going
// back down removes the step-ups still waiting, which would otherwise read
// as sign-ins their first factor never proved.
func TestLiveStepUpUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 5)
	stepUpColumns := `SELECT table_name || '.' || column_name FROM information_schema.columns
		WHERE (table_name = 'login_flows' AND column_name = 'require_second_factor')
			OR (table_name = 'pending_sign_ins' AND column_name = 'step_up_session_id')
		ORDER BY 1`
	if has := rows(t, db, stepUpColumns); len(has) != 0 {
		t.Fatalf("before step-up, the database has %v", has)
	}

	user := &model.User{Email: "ada@example.com", IsActive: true}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	session := &model.UserSession{TokenHash: "session", UserID: user.ID, AuthenticatedAt: time.Now(), ExpiresAt: later}
	waiting := &model.PendingSignIn{HandleHash: "waiting", UserID: user.ID, ExpiresAt: later}
	if err := db.Omit(clause.Associations, "Remember").Create(session).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Omit(clause.Associations, "StepUpSessionID", "ApplicationID", "DeviceAuthorizationID").Create(waiting).Error; err != nil {
		t.Fatal(err)
	}
	flows := rows(t, db, "SELECT id::text FROM login_flows")
	if len(flows) == 0 {
		t.Fatal("the database has no login flow to upgrade")
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to step-up: %v", err)
	}
	if required := rows(t, db, "SELECT id::text FROM login_flows WHERE require_second_factor"); len(required) != 0 {
		t.Errorf("after the upgrade, flows %v require a second factor", required)
	}
	if steps := rows(t, db, "SELECT handle_hash FROM pending_sign_ins WHERE step_up_session_id IS NOT NULL"); len(steps) != 0 {
		t.Errorf("after the upgrade, %v are step-ups", steps)
	}

	stepUp := &model.PendingSignIn{HandleHash: "step-up", UserID: user.ID, StepUpSessionID: &session.ID, ExpiresAt: later}
	if err := db.Omit(clause.Associations).Create(stepUp).Error; err != nil {
		t.Fatal(err)
	}

	migrateDownTo(t, db, log, 5)
	if has := rows(t, db, stepUpColumns); len(has) != 0 {
		t.Errorf("down left %v", has)
	}
	if left := rows(t, db, "SELECT handle_hash FROM pending_sign_ins ORDER BY 1"); !slices.Equal(left, []string{"waiting"}) {
		t.Errorf("down left the waiting sign-ins %v, want only the one that was not a step-up", left)
	}
	if left := rows(t, db, "SELECT id::text FROM login_flows"); len(left) != len(flows) {
		t.Errorf("down left %d login flows, want %d", len(left), len(flows))
	}
}

// Sessions open before 00007 are recorded as not remembered, so a step-up of
// one writes a cookie for the browser's lifetime, as before; going down keeps
// every session.
func TestLiveRememberSessionUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 6)
	column := `SELECT column_name FROM information_schema.columns
		WHERE table_name = 'user_sessions' AND column_name = 'remember'`
	if has := rows(t, db, column); len(has) != 0 {
		t.Fatalf("before 00007, user_sessions has %v", has)
	}

	user := &model.User{Email: "ada@example.com", IsActive: true}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	open := &model.UserSession{TokenHash: "open", UserID: user.ID, AuthenticatedAt: time.Now(), ExpiresAt: later}
	if err := db.Omit(clause.Associations, "Remember").Create(open).Error; err != nil {
		t.Fatal(err)
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00007: %v", err)
	}
	if remembered := rows(t, db, "SELECT token_hash FROM user_sessions WHERE remember"); len(remembered) != 0 {
		t.Errorf("after the upgrade, %v are remembered", remembered)
	}

	kept := &model.UserSession{TokenHash: "kept", UserID: user.ID, AuthenticatedAt: time.Now(), ExpiresAt: later, Remember: true}
	if err := db.Omit(clause.Associations).Create(kept).Error; err != nil {
		t.Fatal(err)
	}

	migrateDownTo(t, db, log, 6)
	if has := rows(t, db, column); len(has) != 0 {
		t.Errorf("down left %v", has)
	}
	if left := rows(t, db, "SELECT token_hash FROM user_sessions ORDER BY 1"); !slices.Equal(left, []string{"kept", "open"}) {
		t.Errorf("down left the sessions %v, want both", left)
	}
}

// Sign-ins waiting before 00008 name no application; going down deletes only
// those that name one, which the default flow would otherwise hold.
func TestLiveSignInApplicationUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 7)
	column := `SELECT column_name FROM information_schema.columns
		WHERE table_name = 'pending_sign_ins' AND column_name = 'application_id'`
	if has := rows(t, db, column); len(has) != 0 {
		t.Fatalf("before 00008, pending_sign_ins has %v", has)
	}

	user := &model.User{Email: "ada@example.com", IsActive: true}
	app := &model.Application{Name: "Telly", ClientID: "telly", Profile: model.ProfileNative, TokenAuthMethod: model.AuthNone, IsEnabled: true}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	writeOldApplication(t, db, app)
	later := time.Now().Add(time.Hour)
	waiting := &model.PendingSignIn{HandleHash: "waiting", UserID: user.ID, ExpiresAt: later}
	if err := db.Omit(clause.Associations, "ApplicationID", "DeviceAuthorizationID").Create(waiting).Error; err != nil {
		t.Fatal(err)
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00008: %v", err)
	}
	if named := rows(t, db, "SELECT handle_hash FROM pending_sign_ins WHERE application_id IS NOT NULL"); len(named) != 0 {
		t.Errorf("after the upgrade, %v name an application", named)
	}

	device := &model.PendingSignIn{HandleHash: "device", UserID: user.ID, ApplicationID: &app.ID, ExpiresAt: later}
	if err := db.Omit(clause.Associations, "DeviceAuthorizationID").Create(device).Error; err != nil {
		t.Fatal(err)
	}

	migrateDownTo(t, db, log, 7)
	if has := rows(t, db, column); len(has) != 0 {
		t.Errorf("down left %v", has)
	}
	if left := rows(t, db, "SELECT handle_hash FROM pending_sign_ins ORDER BY 1"); !slices.Equal(left, []string{"waiting"}) {
		t.Errorf("down left the waiting sign-ins %v, want only the one naming no application", left)
	}
}

// Step-ups waiting before 00009 name no device; going down keeps every
// waiting sign-in, which then only stops naming its device.
func TestLiveSignInDeviceUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 8)
	column := `SELECT column_name FROM information_schema.columns
		WHERE table_name = 'pending_sign_ins' AND column_name = 'device_authorization_id'`
	if has := rows(t, db, column); len(has) != 0 {
		t.Fatalf("before 00009, pending_sign_ins has %v", has)
	}

	user := &model.User{Email: "ada@example.com", IsActive: true}
	app := &model.Application{Name: "Telly", ClientID: "telly", Profile: model.ProfileNative, TokenAuthMethod: model.AuthNone, IsEnabled: true}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	writeOldApplication(t, db, app)
	later := time.Now().Add(time.Hour)
	waiting := &model.PendingSignIn{HandleHash: "waiting", UserID: user.ID, ApplicationID: &app.ID, ExpiresAt: later}
	if err := db.Omit(clause.Associations, "DeviceAuthorizationID").Create(waiting).Error; err != nil {
		t.Fatal(err)
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00009: %v", err)
	}
	if named := rows(t, db, "SELECT handle_hash FROM pending_sign_ins WHERE device_authorization_id IS NOT NULL"); len(named) != 0 {
		t.Errorf("after the upgrade, %v name a device", named)
	}

	device := &model.DeviceAuthorization{DeviceCodeHash: "device", UserCodeHash: "user", ApplicationID: app.ID, Scope: "openid",
		Status: model.DevicePending, Interval: 5, ExpiresAt: later}
	if err := db.Omit(clause.Associations).Create(device).Error; err != nil {
		t.Fatal(err)
	}
	stepUp := &model.PendingSignIn{HandleHash: "step-up", UserID: user.ID, ApplicationID: &app.ID, DeviceAuthorizationID: &device.ID, ExpiresAt: later}
	if err := db.Omit(clause.Associations).Create(stepUp).Error; err != nil {
		t.Fatal(err)
	}

	migrateDownTo(t, db, log, 8)
	if has := rows(t, db, column); len(has) != 0 {
		t.Errorf("down left %v", has)
	}
	if left := rows(t, db, "SELECT handle_hash FROM pending_sign_ins ORDER BY 1"); !slices.Equal(left, []string{"step-up", "waiting"}) {
		t.Errorf("down left the waiting sign-ins %v, want both", left)
	}
}

// Connections from before 00010 answer for nobody's second factor: each is
// local, accepts amr mfa should it be switched to its provider, and lists
// no SAML context. Going down keeps every connection.
func TestLiveSSOSecondFactorUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 9)
	columns := `SELECT column_name FROM information_schema.columns
		WHERE table_name = 'sso_connections' AND column_name IN ('second_factor', 'accepted_amr', 'mfa_contexts')`
	if has := rows(t, db, columns); len(has) != 0 {
		t.Fatalf("before 00010, sso_connections has %v", has)
	}

	acme := &model.SSOConnection{Slug: "acme", Name: "Acme", Protocol: model.SSOProtocolOIDC, Domains: model.StringList{"acme.test"},
		Issuer: "https://acme.test", ClientID: "acme", Matching: model.SSOMatchLink, RoleMappings: model.SSORoleMappings{}}
	if err := db.Omit(clause.Associations, "SecondFactor", "AcceptedAMR", "MFAContexts", "AssumeEmailVerified").Create(acme).Error; err != nil {
		t.Fatal(err)
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00010: %v", err)
	}
	got := rows(t, db, "SELECT second_factor || ' ' || accepted_amr::text || ' ' || mfa_contexts::text FROM sso_connections")
	if want := []string{`local ["mfa"] []`}; !slices.Equal(got, want) {
		t.Errorf("after the upgrade the connection is %v, want %v", got, want)
	}
	if err := db.Exec(`UPDATE sso_connections SET second_factor = 'both'`).Error; err == nil {
		t.Error("a second factor nobody named was stored")
	}
	if err := db.Exec(`UPDATE sso_connections SET second_factor = 'provider'`).Error; err != nil {
		t.Fatal(err)
	}

	migrateDownTo(t, db, log, 9)
	if has := rows(t, db, columns); len(has) != 0 {
		t.Errorf("down left %v", has)
	}
	if left := rows(t, db, "SELECT slug FROM sso_connections"); !slices.Equal(left, []string{"acme"}) {
		t.Errorf("down left the connections %v, want acme", left)
	}
}

// Sign-ins waiting before 00011 name no session, so their step pages refuse
// every answer until the user signs in again. Ending a session unbinds the
// request it reached rather than deleting it, and going down keeps every
// request.
func TestLiveRequestSessionUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 10)
	column := `SELECT column_name FROM information_schema.columns
		WHERE table_name = 'authorization_requests' AND column_name = 'session_id'`
	if has := rows(t, db, column); len(has) != 0 {
		t.Fatalf("before 00011, authorization_requests has %v", has)
	}

	user := &model.User{Email: "ada@example.com", IsActive: true}
	app := &model.Application{Name: "Shop", ClientID: "shop", Profile: model.ProfileWeb, TokenAuthMethod: model.AuthClientSecretBasic, IsEnabled: true}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	writeOldApplication(t, db, app)
	later := time.Now().Add(time.Hour)
	waiting := &model.AuthorizationRequest{HandleHash: "waiting", ApplicationID: app.ID, RedirectURI: "https://shop.test/cb", Scope: "openid", ExpiresAt: later}
	if err := db.Omit(clause.Associations, "SessionID").Create(waiting).Error; err != nil {
		t.Fatal(err)
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00011: %v", err)
	}
	if named := rows(t, db, "SELECT handle_hash FROM authorization_requests WHERE session_id IS NOT NULL"); len(named) != 0 {
		t.Errorf("after the upgrade, %v name a session", named)
	}

	session := &model.UserSession{TokenHash: "session", UserID: user.ID, AuthenticatedAt: time.Now(), ExpiresAt: later}
	if err := db.Omit(clause.Associations).Create(session).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE authorization_requests SET session_id = ? WHERE handle_hash = 'waiting'", session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM user_sessions WHERE id = ?", session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if left := rows(t, db, "SELECT handle_hash FROM authorization_requests WHERE session_id IS NULL"); !slices.Equal(left, []string{"waiting"}) {
		t.Errorf("after its session was deleted, the unbound requests are %v, want waiting", left)
	}

	migrateDownTo(t, db, log, 10)
	if has := rows(t, db, column); len(has) != 0 {
		t.Errorf("down left %v", has)
	}
	if left := rows(t, db, "SELECT handle_hash FROM authorization_requests"); !slices.Equal(left, []string{"waiting"}) {
		t.Errorf("down left the requests %v, want waiting", left)
	}
}

// Connections from before 00012 trust no address their provider sends
// without email_verified. Going down keeps every connection.
func TestLiveSSOAssumeEmailVerifiedUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 11)
	column := `SELECT column_name FROM information_schema.columns
		WHERE table_name = 'sso_connections' AND column_name = 'assume_email_verified'`
	if has := rows(t, db, column); len(has) != 0 {
		t.Fatalf("before 00012, sso_connections has %v", has)
	}

	acme := &model.SSOConnection{Slug: "acme", Name: "Acme", Protocol: model.SSOProtocolOIDC, Domains: model.StringList{"acme.test"},
		Issuer: "https://acme.test", ClientID: "acme", Matching: model.SSOMatchLink, RoleMappings: model.SSORoleMappings{},
		AcceptedAMR: model.DefaultAcceptedAMR, MFAContexts: model.StringList{}}
	if err := db.Omit(clause.Associations, "AssumeEmailVerified").Create(acme).Error; err != nil {
		t.Fatal(err)
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00012: %v", err)
	}
	if got := rows(t, db, "SELECT assume_email_verified::text FROM sso_connections"); !slices.Equal(got, []string{"false"}) {
		t.Errorf("after the upgrade the connection assumes %v, want false", got)
	}
	if got := rows(t, db, `SELECT coalesce(column_default, '') FROM information_schema.columns
		WHERE table_name = 'sso_connections' AND column_name = 'assume_email_verified'`); !slices.Equal(got, []string{""}) {
		t.Errorf("the column keeps the default %v, which the model does not have", got)
	}

	migrateDownTo(t, db, log, 11)
	if has := rows(t, db, column); len(has) != 0 {
		t.Errorf("down left %v", has)
	}
	if left := rows(t, db, "SELECT slug FROM sso_connections"); !slices.Equal(left, []string{"acme"}) {
		t.Errorf("down left the connections %v, want acme", left)
	}
}

// 00013 drops front-channel logout: applications keep everything but their
// frontchannel_logout_uri, and logout pages waiting to be shown go. Going down
// makes the table and the column again, empty.
func TestLiveDroppingFrontchannelLogoutUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 12)
	app := &model.Application{Name: "Shop", ClientID: "shop", Profile: model.ProfileWeb, TokenAuthMethod: model.AuthNone, IsEnabled: true}
	writeOldApplication(t, db, app)
	if err := db.Exec(`UPDATE applications SET frontchannel_logout_uri = 'https://shop.example.com/logout'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO frontchannel_logouts (id, created_at, updated_at, handle_hash, frames, next, expires_at)
		VALUES (gen_random_uuid(), now(), now(), 'waiting', '[]', 'https://id.example.com/logged-out', now() + interval '2 minutes')`).Error; err != nil {
		t.Fatal(err)
	}

	column := `SELECT column_name FROM information_schema.columns
		WHERE table_name = 'applications' AND column_name = 'frontchannel_logout_uri'`
	table := `SELECT table_name FROM information_schema.tables WHERE table_name = 'frontchannel_logouts'`

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00013: %v", err)
	}
	if has := rows(t, db, column); len(has) != 0 {
		t.Errorf("after the upgrade applications still has %v", has)
	}
	if has := rows(t, db, table); len(has) != 0 {
		t.Errorf("after the upgrade the database still has %v", has)
	}
	if left := rows(t, db, "SELECT client_id FROM applications"); !slices.Equal(left, []string{"shop"}) {
		t.Errorf("after the upgrade the applications are %v, want shop", left)
	}

	migrateDownTo(t, db, log, 12)
	if got := rows(t, db, "SELECT frontchannel_logout_uri FROM applications"); !slices.Equal(got, []string{""}) {
		t.Errorf("down made the column hold %v, want it empty", got)
	}
	if got := rows(t, db, "SELECT count(*)::text FROM frontchannel_logouts"); !slices.Equal(got, []string{"0"}) {
		t.Errorf("down made the table hold %v rows, want none", got)
	}
	if err := db.Exec(`INSERT INTO frontchannel_logouts (id, created_at, updated_at, handle_hash, frames, next, expires_at)
		VALUES (gen_random_uuid(), now(), now(), 'again', '[]', 'x', now()), (gen_random_uuid(), now(), now(), 'again', '[]', 'x', now())`).Error; err == nil {
		t.Error("down made the table without its unique handle")
	}
}

// 00014 drops the Facebook kind: its providers go, every other stays, and the
// kind can no longer be stored. Going down lets it be stored again; what was
// deleted does not come back.
func TestLiveDroppingFacebookUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 13)
	for _, kind := range []string{"facebook", "google"} {
		provider := &model.SocialProvider{Kind: model.SocialKind(kind), Slug: kind, Name: kind, ClientID: kind, Scopes: model.StringList{}}
		if err := db.Omit(clause.Associations).Create(provider).Error; err != nil {
			t.Fatalf("a %s provider before 00014: %v", kind, err)
		}
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00014: %v", err)
	}
	if left := rows(t, db, "SELECT kind FROM social_providers ORDER BY kind"); !slices.Equal(left, []string{"google"}) {
		t.Errorf("after the upgrade the providers are %v, want google alone", left)
	}
	again := &model.SocialProvider{Kind: "facebook", Slug: "facebook-again", Name: "Facebook", ClientID: "x", Scopes: model.StringList{}}
	if err := db.Omit(clause.Associations).Create(again).Error; err == nil {
		t.Error("a facebook provider was stored after 00014")
	}

	migrateDownTo(t, db, log, 13)
	again.ID = uuid.Nil
	if err := db.Omit(clause.Associations).Create(again).Error; err != nil {
		t.Errorf("down still refuses a facebook provider: %v", err)
	}
}

// laterApplicationColumns are the application's columns that later
// changelogs add, left out of a row written at an earlier version.
var laterApplicationColumns = []string{"JWKS", "JWKSURI", "Profile", "PreviousSecretHash", "PreviousSecretExpiresAt", "RefreshTokenIdleLifetime", "IDTokenSigningAlg"}

// writeOldApplication writes an application at a version before 00016,
// when what is now its profile was the type column, and require_pkce (gone
// in 00018) was still there, neither with a default: the row goes in
// without them, under defaults that last only as long as the write, and is
// then given its type.
func writeOldApplication(t *testing.T, db *gorm.DB, app *model.Application) {
	t.Helper()

	kind := map[model.Profile]string{
		model.ProfileWeb: "web", model.ProfileSPA: "spa", model.ProfileNative: "native",
		model.ProfileDesktop: "native", model.ProfileMachine: "m2m",
	}[app.Profile]
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, step := range []func() error{
			func() error { return tx.Exec(`ALTER TABLE applications ALTER COLUMN type SET DEFAULT 'web'`).Error },
			func() error {
				return tx.Exec(`ALTER TABLE applications ALTER COLUMN require_pkce SET DEFAULT true`).Error
			},
			func() error { return tx.Omit(laterApplicationColumns...).Create(app).Error },
			func() error { return tx.Exec(`UPDATE applications SET type = ? WHERE id = ?`, kind, app.ID).Error },
			func() error { return tx.Exec(`ALTER TABLE applications ALTER COLUMN type DROP DEFAULT`).Error },
			func() error { return tx.Exec(`ALTER TABLE applications ALTER COLUMN require_pkce DROP DEFAULT`).Error },
		} {
			if err := step(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("an application before 00016: %v", err)
	}
}

// 00015 lets a client sign its assertions: its keys on the application, and
// the assertions already used. Going down leaves such a client on
// client_secret_basic with no secret, so it cannot authenticate until an
// administrator issues one, and up again takes private_key_jwt once more.
func TestLivePrivateKeyJWTUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	app := &model.Application{
		Name: "Job", Profile: model.ProfileMachine, ClientID: "job", TokenAuthMethod: model.AuthPrivateKeyJWT,
		JWKSURI: "https://job.example/jwks", GrantTypes: []string{model.GrantClientCredentials},
		RedirectURIs: []string{}, PostLogoutRedirectURIs: []string{}, Scopes: []string{},
	}
	if err := db.Omit(clause.Associations).Create(app).Error; err != nil {
		t.Fatalf("a private_key_jwt client at 00015: %v", err)
	}
	if err := db.Create(&model.ClientAssertion{Hash: strings.Repeat("a", 64), ExpiresAt: time.Now().Add(time.Minute)}).Error; err != nil {
		t.Fatalf("a used assertion at 00015: %v", err)
	}

	migrateDownTo(t, db, log, 14)
	if method := rows(t, db, "SELECT token_auth_method FROM applications WHERE client_id = 'job'"); !slices.Equal(method, []string{"client_secret_basic"}) {
		t.Errorf("after going down the client authenticates with %v, want client_secret_basic", method)
	}
	if secret := rows(t, db, "SELECT client_secret_hash FROM applications WHERE client_id = 'job'"); !slices.Equal(secret, []string{""}) {
		t.Errorf("after going down the client has a secret: %v", secret)
	}
	if db.Migrator().HasTable("client_assertions") || db.Migrator().HasColumn(&model.Application{}, "jwks_uri") {
		t.Error("going down left the assertions table or the keys column")
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00015 again: %v", err)
	}
	if err := db.Exec("UPDATE applications SET token_auth_method = 'private_key_jwt', jwks_uri = 'https://job.example/jwks' WHERE client_id = 'job'").Error; err != nil {
		t.Errorf("private_key_jwt after going up again: %v", err)
	}
}

// 00016 turns each application's type into its profile: m2m becomes machine
// and loses its secret, a native app sent back to loopback or signing in by
// the device grant alone becomes desktop, and one sent back to its own
// scheme stays native. Going down maps them back.
func TestLiveClientProfilesUpgradeAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 15)
	apps := []*model.Application{
		{Name: "Job", ClientID: "job", Profile: model.ProfileMachine, TokenAuthMethod: model.AuthClientSecretBasic, ClientSecretHash: "hash", SecretHint: "hint"},
		{Name: "Tool", ClientID: "tool", Profile: model.ProfileNative, TokenAuthMethod: model.AuthNone, RedirectURIs: []string{"http://127.0.0.1/callback"}},
		{Name: "Telly", ClientID: "telly", Profile: model.ProfileNative, TokenAuthMethod: model.AuthNone, GrantTypes: []string{model.GrantDeviceCode}},
		{Name: "Phone", ClientID: "phone", Profile: model.ProfileNative, TokenAuthMethod: model.AuthNone, RedirectURIs: []string{"com.example.phone:/callback"}, AllowDevelopmentRedirects: true},
		{Name: "Shop", ClientID: "shop", Profile: model.ProfileWeb, TokenAuthMethod: model.AuthClientSecretBasic, RedirectURIs: []string{"https://shop.example.com/cb"}},
	}
	for _, app := range apps {
		writeOldApplication(t, db, app)
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00016: %v", err)
	}
	profiles := rows(t, db, "SELECT client_id || '=' || profile FROM applications ORDER BY client_id")
	if want := []string{"job=machine", "phone=native", "shop=web", "telly=desktop", "tool=desktop"}; !slices.Equal(profiles, want) {
		t.Errorf("after the upgrade the profiles are %v, want %v", profiles, want)
	}
	if job := rows(t, db, "SELECT token_auth_method || ',' || client_secret_hash FROM applications WHERE client_id = 'job'"); !slices.Equal(job, []string{"private_key_jwt,"}) {
		t.Errorf("after the upgrade the machine client is %v, want private_key_jwt with no secret", job)
	}
	if phone := rows(t, db, "SELECT allow_development_redirects::text FROM applications WHERE client_id = 'phone'"); !slices.Equal(phone, []string{"true"}) {
		t.Errorf("a mobile app lost its development redirects: %v", phone)
	}
	if err := db.Exec("UPDATE applications SET profile = 'm2m' WHERE client_id = 'job'").Error; err == nil {
		t.Error("the profile column took a type 00016 replaced")
	}

	migrateDownTo(t, db, log, 15)
	types := rows(t, db, "SELECT client_id || '=' || type FROM applications ORDER BY client_id")
	if want := []string{"job=m2m", "phone=native", "shop=web", "telly=native", "tool=native"}; !slices.Equal(types, want) {
		t.Errorf("after going down the types are %v, want %v", types, want)
	}
}

// 00017 keeps a rotated secret for its overlap; going down forgets it, and a
// secret still in its overlap stops working.
func TestLiveSecretOverlapUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 16)
	if db.Migrator().HasColumn(&model.Application{}, "previous_secret_hash") {
		t.Fatal("before 00017 applications already keep a previous secret")
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00017: %v", err)
	}
	for _, column := range []string{"previous_secret_hash", "previous_secret_expires_at"} {
		if !db.Migrator().HasColumn(&model.Application{}, column) {
			t.Errorf("after 00017 applications have no %s", column)
		}
	}

	migrateDownTo(t, db, log, 16)
	if db.Migrator().HasColumn(&model.Application{}, "previous_secret_hash") {
		t.Error("going down kept the previous secret")
	}
}

// 00018 drops require_pkce, since every client uses PKCE; going down puts
// it back on for every application.
func TestLivePKCEAlwaysUpgradesAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 15)
	writeOldApplication(t, db, &model.Application{Name: "Shop", ClientID: "shop", Profile: model.ProfileWeb, TokenAuthMethod: model.AuthClientSecretBasic, IsEnabled: true})
	if err := db.Exec("UPDATE applications SET require_pkce = false").Error; err != nil {
		t.Fatal(err)
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00018: %v", err)
	}
	if db.Migrator().HasColumn(&model.Application{}, "require_pkce") {
		t.Error("after 00018 applications still have require_pkce")
	}

	migrateDownTo(t, db, log, 17)
	if on := rows(t, db, "SELECT require_pkce::text FROM applications WHERE client_id = 'shop'"); !slices.Equal(on, []string{"true"}) {
		t.Errorf("after going down require_pkce is %v, want true", on)
	}
}

// 00019 shortens access tokens to an hour, gives each application an idle
// lifetime no longer than its refresh tokens live, and each refresh token
// an idle expiry, which one from before does without.
func TestLiveTokenLifetimesUpgradeAnExistingDatabase(t *testing.T) {
	db, _ := databasetest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	migrateDownTo(t, db, log, 18)
	for _, row := range []struct{ id string }{{"long"}, {"short"}} {
		if err := db.Exec(`INSERT INTO applications (id, created_at, updated_at, name, profile, client_id, token_auth_method,
			access_token_lifetime, id_token_lifetime, refresh_token_lifetime, assert_roles, require_pushed_requests, allow_registration, is_enabled)
			VALUES (gen_random_uuid(), now(), now(), ?, 'web', ?, 'client_secret_basic', 86400, 3600, ?, true, false, true, true)`,
			row.id, row.id, map[string]int{"long": 30 * 86400, "short": 7 * 86400}[row.id]).Error; err != nil {
			t.Fatalf("an application at 00018: %v", err)
		}
	}

	if err := database.Migrate(db, log); err != nil {
		t.Fatalf("up to 00019: %v", err)
	}
	got := rows(t, db, "SELECT client_id || '=' || access_token_lifetime || ',' || refresh_token_idle_lifetime FROM applications ORDER BY client_id")
	if want := []string{"long=3600,1209600", "short=3600,604800"}; !slices.Equal(got, want) {
		t.Errorf("after the upgrade = %v, want %v", got, want)
	}

	migrateDownTo(t, db, log, 18)
	if db.Migrator().HasColumn(&model.Application{}, "refresh_token_idle_lifetime") || db.Migrator().HasColumn(&model.RefreshToken{}, "idle_expires_at") {
		t.Error("going down left the idle lifetime")
	}
}
