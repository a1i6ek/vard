package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/a1i6ek/vard/internal/cache"
	"github.com/a1i6ek/vard/internal/cache/cachetest"
)

// The session database keeps administrators without their password, so a
// profile saved from a request signed in through it has to write back the
// password in the database, not an empty one: afterwards the administrator
// still signs in with it, and a new one still works.
func TestLiveAProfileSavedFromTheSessionCacheKeepsThePassword(t *testing.T) {
	t.Parallel()

	s := newLiveServer(t)
	super := s.superAdmin()

	// Twice, so the second request is answered from Redis when there is one.
	super.must(http.StatusOK, http.MethodGet, "/me", nil, nil)
	super.must(http.StatusOK, http.MethodPatch, "/me", map[string]string{
		"first_name": "Rooted", "email": superEmail,
	}, nil)

	again := s.client()
	if status := again.login(superEmail, superPassword); status != http.StatusOK {
		t.Fatalf("signing in after a profile save = %d, want the password kept", status)
	}

	super.must(http.StatusOK, http.MethodPost, "/me/password", map[string]string{
		"current_password": superPassword, "new_password": "root-password-2",
	}, nil)

	if status := s.client().login(superEmail, "root-password-2"); status != http.StatusOK {
		t.Errorf("signing in with the new password = %d, want 200", status)
	}
}

// Whatever a request was answered from, a revoked session and a changed role
// take effect on the next request.
func TestLiveChangesReachTheNextRequest(t *testing.T) {
	t.Parallel()

	s := newLiveServer(t)
	super := s.superAdmin()

	laptop := s.client()
	if status := laptop.login(superEmail, superPassword); status != http.StatusOK {
		t.Fatalf("second sign-in = %d", status)
	}
	laptop.must(http.StatusOK, http.MethodGet, "/me", nil, nil)
	laptop.must(http.StatusOK, http.MethodGet, "/me", nil, nil)

	// Signing out everywhere else ends the laptop's session at once.
	super.must(http.StatusOK, http.MethodDelete, "/sessions", nil, nil)
	if status := laptop.do(http.MethodGet, "/me", nil, nil); status != http.StatusUnauthorized {
		t.Errorf("a session ended elsewhere = %d, want 401", status)
	}

	// Signing out ends this one.
	super.must(http.StatusOK, http.MethodGet, "/me", nil, nil)
	super.must(http.StatusOK, http.MethodPost, "/auth/logout", nil, nil)
	if status := super.do(http.MethodGet, "/me", nil, nil); status != http.StatusUnauthorized {
		t.Errorf("a session after signing out = %d, want 401", status)
	}
}

// Sessions are read from Postgres on every request, so ending one holds on
// every server at once: none is kept in Redis, even while an administrator is
// signed in and the cache is in use.
func TestLiveNoSessionIsKeptInRedis(t *testing.T) {
	t.Parallel()

	s := newLiveServer(t)
	if s.cache == nil {
		t.Skip("no test Redis: set " + cachetest.Env)
	}
	super := s.superAdmin()

	// Something reads the organisation, so the cache is in use.
	super.must(http.StatusOK, http.MethodGet, "/organization", nil, nil)

	held := s.cache.SessionDB()
	if held == nil {
		t.Fatal("no session database")
	}
	keys, _, err := held.Keys(context.Background(), cache.KeyQuery{Search: "session:"})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Errorf("Redis holds %d session keys, want none", len(keys))
	}
}
