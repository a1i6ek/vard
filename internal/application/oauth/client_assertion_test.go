package oauth

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/a1i6ek/vard/internal/application/authreq"
	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
	"github.com/a1i6ek/vard/internal/domain/model"
)

// Every check an assertion has to pass before its jti is spent: a failure
// here never reaches the store, so a forged or stale assertion cannot use up
// a real one's jti. The jti itself is proved in tests/integration.
func TestAClientAssertionIsCheckedBeforeItsJTIIsSpent(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	const issuer = "https://id.example"

	key := testKey(t, jose.ES256, "k1")
	jwk, err := key.Public()
	if err != nil {
		t.Fatal(err)
	}
	app := &model.Application{ClientID: "job", TokenAuthMethod: model.AuthPrivateKeyJWT, JWKS: []jose.JWK{jwk}}
	s := &Service{issuer: issuer, now: func() time.Time { return now }, log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	sign := func(key jose.Key, change func(map[string]any)) string {
		claims := map[string]any{
			"iss": "job", "sub": "job", "aud": issuer,
			"exp": now.Add(time.Minute).Unix(), "jti": "one",
		}
		change(claims)
		signed, err := jose.Sign(key, "JWT", claims)
		if err != nil {
			t.Fatal(err)
		}
		return signed
	}

	tests := []struct {
		name      string
		assertion string
		want      string
	}{
		{"signed by another key", sign(testKey(t, jose.ES256, "k1"), func(map[string]any) {}), "not signed by one of the client's keys"},
		{"unsigned", "eyJhbGciOiJub25lIn0.eyJpc3MiOiJqb2IifQ.", "not signed by one of the client's keys"},
		{"from another client", sign(key, func(c map[string]any) { c["iss"], c["sub"] = "other", "other" }), "iss and sub"},
		{"with a subject that is not the client", sign(key, func(c map[string]any) { c["sub"] = "ada" }), "iss and sub"},
		{"for another server", sign(key, func(c map[string]any) { c["aud"] = "https://other.example" }), "not addressed to this server"},
		{"for an endpoint of another server", sign(key, func(c map[string]any) { c["aud"] = "https://other.example/oauth2/token" }), "not addressed to this server"},
		{"with no expiry", sign(key, func(c map[string]any) { delete(c, "exp") }), "no exp"},
		{"expired past the leeway", sign(key, func(c map[string]any) { c["exp"] = now.Add(-time.Minute).Unix() }), "expired"},
		{"expiring too far ahead", sign(key, func(c map[string]any) { c["exp"] = now.Add(6 * time.Minute).Unix() }), "too far ahead"},
		{"not valid yet", sign(key, func(c map[string]any) { c["nbf"] = now.Add(time.Minute).Unix() }), "not valid yet"},
		{"issued in the future", sign(key, func(c map[string]any) { c["iat"] = now.Add(time.Minute).Unix() }), "issued in the future"},
		{"with no jti", sign(key, func(c map[string]any) { delete(c, "jti") }), "needs a jti"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.verifyAssertion(context.Background(), app, tt.assertion, authreq.PathToken)
			oauthErr, ok := err.(*authreq.Error)
			if !ok || oauthErr.Code != authreq.ErrInvalidClient {
				t.Fatalf("verifyAssertion = %v, want invalid_client", err)
			}
			if !strings.Contains(oauthErr.Description, tt.want) {
				t.Errorf("verifyAssertion = %q, want it to say %q", oauthErr.Description, tt.want)
			}
		})
	}
}

// aud names this server by its issuer, or by the token endpoint or the
// endpoint the assertion was sent to, as a string or among a list.
func TestAClientAssertionIsAddressedHere(t *testing.T) {
	s := &Service{issuer: "https://id.example"}

	tests := []struct {
		name     string
		audience any
		endpoint string
		want     bool
	}{
		{"the issuer", "https://id.example", authreq.PathToken, true},
		{"the token endpoint", "https://id.example/oauth2/token", authreq.PathIntrospect, true},
		{"the endpoint it was sent to", "https://id.example/oauth2/par", authreq.PathPushedAuthorization, true},
		{"another endpoint than it was sent to", "https://id.example/oauth2/par", authreq.PathRevoke, false},
		{"the issuer among others", []string{"https://other.example", "https://id.example"}, authreq.PathToken, true},
		{"another server alone", []string{"https://other.example"}, authreq.PathToken, false},
		{"the issuer with a trailing slash", "https://id.example/", authreq.PathToken, false},
		{"a number", 7, authreq.PathToken, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(tt.audience)
			if got := s.addressedHere(raw, tt.endpoint); got != tt.want {
				t.Errorf("addressedHere(%s) = %v, want %v", raw, got, tt.want)
			}
		})
	}
}

func testKey(t *testing.T, algorithm, kid string) jose.Key {
	t.Helper()

	private, err := jose.Generate(algorithm)
	if err != nil {
		t.Fatal(err)
	}
	return jose.Key{ID: kid, Algorithm: algorithm, Private: private}
}
