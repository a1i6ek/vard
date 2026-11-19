package federation

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

// documentAt answers every request with a discovery document naming issuer,
// and records the address asked for.
type documentAt struct {
	issuer string
	asked  string
}

func (d *documentAt) RoundTrip(r *http.Request) (*http.Response, error) {
	d.asked = r.URL.String()
	body, _ := json.Marshal(map[string]string{
		"issuer": d.issuer, "authorization_endpoint": d.issuer + "/authorize",
		"token_endpoint": d.issuer + "/token", "jwks_uri": d.issuer + "/jwks",
	})
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body))), Header: http.Header{}}, nil
}

// The issuer a provider's document names must be the connection's exactly: a
// trailing slash, either way, is another issuer (OpenID Connect Discovery
// §4.3). The document's address is built without the slash (§4).
func TestDiscoveryTakesOnlyTheIssuerExactly(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		published  string
		want       bool
	}{
		{"the same issuer", "https://idp.example", "https://idp.example", true},
		{"the same issuer with a path", "https://idp.example/realms/acme", "https://idp.example/realms/acme", true},
		{"a slash the connection has and the document has not", "https://idp.example/", "https://idp.example", false},
		{"a slash the document has and the connection has not", "https://idp.example", "https://idp.example/", false},
		{"another host", "https://idp.example", "https://evil.example", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			published := &documentAt{issuer: tt.published}
			s := &Service{
				log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
				social:   &http.Client{Transport: published},
				ssoCache: newSSOCache(),
				now:      time.Now,
			}

			_, err := s.Discover(context.Background(), tt.configured)
			if got := err == nil; got != tt.want {
				t.Errorf("Discover(%q) with a document for %q = %v, want accepted %v", tt.configured, tt.published, err, tt.want)
			}
			if want := strings.TrimSuffix(tt.configured, "/") + "/.well-known/openid-configuration"; published.asked != want {
				t.Errorf("asked %s, want %s", published.asked, want)
			}
		})
	}
}
