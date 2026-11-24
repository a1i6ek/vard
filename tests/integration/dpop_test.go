package integration

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/a1i6ek/vard/internal/platform/brand"
	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
	"github.com/a1i6ek/vard/internal/domain/model"
)

// dpopKey is a client's key, making DPoP proofs as a client library would.
type dpopKey struct {
	key *ecdsa.PrivateKey
	jwk jose.JWK
}

func newDPoPKey(t *testing.T) *dpopKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	public, err := key.PublicKey.ECDH()
	if err != nil {
		t.Fatal(err)
	}
	point := public.Bytes()

	return &dpopKey{key: key, jwk: jose.JWK{
		KeyType: "EC", Curve: "P-256",
		X: b64.EncodeToString(point[1:33]), Y: b64.EncodeToString(point[33:65]),
	}}
}

func (k *dpopKey) thumbprint(t *testing.T) string {
	t.Helper()
	thumbprint, err := jose.Thumbprint(k.jwk)
	if err != nil {
		t.Fatal(err)
	}
	return thumbprint
}

// proof is a DPoP proof for one request, carrying the nonce the server it
// is for gives now; accessToken "" leaves out ath.
func (k *dpopKey) proof(t *testing.T, method, target, accessToken string) string {
	t.Helper()
	return k.proofWith(t, method, target, accessToken, dpopNonce(t, target))
}

// dpopNonce is the nonce the server at target's origin gives now, from the
// DPoP-Nonce header its token endpoint sends with every answer.
func dpopNonce(t *testing.T, target string) string {
	t.Helper()

	at, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.PostForm(at.Scheme+"://"+at.Host+"/oauth2/token", url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	nonce := res.Header.Get("DPoP-Nonce")
	if nonce == "" {
		t.Fatal("the token endpoint sent no DPoP-Nonce")
	}
	return nonce
}

// proofWith is a proof with the nonce given, "" for none.
func (k *dpopKey) proofWith(t *testing.T, method, target, accessToken, nonce string) string {
	t.Helper()

	header, _ := json.Marshal(map[string]any{
		"typ": "dpop+jwt", "alg": "ES256",
		"jwk": map[string]string{"kty": "EC", "crv": "P-256", "x": k.jwk.X, "y": k.jwk.Y},
	})
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	claims := map[string]any{"jti": b64.EncodeToString(id), "htm": method, "htu": target, "iat": time.Now().Unix()}
	if accessToken != "" {
		sum := sha256.Sum256([]byte(accessToken))
		claims["ath"] = b64.EncodeToString(sum[:])
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	payload, _ := json.Marshal(claims)

	input := b64.EncodeToString(header) + "." + b64.EncodeToString(payload)
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, k.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])

	return input + "." + b64.EncodeToString(signature)
}

// postToken is the token endpoint with a DPoP proof, for a public client.
func postToken(t *testing.T, f *oauthFixture, clientID, proof string, form url.Values) tokenResult {
	t.Helper()

	form.Set("client_id", clientID)
	req, _ := http.NewRequest(http.MethodPost, f.s.root+"/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if proof != "" {
		req.Header.Set("DPoP", proof)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	result := tokenResult{status: res.StatusCode, body: map[string]any{}}
	_ = json.NewDecoder(res.Body).Decode(&result.body)
	return result
}

// A client holding a key gets tokens bound to it: an access token userinfo
// takes only as DPoP with a fresh proof from that key, and a refresh token
// that needs one too. Copied without the key, neither is any use.
func TestLiveDPoP(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	spaID, _, _ := f.register(map[string]any{
		"name": "Single page", "profile": "spa",
		"grant_types":   []string{"authorization_code", "refresh_token"},
		"redirect_uris": []string{shopRedirect},
		"scopes":        []string{"openid", "email", "offline_access"},
	})
	f.clientID, f.clientSecret = spaID, ""

	key := newDPoPKey(t)
	tokenURL := f.s.cfg.Issuer + "/oauth2/token"
	userinfoURL := f.s.cfg.Issuer + "/oauth2/userinfo"

	b := f.s.browser()
	verifier, challenge := pkce(t)
	back := f.signInWith(b, challenge, url.Values{"scope": {"openid email offline_access"}, "resource": {""}})
	tokens := postToken(t, f, spaID, key.proof(t, "POST", tokenURL, ""), url.Values{
		"grant_type": {"authorization_code"}, "code": {back.Query().Get("code")},
		"redirect_uri": {shopRedirect}, "code_verifier": {verifier},
	})
	if tokens.status != http.StatusOK || tokens.str("token_type") != "DPoP" {
		t.Fatalf("the code with a proof = %d %v", tokens.status, tokens.body)
	}
	access := tokens.str("access_token")
	if _, claims := f.verifyJWT(access); claims["cnf"].(map[string]any)["jkt"] != key.thumbprint(t) {
		t.Errorf("the access token's cnf is %v, want the key's thumbprint", claims["cnf"])
	}

	userinfo := func(scheme, proof string) (int, string) {
		req, _ := http.NewRequest(http.MethodGet, f.s.root+"/oauth2/userinfo", nil)
		req.Header.Set("Authorization", scheme+" "+access)
		if proof != "" {
			req.Header.Set("DPoP", proof)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body)
	}

	proof := key.proof(t, "GET", userinfoURL, access)
	if status, body := userinfo("DPoP", proof); status != http.StatusOK {
		t.Fatalf("userinfo with the token and its proof = %d %s", status, body)
	}

	tests := []struct {
		name   string
		scheme string
		proof  string
	}{
		{"as a bearer token, without the key", "Bearer", ""},
		{"with a proof already used", "DPoP", proof},
		{"with a proof from another key", "DPoP", newDPoPKey(t).proof(t, "GET", userinfoURL, access)},
		{"with a proof for another address", "DPoP", key.proof(t, "GET", f.s.cfg.Issuer+"/elsewhere", access)},
		{"with a proof for another token", "DPoP", key.proof(t, "GET", userinfoURL, "another-token")},
	}
	for _, test := range tests {
		if status, _ := userinfo(test.scheme, test.proof); status != http.StatusUnauthorized {
			t.Errorf("userinfo %s = %d, want 401", test.name, status)
		}
	}

	refresh := tokens.str("refresh_token")
	asks := func(proof string) tokenResult {
		return postToken(t, f, spaID, proof, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
	}
	// An SPA's every token request needs a proof (dpop_bound_access_tokens).
	if got := asks(""); got.str("error") != "invalid_dpop_proof" {
		t.Errorf("refreshing without a proof = %v, want invalid_dpop_proof", got.body)
	}
	if got := asks(newDPoPKey(t).proof(t, "POST", tokenURL, "")); got.str("error") != "invalid_grant" {
		t.Errorf("refreshing with another key = %v, want invalid_grant", got.body)
	}
	if got := asks(key.proof(t, "POST", tokenURL, "")); got.status != http.StatusOK || got.str("token_type") != "DPoP" {
		t.Errorf("refreshing with the key = %d %v", got.status, got.body)
	}
}

// formPost posts a form to a provider endpoint as a client: HTTP Basic when
// a secret is given, the client id in the form otherwise, and a DPoP proof
// when one is given.
func formPost(t *testing.T, f *oauthFixture, path, clientID, secret, proof string, form url.Values) tokenResult {
	t.Helper()

	if secret == "" {
		form.Set("client_id", clientID)
	}
	req, _ := http.NewRequest(http.MethodPost, f.s.root+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if secret != "" {
		req.SetBasicAuth(url.QueryEscape(clientID), url.QueryEscape(secret))
	}
	if proof != "" {
		req.Header.Set("DPoP", proof)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	result := tokenResult{status: res.StatusCode, body: map[string]any{}}
	_ = json.NewDecoder(res.Body).Decode(&result.body)
	return result
}

// A code asked for with dpop_jkt (RFC 9449 §10) is redeemed only with a
// proof from that key, so a code intercepted on the way back is no use to
// whoever lacks the key. Tried without it, the code is spent, as it is when
// its PKCE verifier is wrong.
func TestLiveDPoPBindsTheAuthorizationCode(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	spaID, _, _ := f.register(map[string]any{
		"name": "Single page", "profile": "spa",
		"grant_types":   []string{"authorization_code", "refresh_token"},
		"redirect_uris": []string{shopRedirect},
		"scopes":        []string{"openid", "email", "offline_access"},
	})
	f.clientID, f.clientSecret = spaID, ""

	key, other := newDPoPKey(t), newDPoPKey(t)
	tokenURL := f.s.cfg.Issuer + "/oauth2/token"

	boundCode := func() (code, verifier string) {
		verifier, challenge := pkce(t)
		back := f.signInWith(f.s.browser(), challenge, url.Values{
			"scope": {"openid email offline_access"}, "resource": {""}, "dpop_jkt": {key.thumbprint(t)},
		})
		if back.Query().Get("code") == "" {
			t.Fatalf("authorize with dpop_jkt came back with %s", back)
		}
		return back.Query().Get("code"), verifier
	}
	exchange := func(code, verifier, proof string) tokenResult {
		return postToken(t, f, spaID, proof, url.Values{
			"grant_type": {"authorization_code"}, "code": {code},
			"redirect_uri": {shopRedirect}, "code_verifier": {verifier},
		})
	}

	t.Run("a proof from the named key gets tokens bound to it", func(t *testing.T) {
		code, verifier := boundCode()
		tokens := exchange(code, verifier, key.proof(t, "POST", tokenURL, ""))
		if tokens.status != http.StatusOK || tokens.str("token_type") != "DPoP" {
			t.Fatalf("= %d %v", tokens.status, tokens.body)
		}
		if _, claims := f.verifyJWT(tokens.str("access_token")); claims["cnf"].(map[string]any)["jkt"] != key.thumbprint(t) {
			t.Errorf("cnf = %v, want the named key's thumbprint", claims["cnf"])
		}
	})

	t.Run("a proof from another key is refused, and the code spent", func(t *testing.T) {
		code, verifier := boundCode()
		if refused := exchange(code, verifier, other.proof(t, "POST", tokenURL, "")); refused.status != http.StatusBadRequest || refused.str("error") != "invalid_grant" {
			t.Fatalf("= %d %v, want invalid_grant", refused.status, refused.body)
		}
		if again := exchange(code, verifier, key.proof(t, "POST", tokenURL, "")); again.str("error") != "invalid_grant" {
			t.Errorf("the code with the right key after a refusal = %d %v, want it spent", again.status, again.body)
		}
	})

	// An SPA's token requests all need a proof (dpop_bound_access_tokens), so
	// one without is refused before the code is even read.
	t.Run("no proof at all is refused", func(t *testing.T) {
		code, verifier := boundCode()
		if refused := exchange(code, verifier, ""); refused.status != http.StatusBadRequest || refused.str("error") != "invalid_dpop_proof" {
			t.Fatalf("= %d %v, want invalid_dpop_proof", refused.status, refused.body)
		}
	})

	t.Run("a malformed dpop_jkt goes back to the client", func(t *testing.T) {
		_, challenge := pkce(t)
		for _, bad := range []string{"short", strings.Repeat("A", 42) + "!", strings.Repeat("A", 44)} {
			back := f.s.browser().visit(f.authorizeURL(challenge, url.Values{"scope": {"openid"}, "resource": {""}, "dpop_jkt": {bad}}))
			if !strings.HasPrefix(back.String(), shopRedirect) || back.Query().Get("error") != "invalid_request" {
				t.Errorf("dpop_jkt %q led to %s, want invalid_request at the client", bad, back)
			}
		}
	})
}

// A pushed request that brings a DPoP proof binds its code to the proof's key
// (RFC 9449 §10.1), with no dpop_jkt needed; one whose dpop_jkt names another
// key than its proof is refused.
func TestLiveDPoPBindsAPushedRequest(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	key, other := newDPoPKey(t), newDPoPKey(t)
	parURL, tokenURL := f.s.cfg.Issuer+"/oauth2/par", f.s.cfg.Issuer+"/oauth2/token"

	push := func(proof string, extra url.Values) (tokenResult, string) {
		verifier, challenge := pkce(t)
		form := url.Values{
			"response_type": {"code"}, "redirect_uri": {shopRedirect}, "scope": {"openid email"},
			"code_challenge": {challenge}, "code_challenge_method": {"S256"},
		}
		for name, values := range extra {
			form[name] = values
		}
		return formPost(t, f, "/oauth2/par", f.clientID, f.clientSecret, proof, form), verifier
	}
	codeFor := func(requestURI string) string {
		b := f.s.browser()
		login := b.visit(f.s.root + "/oauth2/authorize?" + url.Values{"client_id": {f.clientID}, "request_uri": {requestURI}}.Encode())
		var out struct {
			RedirectTo string `json:"redirect_to"`
		}
		b.account(http.MethodPost, "/login", map[string]string{
			"request": login.Query().Get("request"), "email": adaEmail, "password": adaPassword,
		}, &out)
		back, _ := url.Parse(out.RedirectTo)
		if back.Query().Get("code") == "" {
			t.Fatalf("the pushed request came back with %s", out.RedirectTo)
		}
		return back.Query().Get("code")
	}
	exchange := func(code, verifier, proof string) tokenResult {
		return formPost(t, f, "/oauth2/token", f.clientID, f.clientSecret, proof, url.Values{
			"grant_type": {"authorization_code"}, "code": {code},
			"redirect_uri": {shopRedirect}, "code_verifier": {verifier},
		})
	}

	t.Run("the push's proof binds the code without dpop_jkt", func(t *testing.T) {
		pushed, verifier := push(key.proof(t, "POST", parURL, ""), nil)
		if pushed.status != http.StatusCreated {
			t.Fatalf("pushing with a proof = %d %v", pushed.status, pushed.body)
		}
		if refused := exchange(codeFor(pushed.str("request_uri")), verifier, ""); refused.str("error") != "invalid_grant" {
			t.Errorf("its code without a proof = %d %v, want invalid_grant", refused.status, refused.body)
		}

		pushed, verifier = push(key.proof(t, "POST", parURL, ""), nil)
		tokens := exchange(codeFor(pushed.str("request_uri")), verifier, key.proof(t, "POST", tokenURL, ""))
		if tokens.status != http.StatusOK || tokens.str("token_type") != "DPoP" {
			t.Fatalf("its code with a proof from the key = %d %v", tokens.status, tokens.body)
		}
	})

	t.Run("a proof and a dpop_jkt that agree are taken", func(t *testing.T) {
		if pushed, _ := push(key.proof(t, "POST", parURL, ""), url.Values{"dpop_jkt": {key.thumbprint(t)}}); pushed.status != http.StatusCreated {
			t.Errorf("= %d %v, want 201", pushed.status, pushed.body)
		}
	})

	t.Run("a proof and a dpop_jkt that disagree are refused", func(t *testing.T) {
		refused, _ := push(key.proof(t, "POST", parURL, ""), url.Values{"dpop_jkt": {other.thumbprint(t)}})
		if refused.status != http.StatusBadRequest || refused.str("error") != "invalid_dpop_proof" {
			t.Errorf("= %d %v, want invalid_dpop_proof", refused.status, refused.body)
		}
	})

	t.Run("a malformed dpop_jkt is the client's to read", func(t *testing.T) {
		refused, _ := push("", url.Values{"dpop_jkt": {"not-a-thumbprint"}})
		if refused.status != http.StatusBadRequest || refused.str("error") != "invalid_request" {
			t.Errorf("= %d %v, want invalid_request", refused.status, refused.body)
		}
	})

	t.Run("a proof for another address is refused", func(t *testing.T) {
		refused, _ := push(key.proof(t, "POST", tokenURL, ""), nil)
		if refused.str("error") != "invalid_dpop_proof" {
			t.Errorf("= %d %v, want invalid_dpop_proof", refused.status, refused.body)
		}
	})
}

// A DPoP proof carries a nonce the server gave (RFC 9449 §8, §9), at the
// token endpoint, a pushed request and userinfo: without one, or with one
// that is stale or not the server's, the client is told use_dpop_nonce and
// given one in DPoP-Nonce, and the same proof's jti is not spent, so it may
// try again with it.
func TestLiveADPoPProofCarriesTheServersNonce(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	spaID, _, _ := f.register(map[string]any{
		"name": "Single page", "profile": "spa",
		"grant_types":   []string{"authorization_code"},
		"redirect_uris": []string{shopRedirect},
		"scopes":        []string{"openid", "email"},
	})
	f.clientID, f.clientSecret = spaID, ""
	key := newDPoPKey(t)
	tokenURL := f.s.cfg.Issuer + "/oauth2/token"

	verifier, challenge := pkce(t)
	code := f.signInWith(f.s.browser(), challenge, url.Values{"scope": {"openid email"}, "resource": {""}}).Query().Get("code")
	exchange := url.Values{
		"grant_type": {"authorization_code"}, "code": {code},
		"redirect_uri": {shopRedirect}, "code_verifier": {verifier},
	}

	for name, nonce := range map[string]string{
		"no nonce":           "",
		"a nonce of its own": "bm90LXRoZS1zZXJ2ZXJzLW5vbmNlLWF0LWFsbA",
	} {
		t.Run(name, func(t *testing.T) {
			got := postToken(t, f, spaID, key.proofWith(t, "POST", tokenURL, "", nonce), url.Values{
				"grant_type": {"authorization_code"}, "code": {code},
				"redirect_uri": {shopRedirect}, "code_verifier": {verifier},
			})
			if got.status != http.StatusBadRequest || got.str("error") != "use_dpop_nonce" {
				t.Errorf("a proof with %s = %d %v, want use_dpop_nonce", name, got.status, got.body)
			}
		})
	}

	// The nonce the token endpoint gave: the code was not spent by asking.
	tokens := postToken(t, f, spaID, key.proof(t, "POST", tokenURL, ""), exchange)
	if tokens.status != http.StatusOK {
		t.Fatalf("the code with the server's nonce = %d %v", tokens.status, tokens.body)
	}

	// Userinfo asks for it with a DPoP challenge.
	req, _ := http.NewRequest(http.MethodGet, f.s.root+"/oauth2/userinfo", nil)
	access := tokens.str("access_token")
	req.Header.Set("Authorization", "DPoP "+access)
	req.Header.Set("DPoP", key.proofWith(t, "GET", f.s.cfg.Issuer+"/oauth2/userinfo", access, ""))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized || !strings.Contains(res.Header.Get("WWW-Authenticate"), `error="use_dpop_nonce"`) || res.Header.Get("DPoP-Nonce") == "" {
		t.Errorf("userinfo with no nonce = %d, %q, nonce %q; want 401 use_dpop_nonce with a nonce", res.StatusCode, res.Header.Get("WWW-Authenticate"), res.Header.Get("DPoP-Nonce"))
	}
}

// dpop_bound_access_tokens (RFC 9449 §5.2): a client that sets it is refused
// any token request without a DPoP proof, so all its tokens are bound to its
// key. An SPA and a mobile app always have it, whatever is sent.
func TestLiveAClientMayRequireDPoPBoundTokens(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	clientID, secret, appID := f.register(map[string]any{
		"name": "Backend", "profile": "web",
		"grant_types":              []string{"authorization_code", "client_credentials"},
		"redirect_uris":            []string{shopRedirect},
		"dpop_bound_access_tokens": true,
	})
	f.authorizeAPI(appID, "orders:read")
	grant := func() url.Values {
		return url.Values{"grant_type": {"client_credentials"}, "resource": {ordersAPI}}
	}

	if got := f.token("/oauth2/token", clientID, secret, grant()); got.status != http.StatusBadRequest || got.str("error") != "invalid_dpop_proof" {
		t.Errorf("a token request with no proof = %d %v, want invalid_dpop_proof", got.status, got.body)
	}

	key := newDPoPKey(t)
	req, _ := http.NewRequest(http.MethodPost, f.s.root+"/oauth2/token", strings.NewReader(grant().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(clientID), url.QueryEscape(secret))
	req.Header.Set("DPoP", key.proof(t, "POST", f.s.cfg.Issuer+"/oauth2/token", ""))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var tokens map[string]any
	_ = json.NewDecoder(res.Body).Decode(&tokens)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || tokens["token_type"] != "DPoP" {
		t.Errorf("a token request with a proof = %d %v, want a DPoP-bound token", res.StatusCode, tokens)
	}

	var spa struct {
		Application struct {
			Bound bool `json:"dpop_bound_access_tokens"`
		} `json:"application"`
	}
	f.super.must(http.StatusCreated, http.MethodPost, "/applications", map[string]any{
		"name": "Storefront", "profile": "spa", "grant_types": []string{"authorization_code"},
		"redirect_uris": []string{"https://store.example.com/callback"}, "dpop_bound_access_tokens": false,
	}, &spa)
	if !spa.Application.Bound {
		t.Error("an SPA was made without dpop_bound_access_tokens")
	}
}

// The admin and account APIs take a DPoP-bound access token sent as DPoP,
// with a proof from its key for this very request (RFC 9449 §7), and never as
// a plain bearer token, which would undo the binding.
func TestLiveTheAdminAPITakesADPoPBoundTokenWithItsProof(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	cliID := f.adminCLI()
	f.giveKey(cliID, model.AdminCLIClientID)
	apiID, scopes := f.systemAPI(model.SystemAdminAPI)
	f.allow(cliID, apiID, scopes, model.PermApplicationsRead)

	key := newDPoPKey(t)
	form := url.Values{
		"grant_type": {"client_credentials"}, "resource": {brand.AdminAPIIdentifier}, "scope": {string(model.PermApplicationsRead)},
		"client_id":             {model.AdminCLIClientID},
		"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
		"client_assertion":      {assertion(t, f.keys[model.AdminCLIClientID], model.AdminCLIClientID, f.s.cfg.Issuer, nil)},
	}
	req, _ := http.NewRequest(http.MethodPost, f.s.root+"/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("DPoP", key.proof(t, "POST", f.s.cfg.Issuer+"/oauth2/token", ""))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var tokens map[string]any
	_ = json.NewDecoder(res.Body).Decode(&tokens)
	res.Body.Close()
	access, _ := tokens["access_token"].(string)
	if tokens["token_type"] != "DPoP" || access == "" {
		t.Fatalf("a bound admin API token = %d %v", res.StatusCode, tokens)
	}

	// The proof names the address the panel serves the admin API at.
	path := "/api/v1/admin/applications"
	target := f.s.cfg.AdminURL + path
	call := func(scheme, proof string) int {
		req, _ := http.NewRequest(http.MethodGet, f.s.adminRoot+path, nil)
		req.Header.Set("Authorization", scheme+" "+access)
		if proof != "" {
			req.Header.Set("DPoP", proof)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}

	if got := call("DPoP", key.proofWith(t, "GET", target, access, "")); got != http.StatusOK {
		t.Errorf("the bound token with its proof = %d, want 200", got)
	}
	for name, tt := range map[string]struct{ scheme, proof string }{
		"as a bearer token":                 {"Bearer", ""},
		"as DPoP with no proof":             {"DPoP", ""},
		"with a proof for another address":  {"DPoP", key.proofWith(t, "GET", f.s.cfg.AdminURL+"/api/v1/admin/users", access, "")},
		"with a proof from another key":     {"DPoP", newDPoPKey(t).proofWith(t, "GET", target, access, "")},
		"with a proof naming another token": {"DPoP", key.proofWith(t, "GET", target, "another-token", "")},
	} {
		if got := call(tt.scheme, tt.proof); got != http.StatusUnauthorized {
			t.Errorf("the bound token %s = %d, want 401", name, got)
		}
	}
}
