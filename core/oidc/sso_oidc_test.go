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
