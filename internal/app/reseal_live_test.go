package app

import (
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"

	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/database/databasetest"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
	"github.com/a1i6ek/vard/internal/platform/config"
	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
)

// keys reseal moves every sealed value under the current key: afterwards the
// new key alone opens each, still bound to its place, and the old one may be
// dropped. A second run changes nothing, so a run that stopped is run again.
func TestLiveResealMovesEverySecretToTheCurrentKey(t *testing.T) {
	const old = "the-old-secret-key-for-reseal-0123456789abcdefg"
	const current = "the-new-secret-key-for-reseal-0123456789abcdefg"

	db, dbCfg := databasetest.Open(t)
	ctx := context.Background()
	before, err := jose.NewSealer(old)
	if err != nil {
		t.Fatal(err)
	}

	// A bytea column keyed by name, one keyed by id, and a base64 text one.
	tag := model.ServerSecret{Name: "reseal-test"}
	tagSealed, err := before.SealFor(tag.ValueWhere(), []byte("installation tag"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO server_secrets (name, value, created_at) VALUES (?, ?, now())", tag.Name, tagSealed).Error; err != nil {
		t.Fatal(err)
	}

	provider := model.SocialProvider{Base: model.Base{ID: uuid.New()}, Kind: model.SocialGoogle, Slug: "google", Name: "Google", ClientID: "id", Scopes: model.StringList{}}
	if provider.ClientSecret, err = before.SealFor(provider.ClientSecretWhere(), []byte("provider secret")); err != nil {
		t.Fatal(err)
	}
	if err := db.Omit(clause.Associations).Create(&provider).Error; err != nil {
		t.Fatal(err)
	}

	user := model.User{Email: "ada@example.com", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	factor := model.UserFactor{Base: model.Base{ID: uuid.New()}, UserID: user.ID, Method: model.MFAMethodTOTP}
	sealedSeed, err := before.SealFor(factor.SecretWhere(), []byte("totp seed"))
	if err != nil {
		t.Fatal(err)
	}
	factor.Secret = base64.StdEncoding.EncodeToString(sealedSeed)
	if err := db.Omit(clause.Associations).Create(&factor).Error; err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{DB: dbCfg, SecretKey: current, OldSecretKeys: []string{old}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	changed := 0
	count := func(string) { changed++ }
	if err := Reseal(ctx, cfg, log, count); err != nil {
		t.Fatalf("reseal = %v", err)
	}
	if changed != len(store.SealedColumns) {
		t.Errorf("reseal reported %d columns, want %d", changed, len(store.SealedColumns))
	}

	alone, err := jose.NewSealer(current)
	if err != nil {
		t.Fatal(err)
	}
	var storedTag []byte
	if err := db.Raw("SELECT value FROM server_secrets WHERE name = ?", tag.Name).Row().Scan(&storedTag); err != nil {
		t.Fatal(err)
	}
	var storedProvider model.SocialProvider
	db.First(&storedProvider, "id = ?", provider.ID)
	var storedFactor model.UserFactor
	db.First(&storedFactor, "id = ?", factor.ID)
	seed, _ := base64.StdEncoding.DecodeString(storedFactor.Secret)

	for name, tt := range map[string]struct {
		where  jose.Context
		sealed []byte
		want   string
	}{
		"the installation's tag": {tag.ValueWhere(), storedTag, "installation tag"},
		"a provider's secret":    {provider.ClientSecretWhere(), storedProvider.ClientSecret, "provider secret"},
		"a two-step seed":        {factor.SecretWhere(), seed, "totp seed"},
	} {
		plain, err := alone.OpenFor(tt.where, tt.sealed)
		if err != nil || string(plain) != tt.want {
			t.Errorf("%s under the new key alone = %q, %v", name, plain, err)
		}
	}

	// A second run finds nothing left to move.
	moved := 0
	sealer, _ := jose.NewSealer(current, old)
	st := store.New(db)
	for _, column := range store.SealedColumns {
		n, err := st.Reseal(ctx, column, sealer)
		if err != nil {
			t.Fatal(err)
		}
		moved += n
	}
	if moved != 0 {
		t.Errorf("a second run sealed %d values again, want none", moved)
	}
}
