package sso

import (
	"strings"
	"testing"
)

// The panel shows a new connection's addresses before it exists, built on
// the identifier it works out the same way (identifierFrom, in the console's
// sso/form.svelte.ts); these cases are the ones its tests hold it to.
func TestSlugFromMatchesWhatThePanelShows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, sent, from, want string
	}{
		{name: "from the name", from: "Acme Corp", want: "acme-corp"},
		{name: "punctuation and spaces collapse", from: "  Acme & Co. (EU) ", want: "acme-co-eu"},
		{name: "one sent is lower-cased and kept", sent: " Okta-Main ", from: "Acme", want: "okta-main"},
		{name: "no name, no identifier", want: ""},
		{name: "at most 64 characters, no dash left at the end", from: strings.Repeat("a", 63) + " b", want: strings.Repeat("a", 63)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := slugFrom(tt.sent, tt.from); got != tt.want {
				t.Errorf("slugFrom(%q, %q) = %q, want %q", tt.sent, tt.from, got, tt.want)
			}
		})
	}
}
