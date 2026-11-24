package session

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// A request carries an application's token whether it sends it as Bearer or
// as DPoP: everything that refuses one (the Consent and Agreements answers,
// two-step changes, setup) refuses both.
func TestATokenSentAsDPoPIsStillAToken(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"Bearer abc", "abc"},
		{"bearer abc", "abc"},
		{"DPoP abc", "abc"},
		{"dpop abc", "abc"},
		{"Basic abc", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/api/v1/account/consent", nil)
			if tt.header != "" {
				c.Request.Header.Set("Authorization", tt.header)
			}
			if got := Bearer(c); got != tt.want {
				t.Errorf("Bearer with %q = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}
