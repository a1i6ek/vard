package model

import (
	"slices"
	"testing"
)

func TestAMRSaysHowSomeoneSignedIn(t *testing.T) {
	factor := MethodSecondFactor

	tests := []struct {
		name    string
		methods []string
		want    []string
	}{
		{"a password", []string{AMRPassword}, []string{AMRPassword}},
		{"a password and an emailed code are not a second factor", []string{AMRPassword, AMROneTimeCode}, []string{AMRPassword, AMROneTimeCode}},
		{"a password and an authenticator app", []string{AMRPassword, AMROneTimeCode, factor}, []string{AMRPassword, AMROneTimeCode, AMRMultiFactor}},
		{"a password and a passkey as the second factor", []string{AMRPassword, AMRHardwareKey, factor}, []string{AMRPassword, AMRHardwareKey, AMRMultiFactor}},
		{"a passkey with the person verified", []string{AMRHardwareKey, AMRUserPresence, factor}, []string{AMRHardwareKey, AMRUserPresence, AMRMultiFactor}},
		{"two kinds without the account's own factor", []string{AMRHardwareKey, AMRUserPresence}, []string{AMRHardwareKey, AMRUserPresence}},
		{"a code after another provider", []string{AMROneTimeCode, factor}, []string{AMROneTimeCode, AMRMultiFactor}},
		{"another provider alone", nil, nil},
		{"the marker alone is no claim", []string{factor}, nil},
		{"two codes are still one", []string{AMROneTimeCode, AMROneTimeCode}, []string{AMROneTimeCode}},
		{"something unknown is left out", []string{"face", AMRPassword}, []string{AMRPassword}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AMR(tt.methods); !slices.Equal(got, tt.want) {
				t.Errorf("AMR(%v) = %v, want %v", tt.methods, got, tt.want)
			}
		})
	}
}

// The marker is how this server remembers a second factor; it is no amr
// value, and never reaches a token.
func TestTheSecondFactorMarkerIsNeverAClaim(t *testing.T) {
	if _, ok := amrKind[MethodSecondFactor]; ok {
		t.Fatalf("%q is an amr kind", MethodSecondFactor)
	}
	if slices.Contains(AMR([]string{AMRPassword, MethodSecondFactor}), MethodSecondFactor) {
		t.Errorf("AMR passed %q on", MethodSecondFactor)
	}
}

func TestWithSecondFactorAddsEachMethodOnce(t *testing.T) {
	tests := []struct {
		name    string
		methods []string
		proved  string
		want    StringList
	}{
		{"a password and a code", []string{AMRPassword}, AMROneTimeCode, StringList{AMRPassword, AMROneTimeCode, MethodSecondFactor}},
		{"an emailed code, then an authenticator", []string{AMRPassword, AMROneTimeCode}, AMROneTimeCode, StringList{AMRPassword, AMROneTimeCode, MethodSecondFactor}},
		{"a step-up of a session that had one", []string{AMRPassword, AMRHardwareKey, MethodSecondFactor}, AMROneTimeCode, StringList{AMRPassword, AMRHardwareKey, MethodSecondFactor, AMROneTimeCode}},
		{"nothing proved before", nil, AMRHardwareKey, StringList{AMRHardwareKey, MethodSecondFactor}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := slices.Clone(tt.methods)
			got := WithSecondFactor(tt.methods, tt.proved)
			if !slices.Equal(got, tt.want) {
				t.Errorf("WithSecondFactor(%v, %q) = %v, want %v", tt.methods, tt.proved, got, tt.want)
			}
			if !slices.Equal(tt.methods, before) {
				t.Errorf("WithSecondFactor changed what it was given: %v", tt.methods)
			}
		})
	}
}

// A passkey's amr is what its flags say: a key that may be synced is a
// software key, one bound to its device a hardware key, and user only when
// the person was verified. Neither pin nor fpt is ever guessed.
func TestAPasskeyClaimsOnlyWhatItsFlagsSay(t *testing.T) {
	tests := []struct {
		name             string
		synced, verified bool
		want             []string
	}{
		{"a security key, the person verified", false, true, []string{AMRHardwareKey, AMRUserPresence}},
		{"a security key, only touched", false, false, []string{AMRHardwareKey}},
		{"a synced passkey, the person verified", true, true, []string{AMRSoftwareKey, AMRUserPresence}},
		{"a synced passkey, not verified", true, false, []string{AMRSoftwareKey}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PasskeyAMR(tt.synced, tt.verified)
			if !slices.Equal(got, tt.want) {
				t.Errorf("PasskeyAMR = %v, want %v", got, tt.want)
			}
			claimed := AMR(WithSecondFactor(nil, got...))
			if slices.Contains(claimed, "pin") || slices.Contains(claimed, "fpt") || !slices.Contains(claimed, AMRMultiFactor) {
				t.Errorf("AMR = %v, want no pin or fpt, and mfa", claimed)
			}
		})
	}
}
