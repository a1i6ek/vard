package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
	"github.com/a1i6ek/vard/internal/domain/model"
)

// clientKey is a key a private_key_jwt client holds; its public half is what
// it registers.
func clientKey(t *testing.T, algorithm, kid string) jose.Key {
	t.Helper()

	private, err := jose.Generate(algorithm)
	if err != nil {
		t.Fatal(err)
	}
	return jose.Key{ID: kid, Algorithm: algorithm, Private: private}
}

func publicSet(t *testing.T, keys ...jose.Key) map[string]any {
	t.Helper()

	set := []jose.JWK{}
	for _, key := range keys {
		jwk, err := key.Public()
		if err != nil {
			t.Fatal(err)
		}
		set = append(set, jwk)
	}
	return map[string]any{"keys": set}
}

// assertion is a client assertion (RFC 7523 §2.2), with claims changed by
// change.
func assertion(t *testing.T, key jose.Key, clientID, audience string, change func(map[string]any)) string {
	t.Helper()

	claims := map[string]any{
		"iss": clientID, "sub": clientID, "aud": audience,
		"exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(),
		"jti": randomHex(t, 16),
	}
	if change != nil {
		change(claims)
	}
	signed, err := jose.Sign(key, "JWT", claims)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

// asserted calls a client-authenticating endpoint with an assertion.
func (f *oauthFixture) asserted(path, clientID, signed string, form url.Values) tokenResult {
	f.s.t.Helper()

	form.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	form.Set("client_assertion", signed)
	return f.token(path, clientID, "", form)
}

// A private_key_jwt client authenticates with an assertion signed by a key
// it registered, at every endpoint a client authenticates at: token,
// introspection, revocation, pushed requests and device authorization. Its
// keys may be inline or published at its own jwks_uri.
func TestLivePrivateKeyJWTAuthenticatesAtEveryEndpoint(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	issuer := f.s.root

	key := clientKey(t, jose.ES256, "machine-1")
	machineID, secret, machineApp := f.register(map[string]any{
		"name": "Nightly job", "profile": "machine",
		"token_auth_method": "private_key_jwt", "jwks": publicSet(t, key),
	})
	if secret != "" {
		t.Fatalf("a private_key_jwt client was given a secret")
	}
	f.authorizeAPI(machineApp, "orders:read")

	got := f.asserted("/oauth2/token", machineID, assertion(t, key, machineID, issuer, nil), url.Values{
		"grant_type": {"client_credentials"}, "resource": {ordersAPI},
	})
	if got.status != http.StatusOK || got.str("access_token") == "" {
		t.Fatalf("client_credentials with an assertion = %d %v", got.status, got.body)
	}
	token := got.str("access_token")

	introspected := f.asserted("/oauth2/introspect", machineID, assertion(t, key, machineID, issuer, nil), url.Values{"token": {token}})
	if introspected.status != http.StatusOK || introspected.body["active"] != true {
		t.Errorf("introspection with an assertion = %d %v", introspected.status, introspected.body)
	}
	revoked := f.asserted("/oauth2/revoke", machineID, assertion(t, key, machineID, issuer, nil), url.Values{"token": {token}})
	if revoked.status != http.StatusOK {
		t.Errorf("revocation with an assertion = %d %v", revoked.status, revoked.body)
	}

	// Keys at the client's own address, fetched through the outbound client.
	webKey := clientKey(t, jose.PS256, "web-1")
	published := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(publicSet(t, webKey))
	}))
	t.Cleanup(published.Close)

	webID, _, _ := f.register(map[string]any{
		"name": "Portal", "profile": "web",
		"token_auth_method": "private_key_jwt", "jwks_uri": published.URL,
		"grant_types":   []string{"authorization_code", model.GrantDeviceCode},
		"redirect_uris": []string{shopRedirect}, "scopes": []string{"openid"},
	})
	_, challenge := pkce(t)
	pushed := f.asserted("/oauth2/par", webID, assertion(t, webKey, webID, issuer, nil), url.Values{
		"response_type": {"code"}, "redirect_uri": {shopRedirect}, "scope": {"openid"},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"},
	})
	if pushed.status != http.StatusCreated || pushed.str("request_uri") == "" {
		t.Errorf("a pushed request with an assertion = %d %v", pushed.status, pushed.body)
	}
	device := f.asserted("/oauth2/device_authorization", webID, assertion(t, webKey, webID, issuer, nil), url.Values{"scope": {"openid"}})
	if device.status != http.StatusOK || device.str("device_code") == "" {
		t.Errorf("device authorization with an assertion = %d %v", device.status, device.body)
	}
}

// An assertion proves the client only when everything about it holds; each
// way it can fail is refused as invalid_client, and the client's secret, a
// second way to authenticate, is never taken instead.
func TestLiveAClientAssertionIsRefused(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	issuer := f.s.root

	key := clientKey(t, jose.ES256, "job-1")
	clientID, _, appID := f.register(map[string]any{
		"name": "Job", "profile": "machine",
		"token_auth_method": "private_key_jwt", "jwks": publicSet(t, key),
	})
	f.authorizeAPI(appID, "orders:read")

	grant := func() url.Values {
		return url.Values{"grant_type": {"client_credentials"}, "resource": {ordersAPI}}
	}

	used := assertion(t, key, clientID, issuer, nil)
	if first := f.asserted("/oauth2/token", clientID, used, grant()); first.status != http.StatusOK {
		t.Fatalf("the first use of an assertion = %d %v", first.status, first.body)
	}

	stranger := clientKey(t, jose.ES256, "job-1")
	tests := []struct {
		name   string
		signed string
	}{
		{"an assertion used before", used},
		{"signed by a key the client did not register", assertion(t, stranger, clientID, issuer, nil)},
		{"addressed to another server", assertion(t, key, clientID, "https://other.example", nil)},
		{"expired", assertion(t, key, clientID, issuer, func(c map[string]any) { c["exp"] = time.Now().Add(-2 * time.Minute).Unix() })},
		{"expiring more than five minutes ahead", assertion(t, key, clientID, issuer, func(c map[string]any) { c["exp"] = time.Now().Add(time.Hour).Unix() })},
		{"without an expiry", assertion(t, key, clientID, issuer, func(c map[string]any) { delete(c, "exp") })},
		{"without a jti", assertion(t, key, clientID, issuer, func(c map[string]any) { delete(c, "jti") })},
		{"with another subject", assertion(t, key, clientID, issuer, func(c map[string]any) { c["sub"] = "someone-else" })},
		{"not valid yet", assertion(t, key, clientID, issuer, func(c map[string]any) { c["nbf"] = time.Now().Add(3 * time.Minute).Unix() })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := f.asserted("/oauth2/token", clientID, tt.signed, grant())
			if got.status != http.StatusUnauthorized || got.str("error") != "invalid_client" {
				t.Errorf("%s = %d %v, want invalid_client", tt.name, got.status, got.body)
			}
		})
	}

	// A secret is not this client's way in, whatever it sends.
	if got := f.token("/oauth2/token", clientID, "a-guessed-secret", grant()); got.status != http.StatusUnauthorized || got.str("error") != "invalid_client" {
		t.Errorf("a secret for a private_key_jwt client = %d %v", got.status, got.body)
	}
	// Two ways at once, or an assertion of another type, is a malformed request.
	both := grant()
	both.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	both.Set("client_assertion", assertion(t, key, clientID, issuer, nil))
	if got := f.token("/oauth2/token", clientID, "a-secret-too", both); got.status != http.StatusBadRequest || got.str("error") != "invalid_request" {
		t.Errorf("an assertion and a secret together = %d %v", got.status, got.body)
	}
	saml := grant()
	saml.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:saml2-bearer")
	saml.Set("client_assertion", assertion(t, key, clientID, issuer, nil))
	if got := f.token("/oauth2/token", clientID, "", saml); got.status != http.StatusBadRequest || got.str("error") != "invalid_request" {
		t.Errorf("an assertion of another type = %d %v", got.status, got.body)
	}
}

// aud is the issuer (RFC 7523bis); clients written before that name the
// token endpoint, or the endpoint they send to (RFC 9126 §2), and are taken
// too. A client_id left out is read from the assertion.
func TestLiveAClientAssertionMayNameTheEndpoint(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)

	key := clientKey(t, jose.RS256, "job-rsa")
	clientID, _, appID := f.register(map[string]any{
		"name": "Job", "profile": "machine",
		"token_auth_method": "private_key_jwt", "jwks": publicSet(t, key),
	})
	f.authorizeAPI(appID, "orders:read")

	for _, audience := range []string{f.s.root + "/oauth2/token", f.s.root + "/oauth2/introspect"} {
		got := f.asserted("/oauth2/introspect", clientID, assertion(t, key, clientID, audience, nil), url.Values{"token": {"not-a-token"}})
		if got.status != http.StatusOK {
			t.Errorf("an assertion addressed to %s at introspection = %d %v", audience, got.status, got.body)
		}
	}

	// Without client_id in the form: the assertion's iss names the client.
	form := url.Values{
		"grant_type": {"client_credentials"}, "resource": {ordersAPI},
		"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
		"client_assertion":      {assertion(t, key, clientID, f.s.root, nil)},
	}
	if got := f.formPost("/oauth2/token", form); got.status != http.StatusOK {
		t.Errorf("an assertion with no client_id beside it = %d %v", got.status, got.body)
	}
}

// formPost posts a form exactly as given, with no client credentials added;
// a client_credentials request carries a proof, as a machine's must.
func (f *oauthFixture) formPost(path string, form url.Values) tokenResult {
	f.s.t.Helper()

	req, _ := http.NewRequest(http.MethodPost, f.s.root+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if form.Get("grant_type") == "client_credentials" && form.Get("client_assertion") != "" {
		req.Header.Set("DPoP", newDPoPKey(f.s.t).proof(f.s.t, "POST", f.s.cfg.Issuer+path, ""))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		f.s.t.Fatal(err)
	}
	defer res.Body.Close()

	result := tokenResult{status: res.StatusCode, body: map[string]any{}}
	_ = json.NewDecoder(res.Body).Decode(&result.body)
	return result
}

// A client registering itself (RFC 7591) may register keys instead of being
// given a secret, and what the panel refuses about them, registration
// refuses.
func TestLiveARegisteredClientMayUsePrivateKeyJWT(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	cliID := f.adminCLI()
	adminToken := f.adminCLITokens(cliID)
	apiID, scopes := f.systemAPI(model.SystemAdminAPI)
	f.allow(cliID, apiID, scopes, model.PermApplicationsWrite)
	token := adminToken()

	key := clientKey(t, jose.ES256, "portal-1")
	status, body := f.bearer(http.MethodPost, f.s.root+"/oauth2/register", token, map[string]any{
		"client_name": "Portal", "redirect_uris": []string{"https://partner.example.com/callback"},
		"token_endpoint_auth_method": "private_key_jwt", "jwks": publicSet(t, key),
	})
	if status != http.StatusCreated || body["client_secret"] != nil || body["token_endpoint_auth_method"] != "private_key_jwt" || body["jwks"] == nil {
		t.Fatalf("registering a private_key_jwt client = %d %v", status, body)
	}

	status, body = f.bearer(http.MethodPost, f.s.root+"/oauth2/register", token, map[string]any{
		"client_name": "Keyless", "redirect_uris": []string{"https://partner.example.com/callback"},
		"token_endpoint_auth_method": "private_key_jwt",
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_client_metadata" {
		t.Errorf("registering private_key_jwt with no keys = %d %v", status, body)
	}
}

// A machine client's token (client_credentials) is for one named API, bound
// to the key that asked for it, and short: there is no refresh token, and no
// bearer token without a proof.
func TestLiveAMachineTokenIsBoundShortAndForOneAPI(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	key := clientKey(t, jose.ES256, "machine-1")
	clientID, _, appID := f.register(map[string]any{
		"name": "Nightly job", "profile": "machine",
		"token_auth_method": "private_key_jwt", "jwks": publicSet(t, key),
		"dpop_bound_access_tokens": false, "access_token_lifetime": 15 * 60,
	})
	f.authorizeAPI(appID, "orders:read")

	signed := func(values url.Values) url.Values {
		values.Set("client_id", clientID)
		values.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
		values.Set("client_assertion", assertion(t, key, clientID, f.s.root, nil))
		return values
	}
	refusals := []struct {
		name  string
		form  url.Values
		proof bool
		want  string
	}{
		{name: "no DPoP proof", form: url.Values{"grant_type": {"client_credentials"}, "resource": {ordersAPI}}, want: "invalid_dpop_proof"},
		{name: "no resource", form: url.Values{"grant_type": {"client_credentials"}}, proof: true, want: "invalid_target"},
	}
	for _, tt := range refusals {
		proof := ""
		if tt.proof {
			proof = newDPoPKey(t).proof(t, "POST", f.s.cfg.Issuer+"/oauth2/token", "")
		}
		if got := formPost(t, f, "/oauth2/token", clientID, "", proof, signed(tt.form)); got.status != http.StatusBadRequest || got.str("error") != tt.want {
			t.Errorf("%s = %d %v, want %s", tt.name, got.status, got.body, tt.want)
		}
	}

	bound := newDPoPKey(t)
	got := formPost(t, f, "/oauth2/token", clientID, "", bound.proof(t, "POST", f.s.cfg.Issuer+"/oauth2/token", ""),
		signed(url.Values{"grant_type": {"client_credentials"}, "resource": {ordersAPI}}))
	if got.status != http.StatusOK || got.str("token_type") != "DPoP" {
		t.Fatalf("a token with a proof = %d %v, want a DPoP-bound token", got.status, got.body)
	}
	if _, ok := got.body["refresh_token"]; ok {
		t.Error("a machine was given a refresh token")
	}
	if lasts, _ := got.body["expires_in"].(float64); lasts <= 0 || lasts > 15*60 {
		t.Errorf("expires_in = %v, want at most fifteen minutes", got.body["expires_in"])
	}
	_, claims := f.verifyJWT(got.str("access_token"))
	if cnf, _ := claims["cnf"].(map[string]any); cnf["jkt"] != bound.thumbprint(t) {
		t.Errorf("cnf = %v, want the proof's key", claims["cnf"])
	}

	f.super.must(http.StatusBadRequest, http.MethodPatch, "/applications/"+appID, map[string]any{"access_token_lifetime": 16 * 60}, nil)
}
