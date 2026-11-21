package integration

import (
	"net/http"
	"net/url"
	"slices"
	"testing"
)

// Adding a passkey asks for a discoverable credential and the person
// verified (WebAuthn residentKey and userVerification required), since every
// passkey may later sign in on its own; an authenticator that does not
// verify its user is refused.
func TestLiveAPasskeyIsMadeToSignInOnItsOwn(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	f.withSteps("identifier", "password", "totp")
	owner := f.s.browser()
	_, challenge := pkce(t)
	f.signIn(owner, challenge)

	var begun struct {
		Passkey struct {
			AuthenticatorSelection map[string]any `json:"authenticatorSelection"`
		} `json:"passkey"`
	}
	owner.account(http.MethodPost, "/two-step/passkeys", map[string]any{"current_password": adaPassword}, &begun)
	selection := begun.Passkey.AuthenticatorSelection
	if selection["residentKey"] != "required" || selection["requireResidentKey"] != true || selection["userVerification"] != "required" {
		t.Errorf("the registration asks for %v, want a discoverable credential and the person verified", selection)
	}

	var options struct {
		Passkey map[string]any `json:"passkey"`
	}
	owner.account(http.MethodPost, "/two-step/passkeys", map[string]any{"current_password": adaPassword}, &options)
	key := newVirtualKey(t)
	key.unverified = true
	if status := owner.account(http.MethodPost, "/two-step/passkeys/finish", map[string]any{
		"label": "Unverifying key", "response": key.create(t, options.Passkey),
	}, nil); status == http.StatusOK {
		t.Error("a key that did not verify its user was kept")
	}
}

// An ID token's amr says what kind of key answered (RFC 8176): hwk for one
// bound to its device, swk for a passkey that may be synced (the BE flag),
// and user only when the person was verified, whether the passkey signed in
// alone or answered as a second factor.
func TestLiveAPasskeysAMRSaysWhatItsFlagsSay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		synced     bool
		alone      bool // signed in with the passkey alone, not after a password
		unverified bool // answered without verifying its user (second factor only)
		want       []string
		never      []string
	}{
		{name: "a security key, alone", alone: true, want: []string{"hwk", "user", "mfa"}, never: []string{"swk", "pwd"}},
		{name: "a synced passkey, alone", synced: true, alone: true, want: []string{"swk", "user", "mfa"}, never: []string{"hwk"}},
		{name: "a security key after the password, verified", want: []string{"pwd", "hwk", "user", "mfa"}, never: []string{"swk"}},
		{name: "a security key after the password, only touched", unverified: true, want: []string{"pwd", "hwk", "mfa"}, never: []string{"user"}},
		{name: "a synced passkey after the password", synced: true, want: []string{"pwd", "swk", "user", "mfa"}, never: []string{"hwk"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newOAuthFixture(t)
			f.withSteps("identifier", "password", "totp")
			owner := f.s.browser()
			_, challenge := pkce(t)
			f.signIn(owner, challenge)
			key := newVirtualKey(t)
			key.synced = tt.synced
			var begun struct {
				Passkey map[string]any `json:"passkey"`
			}
			owner.account(http.MethodPost, "/two-step/passkeys", map[string]any{"current_password": adaPassword}, &begun)
			if status := owner.account(http.MethodPost, "/two-step/passkeys/finish", map[string]any{
				"label": "Key", "response": key.create(t, begun.Passkey),
			}, nil); status != http.StatusOK {
				t.Fatalf("adding the key = %d", status)
			}
			key.unverified = tt.unverified

			b := f.s.browser()
			verifier, challenge := pkce(t)
			login := b.visit(f.authorizeURL(challenge, nil))
			request := login.Query().Get("request")

			var to string
			if tt.alone {
				var offer struct {
					Handle  string         `json:"handle"`
					Passkey map[string]any `json:"passkey"`
				}
				b.account(http.MethodPost, "/login/passkey", map[string]any{"request": request}, &offer)
				var out struct {
					RedirectTo string `json:"redirect_to"`
				}
				b.account(http.MethodPost, "/login/passkey/finish", map[string]any{"handle": offer.Handle, "response": key.get(t, offer.Passkey)}, &out)
				to = out.RedirectTo
			} else {
				b.account(http.MethodPost, "/login", map[string]any{"request": request, "email": adaEmail, "password": adaPassword}, nil)
				var offer secondFactorOffer
				b.account(http.MethodGet, "/login/second-factor", nil, &offer)
				_, to = answerSecondFactor(b, map[string]any{"passkey": key.get(t, offer.SecondFactor.Passkey)})
			}

			back, _ := url.Parse(to)
			if back == nil || back.Query().Get("code") == "" {
				t.Fatalf("the sign-in led to %q, want the application with a code", to)
			}
			tokens := f.exchange(back.Query().Get("code"), verifier)
			_, claims := f.verifyJWT(tokens.str("id_token"))
			amr := []string{}
			for _, value := range claims["amr"].([]any) {
				amr = append(amr, value.(string))
			}
			for _, value := range tt.want {
				if !slices.Contains(amr, value) {
					t.Errorf("amr = %v, want %s in it", amr, value)
				}
			}
			for _, value := range tt.never {
				if slices.Contains(amr, value) {
					t.Errorf("amr = %v, want no %s", amr, value)
				}
			}
		})
	}
}
