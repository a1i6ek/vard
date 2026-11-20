package model

import (
	"testing"
	"time"
)

// A refresh token ends at the sooner of its idle and absolute expiry, and
// one from before there was an idle expiry lasts as it always did.
func TestARefreshTokenGoesIdle(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	soon, later := now.Add(time.Hour), now.Add(24*time.Hour)

	tests := []struct {
		name   string
		token  RefreshToken
		at     time.Time
		usable bool
		lasts  time.Time
	}{
		{"used before it goes idle", RefreshToken{ExpiresAt: later, IdleExpiresAt: &soon}, now, true, soon},
		{"unused past its idle expiry", RefreshToken{ExpiresAt: later, IdleExpiresAt: &soon}, soon, false, soon},
		{"idle expiry past the family's end", RefreshToken{ExpiresAt: soon, IdleExpiresAt: &later}, now, true, soon},
		{"from before idle expiry", RefreshToken{ExpiresAt: later}, soon, true, later},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.Usable(tt.at); got != tt.usable {
				t.Errorf("Usable = %v, want %v", got, tt.usable)
			}
			if got := tt.token.Lasts(); !got.Equal(tt.lasts) {
				t.Errorf("Lasts = %v, want %v", got, tt.lasts)
			}
		})
	}
}
