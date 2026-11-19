package integration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/a1i6ek/vard/internal/mail"
	"github.com/a1i6ek/vard/internal/model"
)

// Pre-hijacking: someone registers an address before its owner does, gives
// the account a way back in, and waits. The owner cannot confirm the address
// with the verification link alone — it asks for a password the stranger
// chose — so they prove it with a reset link, and everything the account
// held before that moment goes, the stranger's password with it.

const victimEmail = "victim@example.com"

// preRegister makes the account for victimEmail the way a stranger would: by
// registering it at an application, unverified, with adaPassword (so the
// two-step helpers, which confirm with it, work for them too). The flow lets
// an unverified address in, as an existing installation's may.
func (f *oauthFixture) preRegister(t *testing.T) *browser {
	t.Helper()

	f.super.must(http.StatusOK, http.MethodPatch, "/applications/"+f.appID, map[string]any{
		"name": "Shop", "profile": "web",
		"grant_types":        []string{"authorization_code", "refresh_token"},
		"redirect_uris":      []string{shopRedirect},
		"scopes":             []string{"openid", "profile", "email", "offline_access", "roles"},
		"allow_registration": true,
	}, nil)
	f.super.must(http.StatusOK, http.MethodPatch, "/login-flows/"+defaultFlow(t, f.super), map[string]any{
		"steps":                    []string{"identifier", "password", "social", "totp"},
		"require_verified_email":   false,
		"verify_email_on_register": true,
	}, nil)

	stranger := f.s.browser()
	_, challenge := pkce(t)
	login := stranger.visit(f.authorizeURL(challenge, nil))
	if status := stranger.account(http.MethodPost, "/register", map[string]any{
		"request": login.Query().Get("request"), "email": victimEmail, "password": adaPassword,
	}, nil); status != http.StatusOK {
		t.Fatalf("the stranger registering the address = %d", status)
	}

	return stranger
}

// held counts what the account at victimEmail still holds.
func (f *oauthFixture) held(t *testing.T, table string) int64 {
	t.Helper()

	var count int64
	err := f.s.db.Table(table).
		Joins("JOIN users ON users.id = "+table+".user_id").
		Where("users.email = ?", victimEmail).
		Count(&count).Error
	if err != nil {
		t.Fatal(err)
	}

	return count
}

func TestLiveProvingAnAddressEndsWhatAStrangerSetUpBeforeIt(t *testing.T) {
	t.Parallel()

	ownerResets := func(t *testing.T, f *oauthFixture) {
		f.s.browser().account(http.MethodPost, "/forgot-password", map[string]string{"email": victimEmail}, nil)
		token := tokenIn(t, f.s.mail.waitFor(t, victimEmail, "/reset-password").Body, "/reset-password")
		if status := f.s.browser().account(http.MethodPost, "/reset-password", map[string]string{
			"token": token, "password": "owner-chose-this-1",
		}, nil); status != http.StatusOK {
			t.Fatalf("the owner's reset = %d", status)
		}
	}
	// The verification link does not take the owner in: it asks for the
	// stranger's password, which they cannot give, and proves nothing until
	// they do — so they set their own with a reset instead.
	ownerVerifies := func(t *testing.T, f *oauthFixture) {
		token := tokenIn(t, f.s.mail.waitFor(t, victimEmail, "/verify-email").Body, "/verify-email")
		owner := f.s.browser()
		for _, attempt := range []struct {
			body map[string]string
			code string
		}{
			{map[string]string{"token": token}, "verification_password_required"},
			{map[string]string{"token": token, "password": "a-guess-at-theirs"}, "wrong_password"},
		} {
			var refused problemBody
			if status := owner.account(http.MethodPost, "/verify-email", attempt.body, &refused); status < 400 || refused.Code != attempt.code {
				t.Fatalf("the owner's verification = %d %+v, want %s", status, refused, attempt.code)
			}
		}
		if left := f.held(t, "user_factors"); left == 0 {
			t.Fatal("the refused link cleared what the stranger set up, which only a proof may do")
		}
		ownerResets(t, f)
	}

	cases := []struct {
		name  string
		setUp func(t *testing.T, stranger *browser) *virtualKey
		prove func(t *testing.T, f *oauthFixture)
	}{
		{
			name: "an authenticator, then the owner resets the password",
			setUp: func(t *testing.T, stranger *browser) *virtualKey {
				addAuthenticator(t, stranger)
				return nil
			},
			prove: ownerResets,
		},
		{
			name:  "a passkey, then the owner, refused at the verification link, resets the password",
			setUp: addPasskey,
			prove: ownerVerifies,
		},
		{
			name:  "a passkey, then the owner resets the password",
			setUp: addPasskey,
			prove: ownerResets,
		},
		{
			name: "an authenticator, then the owner, refused at the verification link, resets the password",
			setUp: func(t *testing.T, stranger *browser) *virtualKey {
				addAuthenticator(t, stranger)
				return nil
			},
			prove: ownerVerifies,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newOAuthFixture(t)
			stranger := f.preRegister(t)
			key := c.setUp(t, stranger)

			// What the stranger allowed the application stays allowed for the
			// owner unless it goes too.
			if err := f.s.db.Exec(`INSERT INTO consents (id, created_at, updated_at, user_id, application_id, scopes)
				SELECT gen_random_uuid(), now(), now(), id, ?, '["openid"]' FROM users WHERE email = ?`, f.appID, victimEmail).Error; err != nil {
				t.Fatal(err)
			}

			c.prove(t, f)

			for _, table := range []string{"user_factors", "user_recovery_codes", "consents", "pending_sign_ins"} {
				if left := f.held(t, table); left != 0 {
					t.Errorf("%s still holds %d rows set up before the address was proved", table, left)
				}
			}
			if status := stranger.account(http.MethodGet, "/me", nil, nil); status != http.StatusUnauthorized {
				t.Errorf("the stranger's session = %d after the owner proved the address, want 401", status)
			}
			if status := f.s.browser().account(http.MethodPost, "/login", map[string]string{"email": victimEmail, "password": adaPassword}, nil); status == http.StatusOK {
				t.Error("the stranger's password still signs in after the owner proved the address")
			}

			if key != nil {
				b := f.s.browser()
				_, challenge := pkce(t)
				login := b.visit(f.authorizeURL(challenge, nil))
				var offer struct {
					Handle  string         `json:"handle"`
					Passkey map[string]any `json:"passkey"`
				}
				b.account(http.MethodPost, "/login/passkey", map[string]any{"request": login.Query().Get("request")}, &offer)
				var out struct {
					RedirectTo string `json:"redirect_to"`
				}
				if status := b.account(http.MethodPost, "/login/passkey/finish", map[string]any{
					"handle": offer.Handle, "response": key.get(t, offer.Passkey),
				}, &out); status == http.StatusOK {
					t.Errorf("the stranger's passkey still signs in after the owner proved the address: %s", out.RedirectTo)
				}
			}

			var audit int64
			if err := f.s.db.Model(&model.AuditLog{}).Where("action = ?", "user.address_proved").Count(&audit).Error; err != nil {
				t.Fatal(err)
			}
			if audit != 1 {
				t.Errorf("%d user.address_proved entries, want 1", audit)
			}
		})
	}
}

// An account already verified keeps its factors through a reset: only the
// first proof of an address clears what came before it.
func TestLiveAResetOfAVerifiedAccountKeepsItsFactors(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	f.withSteps("identifier", "password", "totp")

	owner := f.s.browser()
	_, challenge := pkce(t)
	f.signIn(owner, challenge)
	addAuthenticator(t, owner)

	f.s.browser().account(http.MethodPost, "/forgot-password", map[string]string{"email": adaEmail}, nil)
	token := tokenIn(t, f.s.mail.waitFor(t, adaEmail, "/reset-password").Body, "/reset-password")
	if status := f.s.browser().account(http.MethodPost, "/reset-password", map[string]string{
		"token": token, "password": "ada-new-password-1",
	}, nil); status != http.StatusOK {
		t.Fatalf("reset = %d", status)
	}

	var left int64
	if err := f.s.db.Table("user_factors").Where("user_id = ?", f.userID).Count(&left).Error; err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Errorf("a verified account's reset left %d factors, want its 1", left)
	}
}

// A provider that has not verified an address may not make an account for
// it: whoever controls that provider account would hold it before its owner.
func TestLiveSocialSignInRefusesToMakeAnAccountForAnUnverifiedAddress(t *testing.T) {
	t.Parallel()

	s := newLiveServer(t)
	super := s.superAdmin()

	provider := newFakeProvider(t, map[string]any{
		"sub": "stranger-1", "email": victimEmail, "email_verified": false,
	})
	b, start := provider.register(t, s, super, nil)

	landed := through(t, b, start)
	if !strings.HasPrefix(landed.String(), testAccountURL+"/error") || landed.Query().Get("reason") != "social_email_unverified" {
		t.Fatalf("an unverified address landed at %s, want the error page with social_email_unverified", landed)
	}

	var users struct {
		Users []json.RawMessage `json:"users"`
	}
	super.must(http.StatusOK, http.MethodGet, "/users?search="+url.QueryEscape(victimEmail), nil, &users)
	if len(users.Users) != 0 {
		t.Errorf("%d accounts were made for an address the provider did not verify", len(users.Users))
	}
}

// An identity linked before the owner proved the address — on an
// installation from before that was refused — signs nobody in afterwards.
func TestLiveAStrangersSocialLinkDiesWithTheOwnersReset(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	f.preRegister(t)

	provider := newFakeProvider(t, map[string]any{
		"sub": "stranger-1", "email": victimEmail, "email_verified": false,
	})
	b, start := provider.register(t, f.s, f.super, nil)

	var providerID, userID string
	if err := f.s.db.Table("social_providers").Select("id").Where("slug = ?", "fake").Scan(&providerID).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.s.db.Table("users").Select("id").Where("email = ?", victimEmail).Scan(&userID).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.s.db.Exec(`INSERT INTO social_identities (id, created_at, updated_at, user_id, provider_id, subject, email)
		VALUES (gen_random_uuid(), now(), now(), ?, ?, 'stranger-1', ?)`, userID, providerID, victimEmail).Error; err != nil {
		t.Fatal(err)
	}

	if landed := through(t, b, start); strings.HasPrefix(landed.String(), testAccountURL+"/error") {
		t.Fatalf("the planted link did not sign in before the reset, so it proves nothing: %s", landed)
	}

	f.s.browser().account(http.MethodPost, "/forgot-password", map[string]string{"email": victimEmail}, nil)
	token := tokenIn(t, f.s.mail.waitFor(t, victimEmail, "/reset-password").Body, "/reset-password")
	if status := f.s.browser().account(http.MethodPost, "/reset-password", map[string]string{
		"token": token, "password": "owner-chose-this-1",
	}, nil); status != http.StatusOK {
		t.Fatalf("the owner's reset = %d", status)
	}

	again := f.s.browser()
	if landed := through(t, again, start); !strings.HasPrefix(landed.String(), testAccountURL+"/error") {
		t.Fatalf("the stranger's social sign-in after the owner's reset landed at %s, want refused", landed)
	}
	if status := again.account(http.MethodGet, "/me", nil, nil); status == http.StatusOK {
		t.Error("the stranger is signed in to the owner's account")
	}
}

// waitFor is wait for the first message to `to` carrying a link to page.
func (m *mailbox) waitFor(t *testing.T, to, page string) mail.Message {
	t.Helper()

	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		m.mu.Lock()
		for _, msg := range m.messages {
			if strings.EqualFold(msg.To, to) && strings.Contains(msg.Body, testAccountURL+page) {
				m.mu.Unlock()
				return msg
			}
		}
		m.mu.Unlock()
	}

	t.Fatalf("no email to %s with a %s link", to, page)
	return mail.Message{}
}

// A verification link takes its account's holder as well as the inbox: the
// account's password, or a browser signed in to it. Without, a stranger who
// changed their own account's address to somebody else's would have that
// person confirm it — and move into an account whose password the stranger
// knows. The holder themselves confirms as before.
func TestLiveAVerificationLinkTakesTheAccountsHolder(t *testing.T) {
	t.Parallel()

	f := newOAuthFixture(t)
	f.super.must(http.StatusOK, http.MethodPatch, "/login-flows/"+defaultFlow(t, f.super), map[string]any{
		"require_verified_email": false, "allow_email_change": true,
	}, nil)

	// A stranger's own account, signed in, asks to move to the victim's
	// address with its own password; the link goes to the victim.
	var made struct {
		User idOnly `json:"user"`
	}
	f.super.must(http.StatusCreated, http.MethodPost, "/users", map[string]any{
		"email": "stranger@example.com", "password": "stranger-password-1", "confirm_password": "stranger-password-1", "email_verified": true,
	}, &made)
	stranger := f.s.browser()
	if status := stranger.account(http.MethodPost, "/login", map[string]string{"email": "stranger@example.com", "password": "stranger-password-1"}, nil); status != http.StatusOK {
		t.Fatalf("the stranger signing in = %d", status)
	}
	if status := stranger.account(http.MethodPost, "/email", map[string]string{"email": victimEmail, "current_password": "stranger-password-1"}, nil); status != http.StatusAccepted {
		t.Fatalf("asking to move to the victim's address = %d", status)
	}
	token := tokenIn(t, f.s.mail.wait(t, victimEmail).Body, "/verify-email")

	// The victim, curious, follows it: refused, and the link still waits.
	var refused problemBody
	if status := f.s.browser().account(http.MethodPost, "/verify-email", map[string]string{"token": token}, &refused); status != http.StatusForbidden || refused.Code != "verification_password_required" {
		t.Errorf("the victim following the link = %d %+v, want verification_password_required", status, refused)
	}
	var moved int64
	if err := f.s.db.Raw("SELECT count(*) FROM users WHERE email = ?", victimEmail).Scan(&moved).Error; err != nil {
		t.Fatal(err)
	}
	if moved != 0 {
		t.Fatal("the stranger's account moved to the victim's address without its password")
	}

	// The account's holder, in the browser signed in to it, confirms with
	// nothing more.
	if status := stranger.account(http.MethodPost, "/verify-email", map[string]string{"token": token}, nil); status != http.StatusOK {
		t.Errorf("the holder's own browser confirming = %d, want 200", status)
	}
}
