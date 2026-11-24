package oidc

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a1i6ek/vard/internal/infrastructure/outbound"
)

// A logout token goes to the URI the application registered and nowhere else:
// a redirect is answered as it is, not followed, and loopback is refused in
// production like every other outbound call.
func TestBackchannelClient(t *testing.T) {
	followed := false
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		followed = true
	}))
	defer elsewhere.Close()

	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL, http.StatusTemporaryRedirect)
	}))
	defer redirecting.Close()

	resp, err := newBackchannelClient(true).Post(redirecting.URL, "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect || followed {
		t.Errorf("status = %d, followed = %v; want the redirect answered and not followed", resp.StatusCode, followed)
	}

	if _, err := newBackchannelClient(false).Post(redirecting.URL, "application/x-www-form-urlencoded", nil); !errors.Is(err, outbound.ErrBlockedAddress) {
		t.Errorf("err = %v, want loopback refused outside development", err)
	}
}
