package oidc

import "testing"

// An address is proved by a provider's email_verified, never by its
// absence unless the connection says so, and never against an explicit
// false.
func TestVouchedFor(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		assume bool
		want   bool
	}{
		{name: "true", value: true, want: true},
		{name: "the string true, as Cognito sends it", value: " TRUE ", want: true},
		{name: "false", value: false, want: false},
		{name: "the string false", value: "false", want: false},
		{name: "false on a connection that assumes", value: false, assume: true, want: false},
		{name: "the string false on a connection that assumes", value: "false", assume: true, want: false},
		{name: "unsaid", value: nil, want: false},
		{name: "unsaid on a connection that assumes", value: nil, assume: true, want: true},
		{name: "a number is not a boolean", value: 1.0, want: false},
		{name: "a number on a connection that assumes is not unsaid", value: 1.0, assume: true, want: false},
		{name: "an object on a connection that assumes is not unsaid", value: map[string]any{}, assume: true, want: false},
		{name: "any other string", value: "yes", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := vouchedFor(tt.value, tt.assume); got != tt.want {
				t.Errorf("vouchedFor(%#v, %v) = %v, want %v", tt.value, tt.assume, got, tt.want)
			}
		})
	}
}

// email_verified speaks for the email claim alone: an address mapped from
// any other is unsaid, and passes only on a connection that assumes.
func TestEmailVerifiedSpeaksOnlyForTheEmailClaim(t *testing.T) {
	claims := map[string]any{
		"email": "ada@acme.test", "upn": "ada@acme.test", "preferred_username": "ada@acme.test", "email_verified": true,
	}
	tests := []struct {
		name   string
		from   string
		assume bool
		want   bool
	}{
		{name: "the email claim", from: "email", want: true},
		{name: "upn", from: "upn", want: false},
		{name: "preferred_username", from: "preferred_username", want: false},
		{name: "upn on a connection that assumes", from: "upn", assume: true, want: true},
		{name: "a nested claim named like email", from: "profile.email", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := vouchedFor(verifiedClaim(claims, tt.from), tt.assume); got != tt.want {
				t.Errorf("an address from %s (assume %v) vouched for = %v, want %v", tt.from, tt.assume, got, tt.want)
			}
		})
	}

	// Mapped from upn on a connection that assumes, an explicit false about
	// the email claim says nothing about the upn either way.
	unverified := map[string]any{"upn": "ada@acme.test", "email_verified": false}
	if !vouchedFor(verifiedClaim(unverified, "upn"), true) {
		t.Error("email_verified false about the email claim refused an address from upn on a connection that assumes")
	}
}
