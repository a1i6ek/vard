package model

import "slices"

// How a person proved who they are, as an ID token's amr claim says it (RFC
// 8176). A session keeps the methods its sign-in used; mfa is worked out from
// them.
const (
	AMRPassword     = "pwd"
	AMROneTimeCode  = "otp"
	AMRHardwareKey  = "hwk"
	AMRSoftwareKey  = "swk"
	AMRUserPresence = "user"
	AMRMultiFactor  = "mfa"
)

// MethodSecondFactor is kept among a sign-in's methods when it passed one of
// the account's own second factors: an authenticator app, a recovery code or
// a passkey. It is never an amr value. An emailed code is otp too, and proves
// only the mailbox a password reset goes to, so otp alone cannot say a
// second factor was passed.
const MethodSecondFactor = "second_factor"

// amrKind is the kind of factor each method is: something known, something
// held, or the person verified on the device that holds the key.
var amrKind = map[string]string{
	AMRPassword:     "knowledge",
	AMROneTimeCode:  "possession",
	AMRHardwareKey:  "possession",
	AMRSoftwareKey:  "possession",
	AMRUserPresence: "verification",
}

// AMR is the amr claim for a sign-in's methods: each method once, in the
// order used, and mfa when the sign-in passed a second factor of the
// account's own. Nothing proved here (a sign-in through another provider
// alone) is no claim.
func AMR(methods []string) []string {
	out := []string{}
	for _, method := range methods {
		if _, ok := amrKind[method]; ok && !slices.Contains(out, method) {
			out = append(out, method)
		}
	}
	if len(out) == 0 {
		return nil
	}
	if ProvedSecondFactor(methods) {
		out = append(out, AMRMultiFactor)
	}
	return out
}

// ProvedSecondFactor reports whether a sign-in with these methods passed one
// of the account's own second factors.
func ProvedSecondFactor(methods []string) bool {
	return slices.Contains(methods, MethodSecondFactor)
}

// PasskeyAMR is what a passkey's answer proved, as its authenticator's flags
// say and nothing more: a key that may be backed up and synced to other
// devices (BE) is a software key, one bound to its device a hardware key;
// and user when the authenticator verified the person. How it verified them
// (pin, fpt) WebAuthn does not say, so neither is claimed.
func PasskeyAMR(backupEligible, userVerified bool) []string {
	proved := []string{AMRHardwareKey}
	if backupEligible {
		proved[0] = AMRSoftwareKey
	}
	if userVerified {
		proved = append(proved, AMRUserPresence)
	}
	return proved
}

// WithSecondFactor is methods with what a second factor proved added, each
// once.
func WithSecondFactor(methods []string, proved ...string) StringList {
	out := slices.Clone(StringList(methods))
	for _, method := range append(slices.Clone(proved), MethodSecondFactor) {
		if method != "" && !slices.Contains(out, method) {
			out = append(out, method)
		}
	}
	return out
}
