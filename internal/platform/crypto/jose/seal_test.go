package jose

import (
	"bytes"
	"testing"
)

const (
	secretNow    = "the-secret-in-use-now-0123456789abcdef"
	secretBefore = "the-secret-used-before-0123456789abcdef"
)

// A sealed value opens only where it was sealed for: the SMTP password moved
// into a refresh token's sealed successor — which would be handed out as a
// refresh token — does not open there, nor in another row of its own column.
func TestASealedValueOpensOnlyWhereItWasSealedFor(t *testing.T) {
	sealer, err := NewSealer(secretNow)
	if err != nil {
		t.Fatal(err)
	}

	here := Bind("mail_settings", "password", "0192")
	sealed, err := sealer.SealFor(here, []byte("smtp password"))
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		where Context
		opens bool
	}{
		{"where it was sealed for", here, true},
		{"another table's column", Bind("refresh_tokens", "successor_sealed", "0192"), false},
		{"another row of the same column", Bind("mail_settings", "password", "0193"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plain, err := sealer.OpenFor(c.where, sealed)
			if c.opens != (err == nil) {
				t.Fatalf("OpenFor = %q, %v; want it to open: %v", plain, err, c.opens)
			}
			if c.opens && string(plain) != "smtp password" {
				t.Errorf("opened %q", plain)
			}
		})
	}
}

// The installation's tag key makes a keyed hash the same under any
// VARD_SECRET_KEY; without it, a hash follows the secret.
func TestATagFollowsTheTagKeyNotTheSecret(t *testing.T) {
	tagKey := []byte("the installation's own tag key, 32b")

	before := mustSealer(t, secretBefore)
	before.UseTagKey(tagKey)
	after := mustSealer(t, secretNow)
	after.UseTagKey(tagKey)

	if !bytes.Equal(before.Tag("purpose", []byte("code")), after.Tag("purpose", []byte("code"))) {
		t.Error("a tag changed with the secret, though the tag key did not")
	}
	if bytes.Equal(mustSealer(t, secretBefore).Tag("purpose", []byte("code")), mustSealer(t, secretNow).Tag("purpose", []byte("code"))) {
		t.Error("without a tag key, two secrets made the same tag")
	}
}

func mustSealer(t *testing.T, secret string) *Sealer {
	t.Helper()
	s, err := NewSealer(secret)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// A keyring seals under its first key and opens what any of its keys sealed,
// each still bound to its place; Reseal moves a value under the current key,
// after which the old key is no longer needed to open it.
func TestAKeyringOpensWhatItsOlderKeysSealed(t *testing.T) {
	const old, current = "the-old-secret-key-for-sealing-0123456789abcdef", "the-new-secret-key-for-sealing-0123456789abcdef"
	where := Bind("social_providers", "client_secret", "row-1")

	before, err := NewSealer(old)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := before.SealFor(where, []byte("provider secret"))
	if err != nil {
		t.Fatal(err)
	}

	ring, err := NewSealer(current, old)
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := ring.OpenFor(where, sealed); err != nil || string(plain) != "provider secret" {
		t.Fatalf("the keyring opening the old key's value = %q, %v", plain, err)
	}
	if ring.Current(sealed) {
		t.Error("a value sealed under the old key is called current")
	}
	if _, err := ring.OpenFor(Bind("social_providers", "client_secret", "row-2"), sealed); err == nil {
		t.Error("the old key's value opened bound to another row")
	}

	resealed, err := ring.Reseal(where, sealed)
	if err != nil || !ring.Current(resealed) {
		t.Fatalf("Reseal = %v, current %v", err, ring.Current(resealed))
	}
	alone, err := NewSealer(current)
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := alone.OpenFor(where, resealed); err != nil || string(plain) != "provider secret" {
		t.Errorf("the resealed value under the new key alone = %q, %v", plain, err)
	}
	if _, err := alone.OpenFor(where, sealed); err == nil {
		t.Error("the new key alone opened what the old one sealed")
	}
}
