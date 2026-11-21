package integration

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// problemBody is an error as the API answers it: problem details (RFC
// 9457), with the code and its parameters as extension members.
type problemBody struct {
	Type   string         `json:"type"`
	Title  string         `json:"title"`
	Status int            `json:"status"`
	Detail string         `json:"detail"`
	Code   string         `json:"code"`
	Params map[string]any `json:"params"`
}

// Every error an app shows comes with a code it can say in the reader's
// language, the parameters that sentence needs, and the English beside them —
// from the public server and the admin one, from a handler, the validator and
// the middleware alike.
func TestLiveErrorsCarryCodes(t *testing.T) {
	t.Parallel()

	s := newLiveServerLimited(t, 3)
	super := s.superAdmin()
	b := s.browser()

	tests := []struct {
		name       string
		send       func(out *problemBody) int
		wantStatus int
		wantCode   string
		wantParams map[string]any
	}{
		{
			name: "a wrong password",
			send: func(out *problemBody) int {
				return b.account(http.MethodPost, "/login", map[string]string{"email": "nobody@example.com", "password": "wrong-password"}, out)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "invalid_credentials",
		},
		{
			name: "a field left out",
			send: func(out *problemBody) int {
				return b.account(http.MethodPost, "/forgot-password", map[string]string{}, out)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation.required",
			wantParams: map[string]any{"field": "email"},
		},
		{
			name: "a language that is not offered",
			send: func(out *problemBody) int {
				return s.publicInto("/languages/xx", out)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "language_not_found",
		},
		{
			name: "the admin API, a language added twice",
			send: func(out *problemBody) int {
				return super.do(http.MethodPost, "/languages", map[string]any{"code": "en", "name": "English", "native_name": "English"}, out)
			},
			wantStatus: http.StatusConflict,
			wantCode:   "language_code_taken",
		},
		{
			name: "the admin API, a code that is not a language tag",
			send: func(out *problemBody) int {
				return super.do(http.MethodPost, "/languages", map[string]any{"code": "not a tag", "name": "X", "native_name": "X"}, out)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation.languagetag",
			wantParams: map[string]any{"field": "code"},
		},
		{
			name: "the admin API, signed out",
			send: func(out *problemBody) int {
				return s.client().do(http.MethodGet, "/languages", nil, out)
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "not_signed_in",
		},
		{
			name: "a path nothing answers",
			send: func(out *problemBody) int {
				return s.publicInto("/nothing-here", out)
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got problemBody
			if status := tt.send(&got); status != tt.wantStatus {
				t.Fatalf("status = %d %+v, want %d", status, got, tt.wantStatus)
			}

			if got.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", got.Code, tt.wantCode)
			}
			if got.Detail == "" || got.Detail == got.Code {
				t.Errorf("detail = %q, want the English sentence for whoever calls without an app", got.Detail)
			}
			if got.Status != tt.wantStatus || got.Title != http.StatusText(tt.wantStatus) {
				t.Errorf("status %d, title %q; want the answer's own", got.Status, got.Title)
			}
			if !strings.HasSuffix(got.Type, "/problems/"+got.Code) || !strings.HasPrefix(got.Type, s.cfg.Issuer) {
				t.Errorf("type = %q, want %s/problems/%s", got.Type, s.cfg.Issuer, got.Code)
			}
			for name, want := range tt.wantParams {
				if got.Params[name] != want {
					t.Errorf("params[%s] = %v, want %v", name, got.Params[name], want)
				}
			}
		})
	}

	// The rate limit says when to try again, as a number the app puts in
	// its own sentence.
	var limited problemBody
	for range 5 {
		b.account(http.MethodPost, "/login", map[string]string{"email": "nobody@example.com", "password": "x"}, &limited)
	}
	if limited.Code != "rate_limited" || limited.Params["seconds"] == nil {
		t.Errorf("over the limit = %+v, want rate_limited with the seconds to wait", limited)
	}

	// Sent as problem details, from either server.
	for _, address := range []string{s.root + "/nothing-here", s.adminRoot + "/api/v1/admin/languages"} {
		res, err := http.Get(address)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/problem+json") {
			t.Errorf("%s: Content-Type = %q, want application/problem+json", address, got)
		}
	}
}

// The reset email is written in the language the page was shown in, and in
// the default language when that is one nobody offers.
func TestLiveResetEmailIsInTheReadersLanguage(t *testing.T) {
	t.Parallel()

	s := newLiveServer(t)
	super := s.superAdmin()

	// A language added in the panel, with its own reset email.
	super.must(http.StatusCreated, http.MethodPost, "/languages", map[string]any{
		"code": "de", "name": "German", "native_name": "Deutsch", "is_enabled": true,
	}, nil)
	super.must(http.StatusOK, http.MethodPut, "/languages/de/translations/id", map[string]any{"messages": map[string]string{
		"email.reset.subject": "Passwort zurücksetzen",
		"email.reset.body":    "Wählen Sie hier ein neues Passwort für {app}:\n{link}\n",
	}}, nil)

	for _, email := range []string{"de@example.com", "xx@example.com"} {
		super.must(http.StatusCreated, http.MethodPost, "/users", map[string]any{
			"email": email, "first_name": "R", "password": "reader-password-1", "confirm_password": "reader-password-1",
		}, nil)
	}

	b := s.browser()
	b.account(http.MethodPost, "/forgot-password", map[string]string{"email": "de@example.com", "language": "de"}, nil)
	b.account(http.MethodPost, "/forgot-password", map[string]string{"email": "xx@example.com", "language": "xx"}, nil)

	german := s.mail.wait(t, "de@example.com")
	if german.Subject != "Passwort zurücksetzen" || !strings.Contains(german.Body, "Wählen Sie hier") {
		t.Errorf("the email to a German reader = %q / %q, want it in German", german.Subject, german.Body)
	}
	if !strings.Contains(german.Body, testAccountURL+"/reset-password?") {
		t.Errorf("the German email has no link: %q", german.Body)
	}

	fallback := s.mail.wait(t, "xx@example.com")
	if fallback.Subject != "Reset your password" {
		t.Errorf("the email for a language nobody offers = %q, want the default language's", fallback.Subject)
	}
}

// publicInto asks the public server's account API and decodes any answer.
func (s *liveServer) publicInto(path string, out any) int {
	s.t.Helper()

	res, err := http.Get(s.root + "/api/v1/account" + path)
	if err != nil {
		s.t.Fatal(err)
	}
	defer res.Body.Close()

	_ = json.NewDecoder(res.Body).Decode(out)

	return res.StatusCode
}
