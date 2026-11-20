package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// receiver is an application's back-channel logout endpoint: it keeps every
// logout token posted to it, and answers with what `answers` says, in turn,
// then 200.
type receiver struct {
	url     string
	tokens  chan string
	answers []int
}

func newReceiver(t *testing.T, answers ...int) *receiver {
	t.Helper()

	r := &receiver{tokens: make(chan string, 16), answers: answers}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("the logout came as %s %s", req.Method, req.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(req.Body)
		form, _ := url.ParseQuery(string(body))
		r.tokens <- form.Get("logout_token")
		status := http.StatusOK
		if len(r.answers) > 0 {
			status, r.answers = r.answers[0], r.answers[1:]
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	r.url = server.URL + "/backchannel-logout"

	return r
}

func (r *receiver) wait(t *testing.T) string {
	t.Helper()

	select {
	case token := <-r.tokens:
		return token
	// A retry comes model.LogoutRetryAfter(1) later, found at the sender's
	// next look.
	case <-time.After(15 * time.Second):
		t.Fatal("no logout token was posted")
		return ""
	}
}

func (r *receiver) none(t *testing.T) {
	t.Helper()

	select {
	case token := <-r.tokens:
		t.Fatalf("a logout token was posted when none should be: %s", token)
	case <-time.After(300 * time.Millisecond):
	}
}

// An application with a back-channel logout URI is told, server to server,
// whenever a session it signed the user in through ends — however it ends —
// with a logout token naming the user and the session from its ID token.
func TestLiveBackchannelLogout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		end  func(f *oauthFixture, b *browser, idToken string)
	}{
		{
			name: "the application signs the user out",
			end: func(f *oauthFixture, b *browser, idToken string) {
				b.visit(f.s.root + "/oauth2/logout?" + url.Values{"id_token_hint": {idToken}}.Encode())
			},
		},
		{
			name: "the user confirms signing out, with no hint",
			end: func(f *oauthFixture, b *browser, _ string) {
				b.account(http.MethodPost, "/logout", map[string]string{}, nil)
			},
		},
		{
			name: "an administrator ends that one session",
			end: func(f *oauthFixture, b *browser, _ string) {
				f.super.must(http.StatusNoContent, http.MethodDelete, "/user-sessions/"+currentSession(f.s.t, b), nil, nil)
			},
		},
		{
			name: "the user changes their password in another browser",
			end: func(f *oauthFixture, _ *browser, _ string) {
				other := f.s.browser()
				_, challenge := pkce(f.s.t)
				f.signIn(other, challenge)
				if status := other.account(http.MethodPost, "/password", map[string]string{"current_password": adaPassword, "new_password": "ada-new-password-1"}, nil); status != http.StatusNoContent && status != http.StatusOK {
					f.s.t.Fatalf("changing the password = %d", status)
				}
			},
		},
		{
			name: "the user resets their password",
			end: func(f *oauthFixture, _ *browser, _ string) {
				f.s.browser().account(http.MethodPost, "/forgot-password", map[string]string{"email": adaEmail}, nil)
				token := tokenIn(f.s.t, f.s.mail.waitFor(f.s.t, adaEmail, "/reset-password").Body, "/reset-password")
				if status := f.s.browser().account(http.MethodPost, "/reset-password", map[string]string{"token": token, "password": "ada-new-password-1"}, nil); status != http.StatusOK {
					f.s.t.Fatalf("resetting = %d", status)
				}
			},
		},
		{
			name: "an administrator turns the user off",
			end: func(f *oauthFixture, _ *browser, _ string) {
				f.super.must(http.StatusOK, http.MethodPatch, "/users/"+f.userID, map[string]any{
					"email": adaEmail, "first_name": "Ada", "last_name": "Lovelace", "is_active": false,
				}, nil)
			},
		},
		{
			name: "an administrator signs the user out everywhere",
			end: func(f *oauthFixture, _ *browser, _ string) {
				f.super.must(http.StatusOK, http.MethodDelete, "/users/"+f.userID+"/sessions", nil, nil)
			},
		},
		{
			name: "an administrator deletes the user",
			end: func(f *oauthFixture, _ *browser, _ string) {
				f.super.must(http.StatusNoContent, http.MethodDelete, "/users/"+f.userID, nil, nil)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newOAuthFixture(t)
			told := newReceiver(t)

			// The fixture's own application has no URI and must not be told.
			f.clientID, f.clientSecret, f.appID = f.register(map[string]any{
				"name": "Back office", "profile": "web",
				"grant_types":            []string{"authorization_code"},
				"redirect_uris":          []string{shopRedirect},
				"scopes":                 []string{"openid", "email"},
				"backchannel_logout_uri": told.url,
			})

			b := f.s.browser()
			verifier, challenge := pkce(t)
			back := f.signInWith(b, challenge, url.Values{"scope": {"openid email"}, "resource": {""}})
			tokens := f.exchange(back.Query().Get("code"), verifier)
			if tokens.str("id_token") == "" {
				t.Fatalf("no ID token: %d %v", tokens.status, tokens.body)
			}
			_, id := f.verifyJWT(tokens.str("id_token"))
			if id["sid"] == nil {
				t.Fatalf("the ID token has no sid: %v", id)
			}

			test.end(f, b, tokens.str("id_token"))

			header, claims := f.verifyJWT(told.wait(t))
			if header["typ"] != "logout+jwt" {
				t.Errorf("typ = %v, want logout+jwt", header["typ"])
			}
			want := map[string]any{"iss": f.s.cfg.Issuer, "aud": f.clientID, "sub": f.userID, "sid": id["sid"]}
			for claim, value := range want {
				if claims[claim] != value {
					t.Errorf("%s = %v, want %v", claim, claims[claim], value)
				}
			}
			events, _ := claims["events"].(map[string]any)
			if _, ok := events["http://schemas.openid.net/event/backchannel-logout"]; !ok {
				t.Errorf("events = %v, want the back-channel logout event", claims["events"])
			}
			if _, ok := claims["nonce"]; ok {
				t.Error("a logout token must not carry a nonce")
			}
			if claims["jti"] == nil || claims["iat"] == nil || claims["exp"] == nil {
				t.Errorf("jti, iat and exp are required: %v", claims)
			}

			told.none(t)
		})
	}
}

// A session that ran out on its own is not a logout, and notifies nobody
// (ADR 0008): ending it afterwards, with every other, tells no application.
func TestLiveAnExpiredSessionTellsNobody(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	told := newReceiver(t)
	f.clientID, f.clientSecret, f.appID = f.register(map[string]any{
		"name": "Back office", "profile": "web",
		"grant_types":            []string{"authorization_code"},
		"redirect_uris":          []string{shopRedirect},
		"scopes":                 []string{"openid", "email"},
		"backchannel_logout_uri": told.url,
	})

	b := f.s.browser()
	verifier, challenge := pkce(t)
	f.exchange(f.signInWith(b, challenge, url.Values{"scope": {"openid email"}, "resource": {""}}).Query().Get("code"), verifier)
	if err := f.s.db.Exec("UPDATE user_sessions SET expires_at = now() - interval '1 second' WHERE id = ?", currentSession(t, b)).Error; err != nil {
		t.Fatal(err)
	}

	f.super.must(http.StatusOK, http.MethodDelete, "/users/"+f.userID+"/sessions", nil, nil)
	told.none(t)
}

// Only applications that were given an ID token in the session are told: one
// with a URI that the user never signed in to hears nothing.
func TestLiveBackchannelLogoutTellsOnlyTheSessionsApplications(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	told := newReceiver(t)

	f.register(map[string]any{
		"name": "Never used", "profile": "web",
		"grant_types":            []string{"authorization_code"},
		"redirect_uris":          []string{shopRedirect},
		"scopes":                 []string{"openid"},
		"backchannel_logout_uri": told.url,
	})

	b := f.s.browser()
	verifier, challenge := pkce(t)
	tokens := f.exchange(f.signIn(b, challenge).Query().Get("code"), verifier)
	b.visit(f.s.root + "/oauth2/logout?" + url.Values{"id_token_hint": {tokens.str("id_token")}}.Encode())

	told.none(t)
}

// A refresh keeps the session: the new ID token names the sid the first one
// did, so the application can match the logout token to it.
func TestLiveRefreshedIDTokenKeepsTheSid(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	b := f.s.browser()

	verifier, challenge := pkce(t)
	first := f.exchange(f.signIn(b, challenge).Query().Get("code"), verifier)
	_, before := f.verifyJWT(first.str("id_token"))

	refreshed := f.token("/oauth2/token", f.clientID, f.clientSecret, url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {first.str("refresh_token")},
	})
	if refreshed.str("id_token") == "" {
		t.Fatalf("the refresh gave no ID token: %d %v", refreshed.status, refreshed.body)
	}
	_, after := f.verifyJWT(refreshed.str("id_token"))

	if before["sid"] == nil || after["sid"] != before["sid"] {
		t.Errorf("sid went from %v to %v, want it kept", before["sid"], after["sid"])
	}
}

// A logout the application could not take is tried again; one it refuses is
// not, and ends up in the activity log, where administrators see which
// application missed it.
func TestLiveBackchannelLogoutIsTriedAgainAndLogged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		answers []int
		posts   int
		logged  bool
	}{
		{name: "an application that fails once gets it on the next try", answers: []int{http.StatusServiceUnavailable}, posts: 2},
		{name: "an application that refuses it is not asked again", answers: []int{http.StatusBadRequest}, posts: 1, logged: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newOAuthFixture(t)
			told := newReceiver(t, test.answers...)
			f.clientID, f.clientSecret, f.appID = f.register(map[string]any{
				"name": "Back office", "profile": "web",
				"grant_types":            []string{"authorization_code"},
				"redirect_uris":          []string{shopRedirect},
				"scopes":                 []string{"openid", "email"},
				"backchannel_logout_uri": told.url,
			})

			b := f.s.browser()
			verifier, challenge := pkce(t)
			back := f.signInWith(b, challenge, url.Values{"scope": {"openid email"}, "resource": {""}})
			tokens := f.exchange(back.Query().Get("code"), verifier)
			b.visit(f.s.root + "/oauth2/logout?" + url.Values{"id_token_hint": {tokens.str("id_token")}}.Encode())

			for range test.posts {
				told.wait(t)
			}
			told.none(t)

			logged := func() bool {
				var page struct {
					Logs []struct {
						Action string `json:"action"`
					} `json:"logs"`
				}
				f.super.must(http.StatusOK, http.MethodGet, "/logs?limit=50", nil, &page)
				for _, entry := range page.Logs {
					if entry.Action == "application.logout_undelivered" {
						return true
					}
				}
				return false
			}
			if got := logged(); got != test.logged {
				t.Errorf("the activity log has the undelivered logout: %v, want %v", got, test.logged)
			}
		})
	}
}

// A notice is kept from the moment the session ends until the application
// takes it: one that failed stays owed, saying why and when it is tried
// next, and the panel shows each as delivered, owed or given up.
func TestLiveALogoutNoticeIsKeptUntilDelivered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		answers []int
		want    string
	}{
		{name: "taken at once", want: "delivered"},
		{name: "failed once, then taken", answers: []int{http.StatusServiceUnavailable}, want: "delivered"},
		{name: "refused", answers: []int{http.StatusBadRequest}, want: "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newOAuthFixture(t)
			told := newReceiver(t, tt.answers...)
			f.clientID, f.clientSecret, f.appID = f.register(map[string]any{
				"name": "Back office", "profile": "web",
				"grant_types":            []string{"authorization_code"},
				"redirect_uris":          []string{shopRedirect},
				"scopes":                 []string{"openid", "email"},
				"backchannel_logout_uri": told.url,
			})
			b := f.s.browser()
			verifier, challenge := pkce(t)
			f.exchange(f.signInWith(b, challenge, url.Values{"scope": {"openid email"}, "resource": {""}}).Query().Get("code"), verifier)
			session := currentSession(t, b)

			type delivery struct {
				SessionID string  `json:"session_id"`
				Status    string  `json:"status"`
				Tries     int     `json:"tries"`
				LastError string  `json:"last_error"`
				NextTryAt *string `json:"next_try_at"`
			}
			deliveries := func() []delivery {
				var page struct {
					Deliveries []delivery `json:"deliveries"`
				}
				f.super.must(http.StatusOK, http.MethodGet, "/applications/"+f.appID+"/logout-deliveries", nil, &page)
				return page.Deliveries
			}

			f.super.must(http.StatusOK, http.MethodDelete, "/users/"+f.userID+"/sessions", nil, nil)
			told.wait(t)

			if len(tt.answers) > 0 && tt.answers[0] >= 500 {
				// Between the tries: owed, with the reason and the next try.
				waitUntil(t, func() bool { d := deliveries(); return len(d) == 1 && d[0].LastError != "" })
				d := deliveries()[0]
				if d.Status != "pending" || d.Tries != 1 || d.NextTryAt == nil || !strings.Contains(d.LastError, "503") {
					t.Errorf("after a failed try = %+v, want pending, one try, the 503 and a next try", d)
				}
				told.wait(t)
			}

			waitUntil(t, func() bool { d := deliveries(); return len(d) == 1 && d[0].Status != "pending" })
			if d := deliveries()[0]; d.Status != tt.want || d.SessionID != session {
				t.Errorf("in the end = %+v, want %s for session %s", d, tt.want, session)
			}
		})
	}
}

// waitUntil waits for a condition the server's workers make true.
func waitUntil(t *testing.T, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if done() {
			return
		}
	}
	t.Fatal("it never happened")
}
