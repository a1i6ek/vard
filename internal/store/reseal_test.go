package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/a1i6ek/vard/internal/crypto/jose"
	"github.com/a1i6ek/vard/internal/model"
)

// Every sealed column is resealed, each bound exactly as the model binds it:
// a column missing here would keep values only the old key opens, and one
// bound differently would fail to open, or worse, open under another row.
func TestEverySealedColumnIsResealed(t *testing.T) {
	id := uuid.New()
	base := model.Base{ID: id}
	want := map[string]jose.Context{
		"server_secrets.value":            model.ServerSecret{Name: "tag"}.ValueWhere(),
		"signing_keys.private_key":        model.SigningKey{KID: "kid-1"}.PrivateKeyWhere(),
		"social_providers.client_secret":  model.SocialProvider{Base: base}.ClientSecretWhere(),
		"social_providers.private_key":    model.SocialProvider{Base: base}.PrivateKeyWhere(),
		"sso_connections.client_secret":   model.SSOConnection{Base: base}.ClientSecretWhere(),
		"sso_connections.sp_key":          model.SSOConnection{Base: base}.SPKeyWhere(),
		"mail_settings.password":          model.MailSettings{Base: base}.PasswordWhere(),
		"user_factors.secret":             model.UserFactor{Base: base}.SecretWhere(),
		"mfa_factors.secret":              model.MFA{Base: base}.SecretWhere(),
		"refresh_tokens.successor_sealed": model.RefreshToken{Base: base}.SuccessorWhere(),
		"sso_logins.saml_response":        model.SAMLResponseWhere("relay-hash"),
	}
	keyOf := map[string]string{
		"server_secrets": "tag", "signing_keys": "kid-1", "sso_logins": "relay-hash",
	}

	if len(SealedColumns) != len(want) {
		t.Errorf("%d sealed columns are resealed, want %d", len(SealedColumns), len(want))
	}
	for _, column := range SealedColumns {
		name := column.Table + "." + column.Column
		expected, ok := want[name]
		if !ok {
			t.Errorf("%s is resealed, but the model binds nothing there", name)
			continue
		}
		key, ok := keyOf[column.Table]
		if !ok {
			key = id.String()
		}
		if got := column.Where(key); got != expected {
			t.Errorf("%s is resealed bound to %q, the model seals it bound to %q", name, got, expected)
		}
	}

	// Every binding the model makes is one of them.
	bindings := 0
	files, _ := filepath.Glob("../model/*.go")
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		bindings += strings.Count(string(source), "jose.Bind(")
	}
	if bindings != len(want) {
		t.Errorf("the model binds %d sealed columns, this test knows %d: add the new one to SealedColumns and here", bindings, len(want))
	}
}
