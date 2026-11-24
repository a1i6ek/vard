// Package passkey is the WebAuthn relying party both kinds of sign-in use:
// the sign-in pages' for users, the panel's for administrators. Each is its
// own site, so a passkey made on one is never offered on the other.
package passkey

import (
	"fmt"
	"net/url"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/a1i6ek/vard/internal/platform/brand"
)

// RelyingParty is the passkey relying party of the site at siteURL, where
// every ceremony runs: its passkeys are bound to rpID, the site's host or a
// domain above it (config.PasskeyRPID), or the host itself when "".
func RelyingParty(siteURL, rpID string) (*webauthn.WebAuthn, error) {
	parsed, err := url.Parse(siteURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("passkey: %q has no host for passkeys", siteURL)
	}
	if rpID == "" {
		rpID = parsed.Hostname()
	}

	return webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: brand.Name,
		RPOrigins:     []string{parsed.Scheme + "://" + parsed.Host},
	})
}
