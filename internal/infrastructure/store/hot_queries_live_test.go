package store

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/a1i6ek/vard/internal/infrastructure/database/databasetest"
)

// What every request, sign-in or token asks the database, and what the panel
// asks of the tables that grow without bound, is answered through an index:
// with sequential scans priced out, none of these plans reads a table whole.
// A table of millions read whole on each request is the first thing to fail
// at scale, and on the few rows a test has the planner would scan anyway, so
// the price is what makes the test say anything.
func TestLiveTheHotQueriesUseAnIndex(t *testing.T) {
	db, _ := databasetest.Open(t)
	now := time.Now()
	hash := strings.Repeat("a", 64)
	id := uuid.New()

	tests := []struct {
		name  string
		query string
		args  []any
	}{
		{"a person's session by its cookie", `SELECT * FROM user_sessions WHERE token_hash = ? LIMIT 1`, []any{hash}},
		{"an administrator's session by its cookie", `SELECT * FROM admin_sessions WHERE token_hash = ? LIMIT 1`, []any{hash}},
		{"a refresh token", `SELECT * FROM refresh_tokens WHERE token_hash = ? LIMIT 1`, []any{hash}},
		{"a refresh family's live token", `SELECT * FROM refresh_tokens WHERE family_id = ? AND revoked_at IS NULL`, []any{id}},
		{"an authorization code, once", `UPDATE authorization_codes SET used_at = ? WHERE code_hash = ? AND application_id = ? AND used_at IS NULL`, []any{now, hash, id}},
		{"a waiting authorization request", `SELECT * FROM authorization_requests WHERE handle_hash = ? LIMIT 1`, []any{hash}},
		{"a pushed request", `SELECT * FROM pushed_requests WHERE handle_hash = ? LIMIT 1`, []any{hash}},
		{"a device code", `SELECT * FROM device_authorizations WHERE device_code_hash = ? LIMIT 1`, []any{hash}},
		{"a waiting sign-in", `SELECT * FROM pending_sign_ins WHERE handle_hash = ? LIMIT 1`, []any{hash}},
		{"whether an access token was revoked", `SELECT count(*) FROM token_revocations WHERE key IN (?, ?, ?) AND revoked_at >= ?`, []any{"jti:x", "sid:y", "fid:z", now}},
		{"a sign-in's throttle", `SELECT * FROM sign_in_throttles WHERE key IN (?, ?)`, []any{"ip:x", "email:y"}},
		{"locked administrators", `SELECT count(*) FROM sign_in_throttles WHERE key LIKE 'admin:%' AND blocked_until > ?`, []any{now}},
		{"a person by address", `SELECT * FROM users WHERE email = ? LIMIT 1`, []any{"ada@example.com"}},
		{"a person's sessions", `SELECT * FROM user_sessions WHERE user_id = ? AND revoked_at IS NULL`, []any{id}},
		{"the newest users", `SELECT * FROM users ORDER BY created_at DESC LIMIT 50`, nil},
		{"a page of the log", `SELECT * FROM audit_logs WHERE (created_at, id) < (?, ?) ORDER BY created_at DESC, id DESC LIMIT 51`, []any{now, id}},
		{"the log of one kind", `SELECT * FROM audit_logs WHERE action IN ('user.login_failed', 'user.login_blocked') ORDER BY created_at DESC LIMIT 51`, nil},
		{"the log of one actor", `SELECT * FROM audit_logs WHERE actor_email = ? ORDER BY created_at DESC LIMIT 51`, []any{"root@example.com"}},
		{"an API's log", `SELECT * FROM audit_logs WHERE (target_type = 'api' AND target_id = ?) OR metadata ->> 'api_id' = ? ORDER BY created_at DESC LIMIT 50`, []any{"x", "x"}},
		{"expired rows to sweep", `DELETE FROM refresh_tokens WHERE expires_at < ?`, []any{now}},
		{"due logout notices", `SELECT * FROM logout_deliveries WHERE status = 'pending' AND next_try_at <= ? LIMIT 50`, []any{now}},
	}

	tx := db.Begin()
	defer tx.Rollback()
	if err := tx.Exec("SET LOCAL enable_seqscan = off").Error; err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var lines []string
			if err := tx.Raw("EXPLAIN "+test.query, test.args...).Scan(&lines).Error; err != nil {
				t.Fatal(err)
			}
			if plan := strings.Join(lines, "\n"); strings.Contains(plan, "Seq Scan") {
				t.Errorf("read whole:\n%s", plan)
			}
		})
	}
}
