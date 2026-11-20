package integration

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// A resource server that asks for application/token-introspection+jwt is
// answered with the introspection signed by this server (RFC 9701):
// addressed to it, verifiable with the published keys, the answer under
// token_introspection, for a live token and a dead one alike.
func TestLiveIntrospectionMayBeAnsweredSigned(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	b := f.s.browser()
	verifier, challenge := pkce(t)
	tokens := f.exchange(f.signIn(b, challenge).Query().Get("code"), verifier)
	if tokens.status != http.StatusOK {
		t.Fatalf("signing in = %d %v", tokens.status, tokens.body)
	}

	introspect := func(token, accept string) (*http.Response, string) {
		req, _ := http.NewRequest(http.MethodPost, f.s.root+"/oauth2/introspect", strings.NewReader(url.Values{"token": {token}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", accept)
		req.SetBasicAuth(url.QueryEscape(f.clientID), url.QueryEscape(f.clientSecret))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res, string(body)
	}

	tests := []struct {
		name   string
		token  string
		active bool
	}{
		{name: "a live access token", token: tokens.str("access_token"), active: true},
		{name: "a token this server never issued", token: "not-a-token", active: false},
	}
	for _, tt := range tests {
		res, body := introspect(tt.token, "application/token-introspection+jwt")
		if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/token-introspection+jwt" {
			t.Errorf("%s: %d %q, want a signed answer", tt.name, res.StatusCode, res.Header.Get("Content-Type"))
			continue
		}

		header, claims := f.verifyJWT(body)
		answer, _ := claims["token_introspection"].(map[string]any)
		switch {
		case header["typ"] != "token-introspection+jwt":
			t.Errorf("%s: typ = %v", tt.name, header["typ"])
		case claims["iss"] != f.s.cfg.Issuer || claims["aud"] != f.clientID:
			t.Errorf("%s: iss %v, aud %v, want this server to the client that asked", tt.name, claims["iss"], claims["aud"])
		case answer["active"] != tt.active:
			t.Errorf("%s: token_introspection = %v, want active %v", tt.name, answer, tt.active)
		case tt.active && answer["sub"] != f.userID:
			t.Errorf("%s: sub = %v, want Ada", tt.name, answer["sub"])
		}
	}

	if res, _ := introspect(tokens.str("access_token"), "application/json"); res.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("asked for JSON: %q, want the plain answer", res.Header.Get("Content-Type"))
	}
}
