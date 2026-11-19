package authreq

// The provider's paths, which applications and identity providers are given
// and which must never move, and the sign-in pages it sends a browser to.

// PathPushedAuthorization is where a client pushes its request (RFC 9126).
const PathPushedAuthorization = "/oauth2/par"

// PathRegistration is dynamic client registration (RFC 7591).
const PathRegistration = "/oauth2/register"

const (
	PathSocialStart    = "/oauth2/social/:slug/start"
	PathSocialCallback = "/oauth2/social/:slug/callback"
)

// The paths a single sign-on connection's identity provider is given.
const (
	PathSSOStart    = "/oauth2/sso/:slug/start"
	PathSSOCallback = "/oauth2/sso/:slug/callback"
	PathSSOACS      = "/oauth2/sso/:slug/acs"
	PathSSOMetadata = "/oauth2/sso/:slug/metadata"
)

// The device authorization grant (RFC 8628).
const (
	PathDeviceAuthorization = "/oauth2/device_authorization"
	// PageDevice is where the person enters the code.
	PageDevice = "/device"
)

// The Agreements and Consent steps' pages.
const (
	PageAgreements = "/agreements"
	PageConsent    = "/consent"
)

// PageSecondFactor is where a sign-in waiting for its second factor is
// finished.
const PageSecondFactor = "/second-factor"
