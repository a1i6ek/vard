package integration

import (
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/a1i6ek/vard/internal/domain/model"
)

const reportsAPI = "https://api.example.com/reports"

// exchangeFixture is the OAuth fixture with a second API, Reports, and a
// machine client, Billing, that may act for Shop's users only as the rules
// an administrator sets for Reports allow.
type exchangeFixture struct {
	*oauthFixture
	reportsID  string
	billingID  string
	billingApp string
}

func newExchangeFixture(t *testing.T) *exchangeFixture {
	f := &exchangeFixture{oauthFixture: newOAuthFixture(t)}

	var api struct {
		API idOnly `json:"api"`
	}
	f.super.must(http.StatusCreated, http.MethodPost, "/apis", map[string]any{
		"name": "Reports", "identifier": reportsAPI, "enforce_roles": false,
		"scopes": []map[string]any{{"name": "orders:read"}, {"name": "orders:write"}, {"name": "reports:read"}},
	}, &api)
	f.reportsID = api.API.ID
	f.billingID, _, f.billingApp = f.register(map[string]any{"name": "Billing", "profile": "machine"})
	return f
}

// rules replaces Reports' exchange rules: each is {acting app, subject app,
// scopes...}.
func (f *exchangeFixture) rules(rules ...[]string) {
	body := []map[string]any{}
	for _, rule := range rules {
		body = append(body, map[string]any{"application_id": rule[0], "subject_application_id": rule[1], "scopes": rule[2:]})
	}
	f.super.must(http.StatusOK, http.MethodPut, "/apis/"+f.reportsID+"/exchange-rules", map[string]any{"rules": body}, nil)
}

// enable turns an application on or off, sending back the rest as it is.
func (f *exchangeFixture) enable(appID string, on bool) {
	var got struct {
		Application map[string]any `json:"application"`
	}
	f.super.must(http.StatusOK, http.MethodGet, "/applications/"+appID, nil, &got)
	got.Application["is_enabled"] = on
	f.super.must(http.StatusOK, http.MethodPatch, "/applications/"+appID, got.Application, nil)
}

// exchange is Billing trading a token for one to Reports.
func (f *exchangeFixture) exchange(subject, kind string, extra url.Values) tokenResult {
	form := url.Values{
		"grant_type": {model.GrantTokenExchange}, "resource": {reportsAPI},
		"subject_token": {subject}, "subject_token_type": {kind},
	}
	for key, values := range extra {
		form[key] = values
	}
	return f.token("/oauth2/token", f.billingID, signsWithKey, form)
}

// adasTokens signs Ada in to Shop and returns its tokens: an access token for
// Orders holding orders:read, which her role grants, and an ID token.
func (f *exchangeFixture) adasTokens() tokenResult {
	b := f.s.browser()
	verifier, challenge := pkce(f.s.t)
	tokens := f.oauthFixture.exchange(f.signIn(b, challenge).Query().Get("code"), verifier)
	if tokens.status != http.StatusOK || tokens.str("access_token") == "" {
		f.s.t.Fatalf("signing Ada in = %d %v", tokens.status, tokens.body)
	}
	return tokens
}

// Token exchange (RFC 8693) is allowed only by an administrator's rule for
// the API, naming who acts and whose tokens. What it issues acts for the
// subject token's user, says who acts (act), holds no scope the subject
// token lacked, lives no longer, is bound to the acting client's key, and
// is written to the activity log.
func TestLiveATokenIsExchangedOnlyAsARuleAllows(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	ada := f.adasTokens()
	access := ada.str("access_token")

	if got := f.exchange(access, model.TokenTypeAccess, nil); got.status != http.StatusBadRequest || got.str("error") != "unauthorized_client" {
		t.Errorf("an exchange with no rule = %d %v, want unauthorized_client", got.status, got.body)
	}

	f.rules([]string{f.billingApp, f.appID, "orders:read", "orders:write"})

	refusals := []struct {
		name  string
		token string
		extra url.Values
		want  string
	}{
		{name: "a scope the subject token lacks", token: access, extra: url.Values{"scope": {"orders:write"}}, want: "invalid_scope"},
		{name: "a scope the rule leaves out", token: access, extra: url.Values{"scope": {"reports:read"}}, want: "invalid_scope"},
		{name: "a token this server did not issue", token: "not-a-token", want: "invalid_request"},
		{name: "a refresh token as the subject", token: ada.str("refresh_token"), want: "invalid_request"},
		{name: "an API with no such identifier", token: access, extra: url.Values{"resource": {"https://api.example.com/nothing"}}, want: "invalid_target"},
		{name: "a refresh token asked for", token: access, extra: url.Values{"requested_token_type": {"urn:ietf:params:oauth:token-type:refresh_token"}}, want: "invalid_request"},
		{name: "an actor token that is not Billing's own", token: access, extra: url.Values{"actor_token": {access}, "actor_token_type": {model.TokenTypeAccess}}, want: "invalid_request"},
	}
	for _, tt := range refusals {
		if got := f.exchange(tt.token, model.TokenTypeAccess, tt.extra); got.status != http.StatusBadRequest || got.str("error") != tt.want {
			t.Errorf("%s = %d %v, want %s", tt.name, got.status, got.body, tt.want)
		}
	}

	got := f.exchange(access, model.TokenTypeAccess, nil)
	if got.status != http.StatusOK || got.str("issued_token_type") != model.TokenTypeAccess || got.str("token_type") != "DPoP" {
		t.Fatalf("an exchange a rule allows = %d %v", got.status, got.body)
	}
	if got.str("scope") != "orders:read" {
		t.Errorf("scope = %q, want orders:read: what Ada's token held, of what the rule lists", got.str("scope"))
	}
	if _, ok := got.body["refresh_token"]; ok {
		t.Error("an exchange gave a refresh token")
	}

	_, subject := f.verifyJWT(access)
	_, claims := f.verifyJWT(got.str("access_token"))
	act, _ := claims["act"].(map[string]any)
	switch {
	case claims["sub"] != f.userID:
		t.Errorf("sub = %v, want Ada", claims["sub"])
	case !slices.Equal(audiences(claims), []string{reportsAPI}):
		t.Errorf("aud = %v, want Reports alone", claims["aud"])
	case claims["client_id"] != f.billingID || act["sub"] != f.billingID:
		t.Errorf("client_id %v, act %v, want Billing acting", claims["client_id"], claims["act"])
	case claims["exp"].(float64) > subject["exp"].(float64):
		t.Errorf("exp %v outlives the subject token's %v", claims["exp"], subject["exp"])
	case claims["cnf"] == nil:
		t.Error("the exchanged token is not bound to Billing's key")
	}

	// An ID token speaks for its user and holds no scope, so neither does
	// what it is exchanged for.
	if got := f.exchange(ada.str("id_token"), model.TokenTypeID, nil); got.status != http.StatusOK || got.str("scope") != "" {
		t.Errorf("an exchanged ID token = %d %v, want a token with no scope", got.status, got.body)
	}

	var logs struct {
		Logs []struct {
			Action string `json:"action"`
			Actor  string `json:"actor"`
		} `json:"logs"`
	}
	f.super.must(http.StatusOK, http.MethodGet, "/logs?action=token.exchanged", nil, &logs)
	if len(logs.Logs) != 2 || logs.Logs[0].Actor != f.billingID {
		t.Errorf("the activity log holds %v, want both exchanges, by Billing", logs.Logs)
	}

	f.enable(f.appID, false)
	if got := f.exchange(access, model.TokenTypeAccess, nil); got.str("error") != "invalid_request" {
		t.Errorf("a token of a turned-off application = %d %v, want invalid_request", got.status, got.body)
	}
	f.enable(f.appID, true)

	f.rules()
	if got := f.exchange(access, model.TokenTypeAccess, nil); got.str("error") != "unauthorized_client" {
		t.Errorf("an exchange once its rule is removed = %d %v, want unauthorized_client", got.status, got.body)
	}
}

// An exchanged token may itself be exchanged, each hop nesting the one
// before in act, to model.MaxDelegationDepth actors and no further; and
// revoking the token everything began from ends every one of them.
func TestLiveADelegationChainIsBoundedAndEndsWithItsSubject(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	access := f.adasTokens().str("access_token")
	// Billing may act for Shop's users, and then for its own exchanged tokens.
	f.rules([]string{f.billingApp, f.appID, "orders:read"}, []string{f.billingApp, f.billingApp, "orders:read"})

	chain := []string{}
	token := access
	for depth := 1; depth <= model.MaxDelegationDepth; depth++ {
		got := f.exchange(token, model.TokenTypeAccess, nil)
		if got.status != http.StatusOK {
			t.Fatalf("hop %d = %d %v", depth, got.status, got.body)
		}
		token = got.str("access_token")
		chain = append(chain, token)

		_, claims := f.verifyJWT(token)
		if got := delegationDepth(claims["act"]); got != depth {
			t.Errorf("hop %d nests %d actors in act", depth, got)
		}
	}
	if got := f.exchange(token, model.TokenTypeAccess, nil); got.status != http.StatusBadRequest || got.str("error") != "invalid_request" {
		t.Errorf("hop %d = %d %v, want refused", model.MaxDelegationDepth+1, got.status, got.body)
	}

	if got := f.token("/oauth2/revoke", f.clientID, f.clientSecret, url.Values{"token": {access}}); got.status != http.StatusOK {
		t.Fatalf("revoking Ada's token = %d %v", got.status, got.body)
	}
	for i, exchanged := range chain {
		if got := f.token("/oauth2/introspect", f.billingID, signsWithKey, url.Values{"token": {exchanged}}); got.body["active"] != false {
			t.Errorf("hop %d after its subject was revoked: introspection = %v, want inactive", i+1, got.body)
		}
	}
}

// Ending the browser session a token was issued in ends what was exchanged
// from it, as it ends the token itself.
func TestLiveAnExchangedTokenEndsWithItsSession(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	f.rules([]string{f.billingApp, f.appID, "orders:read"})
	access := f.adasTokens().str("access_token")
	exchanged := f.exchange(access, model.TokenTypeAccess, nil).str("access_token")

	f.super.must(http.StatusOK, http.MethodDelete, "/users/"+f.userID+"/sessions", nil, nil)

	if got := f.token("/oauth2/introspect", f.billingID, signsWithKey, url.Values{"token": {exchanged}}); got.body["active"] != false {
		t.Errorf("after Ada was signed out everywhere: introspection = %v, want inactive", got.body)
	}
}

func audiences(claims map[string]any) []string {
	out := []string{}
	list, _ := claims["aud"].([]any)
	for _, aud := range list {
		out = append(out, aud.(string))
	}
	return out
}

func delegationDepth(act any) int {
	depth := 0
	for claim, ok := act.(map[string]any); ok; claim, ok = claim["act"].(map[string]any) {
		depth++
	}
	return depth
}

// An exchange rule is checked as it is saved: only a confidential client may
// act for somebody else, every scope is the API's own, and a pair of
// applications has one rule.
func TestLiveAnExchangeRuleIsCheckedAsItIsSaved(t *testing.T) {
	t.Parallel()

	f := newExchangeFixture(t)
	_, _, spa := f.register(map[string]any{"name": "Storefront", "profile": "spa", "grant_types": []string{"authorization_code"}, "redirect_uris": []string{shopRedirect}})

	rule := func(actor, subject string, scopes ...string) map[string]any {
		return map[string]any{"application_id": actor, "subject_application_id": subject, "scopes": scopes}
	}
	tests := []struct {
		name  string
		rules []map[string]any
		want  string
	}{
		{name: "a public client acting", rules: []map[string]any{rule(spa, f.appID, "orders:read")}, want: "api_exchange_public"},
		{name: "a scope of another API", rules: []map[string]any{rule(f.billingApp, f.appID, "billing:read")}, want: "api_scope_foreign"},
		{name: "no scope at all", rules: []map[string]any{rule(f.billingApp, f.appID)}, want: "api_exchange_rule_no_scope"},
		{
			name:  "one pair twice",
			rules: []map[string]any{rule(f.billingApp, f.appID, "orders:read"), rule(f.billingApp, f.appID, "orders:write")},
			want:  "api_exchange_rule_repeated",
		},
	}
	for _, tt := range tests {
		var problem struct {
			Code string `json:"code"`
		}
		status := f.super.do(http.MethodPut, "/apis/"+f.reportsID+"/exchange-rules", map[string]any{"rules": tt.rules}, &problem)
		if status != http.StatusBadRequest || problem.Code != tt.want {
			t.Errorf("%s = %d %q, want 400 %s", tt.name, status, problem.Code, tt.want)
		}
	}

	var saved struct {
		Rules []map[string]any `json:"rules"`
	}
	f.super.must(http.StatusOK, http.MethodGet, "/apis/"+f.reportsID+"/exchange-rules", nil, &saved)
	if len(saved.Rules) != 0 {
		t.Errorf("a refused save left rules: %v", saved.Rules)
	}
}
