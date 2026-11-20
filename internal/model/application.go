package model

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/a1i6ek/vard/internal/crypto/jose"
)

// Application is an OAuth client. It follows RFC 7591 metadata and RFC 9700:
// exact redirect URI matching, no implicit or password grants, and PKCE for
// clients that cannot keep a secret. Its roles mean something only within it.
type Application struct {
	Base

	Name        string `gorm:"size:100;not null" json:"name"`
	Description string `gorm:"size:255;not null;default:''" json:"description"`
	// Profile is the kind of client (ADR 0005), and decides what the
	// application may use: Normalize applies it and Validate refuses what it
	// forbids, on every save.
	Profile Profile `gorm:"type:varchar(16);not null;index;check:chk_applications_profile,profile IN ('web','spa','native','desktop','machine')" json:"profile"`

	// LogoURL and ClientURI are shown on the sign-in pages, and on the
	// consent page when the application's login flow has that step.
	LogoURL    string `gorm:"size:512;not null;default:''" json:"logo_url"`
	WebsiteURL string `gorm:"size:512;not null;default:''" json:"website_url"`

	PrivacyURL string `gorm:"size:512;not null;default:''" json:"privacy_url"`
	TermsURL   string `gorm:"size:512;not null;default:''" json:"terms_url"`

	AllowRegistration bool `gorm:"not null" json:"allow_registration"`

	ClientID string `gorm:"size:64;uniqueIndex;not null" json:"client_id"`

	ClientSecretHash string     `gorm:"size:64;not null;default:''" json:"-"`
	SecretHint       string     `gorm:"size:8;not null;default:''" json:"secret_hint"`
	SecretCreatedAt  *time.Time `json:"secret_created_at"`
	// PreviousSecretHash is the secret a rotation replaced, kept hashed for
	// the overlap its administrator chose, so the client can be given the
	// new one before the old stops working. Empty with none.
	PreviousSecretHash      string     `gorm:"size:64;not null;default:''" json:"-"`
	PreviousSecretExpiresAt *time.Time `json:"previous_secret_expires_at"`

	TokenAuthMethod AuthMethod `gorm:"type:varchar(32);not null;check:chk_applications_token_auth_method,token_auth_method IN ('client_secret_basic','client_secret_post','private_key_jwt','none')" json:"token_auth_method"`

	// JWKS or JWKSURI holds the public keys a private_key_jwt client signs
	// its assertions with (RFC 7523 §2.2): inline, or at an address of the
	// client's, fetched through the outbound client. Exactly one is set for
	// that method, neither for any other.
	JWKS    []jose.JWK `gorm:"type:jsonb;serializer:json" json:"jwks"`
	JWKSURI string     `gorm:"size:512;not null;default:''" json:"jwks_uri"`

	GrantTypes             []string `gorm:"type:jsonb;serializer:json" json:"grant_types"`
	RedirectURIs           []string `gorm:"type:jsonb;serializer:json" json:"redirect_uris"`
	PostLogoutRedirectURIs []string `gorm:"type:jsonb;serializer:json" json:"post_logout_redirect_uris"`
	// AllowDevelopmentRedirects lets a native app register Expo Go's exp://
	// addresses while it is developed. They name a developer's machine on
	// their network, so a released app turns it off.
	AllowDevelopmentRedirects bool     `gorm:"not null;default:false" json:"allow_development_redirects"`
	Scopes                    []string `gorm:"type:jsonb;serializer:json" json:"scopes"`

	// BackchannelLogoutURI is told, server to server, when a session the
	// application signed someone in through ends (OpenID Connect
	// Back-Channel Logout 1.0). Empty, it is not told.
	BackchannelLogoutURI string `gorm:"size:512;not null;default:''" json:"backchannel_logout_uri"`

	// RequirePushedRequests refuses an authorization request the application
	// did not push first (RFC 9126 §6), so its parameters never travel in a
	// browser's address.
	RequirePushedRequests bool `gorm:"not null" json:"require_pushed_authorization_requests"`

	AccessTokenLifetime  int `gorm:"not null" json:"access_token_lifetime"`
	IDTokenLifetime      int `gorm:"not null" json:"id_token_lifetime"`
	RefreshTokenLifetime int `gorm:"not null" json:"refresh_token_lifetime"`
	// RefreshTokenIdleLifetime ends a refresh family unused for that long,
	// within its absolute RefreshTokenLifetime.
	RefreshTokenIdleLifetime int `gorm:"not null" json:"refresh_token_idle_lifetime"`

	// IDTokenSigningAlg signs the application's ID tokens and logout tokens
	// (id_token_signed_response_alg): RS256, which every client accepts,
	// unless it asks for PS256 or ES256.
	// DPoPBoundAccessTokens refuses a token request without a DPoP proof,
	// so every token the application gets is bound to its key (RFC 9449
	// §5.2). Always on for an SPA and a mobile app (ADR 0005).
	DPoPBoundAccessTokens bool `gorm:"column:dpop_bound_access_tokens;not null;default:false" json:"dpop_bound_access_tokens"`

	IDTokenSigningAlg string `gorm:"column:id_token_signed_response_alg;type:varchar(8);not null;default:'RS256';check:chk_applications_id_token_signed_response_alg,id_token_signed_response_alg IN ('RS256','PS256','ES256')" json:"id_token_signed_response_alg"`

	AssertRoles           bool `gorm:"not null" json:"assert_roles"`
	RequireRoleAssignment bool `gorm:"not null;default:false" json:"require_role_assignment"`

	// A flow deleted leaves its applications on the default one.
	LoginFlowID *uuid.UUID `gorm:"type:uuid;index" json:"login_flow_id"`
	LoginFlow   *LoginFlow `gorm:"constraint:OnDelete:SET NULL" json:"-"`

	IsEnabled bool `gorm:"not null;index" json:"is_enabled"`
}

func (Application) TableName() string {
	return "applications"
}

// Profile is one of the five kinds of client the server serves (ADR 0005).
type Profile string

const (
	// ProfileWeb is a confidential server-side application (a BFF):
	// private_key_jwt, or a secret while it migrates.
	ProfileWeb Profile = "web"
	// ProfileSPA is a browser application with no backend: public.
	ProfileSPA Profile = "spa"
	// ProfileNative is a mobile app: public, redirecting to a claimed https
	// address or a private-use scheme (RFC 8252).
	ProfileNative Profile = "native"
	// ProfileDesktop is a desktop or command-line app, or a TV: public,
	// redirecting to a loopback address on any port, or using the device
	// grant.
	ProfileDesktop Profile = "desktop"
	// ProfileMachine is a service with no user: client_credentials, signed
	// by private_key_jwt alone.
	ProfileMachine Profile = "machine"
)

var Profiles = []Profile{ProfileWeb, ProfileSPA, ProfileNative, ProfileDesktop, ProfileMachine}

func (p Profile) Valid() bool {
	return slices.Contains(Profiles, p)
}

// Public is whether the profile is a client that cannot keep a credential.
func (p Profile) Public() bool {
	return p == ProfileSPA || p == ProfileNative || p == ProfileDesktop
}

type AuthMethod string

const (
	AuthClientSecretBasic AuthMethod = "client_secret_basic"
	AuthClientSecretPost  AuthMethod = "client_secret_post"
	// AuthPrivateKeyJWT is a signed assertion (RFC 7523 §2.2, OpenID
	// Connect Core §9): the client holds a private key, never a secret the
	// server also knows.
	AuthPrivateKeyJWT AuthMethod = "private_key_jwt"
	AuthNone          AuthMethod = "none"
)

var AuthMethods = []AuthMethod{AuthClientSecretBasic, AuthClientSecretPost, AuthPrivateKeyJWT, AuthNone}

// MaxClientKeys is how many keys a client may register inline.
const MaxClientKeys = 10

// The grants an application may use. The implicit and resource owner password
// grants are left out on purpose: RFC 9700 says not to use them.
const (
	GrantAuthorizationCode = "authorization_code"
	GrantRefreshToken      = "refresh_token"
	GrantClientCredentials = "client_credentials"
	// GrantDeviceCode is the device authorization grant (RFC 8628).
	GrantDeviceCode = "urn:ietf:params:oauth:grant-type:device_code"
)

var GrantTypes = []string{GrantAuthorizationCode, GrantRefreshToken, GrantClientCredentials, GrantDeviceCode}

const (
	ScopeOpenID        = "openid"
	ScopeProfile       = "profile"
	ScopeEmail         = "email"
	ScopeOfflineAccess = "offline_access"
	ScopeRoles         = "roles"
)

var Scopes = []string{ScopeOpenID, ScopeProfile, ScopeEmail, ScopeOfflineAccess, ScopeRoles}

const (
	// DefaultAccessTokenLifetime is short: an API checks an access token on
	// its own, so a revoked one is only refused by this server's endpoints
	// until it expires.
	DefaultAccessTokenLifetime  = 10 * 60
	DefaultIDTokenLifetime      = 60 * 60
	DefaultRefreshTokenLifetime = 30 * 24 * 60 * 60
	// DefaultRefreshTokenIdleLifetime is how long a refresh family may go
	// unused before it ends, whatever is left of its absolute lifetime.
	DefaultRefreshTokenIdleLifetime = 14 * 24 * 60 * 60
)

// MaxAccessTokenLifetime is the longest an access token lives, an
// application's or an API's.
const MaxAccessTokenLifetime = 60 * 60

const (
	minTokenLifetime        = 60
	maxAccessTokenLifetime  = MaxAccessTokenLifetime
	maxIDTokenLifetime      = 24 * 60 * 60
	maxRefreshTokenLifetime = 365 * 24 * 60 * 60
)

func (a Application) ResponseTypes() []string {
	if slices.Contains(a.GrantTypes, GrantAuthorizationCode) {
		return []string{"code"}
	}

	return []string{}
}

// SignsUsersIn says the application has a grant that gets tokens for a user:
// through a browser, or through a device the user allows elsewhere.
func (a Application) SignsUsersIn() bool {
	return slices.Contains(a.GrantTypes, GrantAuthorizationCode) || slices.Contains(a.GrantTypes, GrantDeviceCode)
}

// HasSecret says the application authenticates with a client secret.
func (a Application) HasSecret() bool {
	return a.TokenAuthMethod == AuthClientSecretBasic || a.TokenAuthMethod == AuthClientSecretPost
}

// Confidential says whether the application proves who it is at the token
// endpoint: a secret, a signed assertion or a certificate. A public one
// cannot, so whatever it is allowed is allowed to anyone holding its id.
func (a Application) Confidential() bool {
	return a.TokenAuthMethod != AuthNone
}

// AllowsRedirect is whether uri is one of the application's redirect URIs.
// Matching is exact, but a desktop app's loopback URI matches on any port: the
// app opens whichever port is free when it signs in (RFC 8252 §7.3). Only
// 127.0.0.1 and [::1] are loosened so; localhost matches exactly or not at
// all, since a resolver may point it elsewhere (RFC 8252 §8.3).
func (a Application) AllowsRedirect(uri string) bool {
	if slices.Contains(a.RedirectURIs, uri) {
		return true
	}
	if a.Profile != ProfileDesktop {
		return false
	}

	sent, err := url.Parse(uri)
	if err != nil || sent.Scheme != "http" || !loopbackAddress(sent.Hostname()) {
		return false
	}

	for _, registered := range a.RedirectURIs {
		want, err := url.Parse(registered)
		if err != nil || want.Scheme != "http" || !loopbackAddress(want.Hostname()) {
			continue
		}
		if want.Hostname() == sent.Hostname() && want.Path == sent.Path && want.RawQuery == sent.RawQuery && sent.User == nil && sent.Fragment == "" {
			return true
		}
	}

	return false
}

func (a *Application) Normalize() {
	switch {
	case a.Profile.Public():
		a.TokenAuthMethod = AuthNone
	case a.TokenAuthMethod == "" || a.TokenAuthMethod == AuthNone:
		a.TokenAuthMethod = AuthClientSecretBasic
	}

	if a.Profile != ProfileNative {
		a.AllowDevelopmentRedirects = false
	}

	if a.Profile == ProfileMachine {
		// A service signs its assertions: it keeps no secret (ADR 0005).
		a.TokenAuthMethod = AuthPrivateKeyJWT
		a.AllowRegistration = false
		a.GrantTypes = []string{GrantClientCredentials}
		a.RedirectURIs = []string{}
		a.PostLogoutRedirectURIs = []string{}
		a.BackchannelLogoutURI = ""
		a.Scopes = []string{}
		a.RequireRoleAssignment = false
		a.RequirePushedRequests = false
	}

	// Keys are a private_key_jwt client's alone, and a client that does not
	// authenticate with a secret keeps none: one left from before would be a
	// credential nobody uses.
	if !a.HasSecret() {
		a.ClientSecretHash, a.SecretHint, a.SecretCreatedAt = "", "", nil
		a.PreviousSecretHash, a.PreviousSecretExpiresAt = "", nil
	}
	if a.TokenAuthMethod != AuthPrivateKeyJWT {
		a.JWKS, a.JWKSURI = nil, ""
	}
	a.JWKSURI = strings.TrimSpace(a.JWKSURI)

	if a.IDTokenSigningAlg == "" {
		a.IDTokenSigningAlg = AlgRS256
	}
	// A browser or mobile app's tokens, and a machine's, are always bound to
	// a key (ADR 0005; mTLS, the other binding, is not offered: ADR 0006).
	if a.Profile == ProfileSPA || a.Profile == ProfileNative || a.Profile == ProfileMachine {
		a.DPoPBoundAccessTokens = true
	}

	a.GrantTypes = ordered(a.GrantTypes, GrantTypes)
	a.Scopes = ordered(a.Scopes, Scopes)
	a.RedirectURIs = tidyURIs(a.RedirectURIs)
	a.PostLogoutRedirectURIs = tidyURIs(a.PostLogoutRedirectURIs)
	a.BackchannelLogoutURI = strings.TrimSpace(a.BackchannelLogoutURI)
}

func (a Application) Validate() error {
	if !a.Profile.Valid() {
		return fmt.Errorf("profile must be one of: web, spa, native, desktop, machine")
	}

	if !slices.Contains(AuthMethods, a.TokenAuthMethod) {
		return fmt.Errorf("token_auth_method must be one of: client_secret_basic, client_secret_post, private_key_jwt, none")
	}

	if a.TokenAuthMethod == AuthPrivateKeyJWT {
		if err := a.checkKeys(); err != nil {
			return err
		}
	}

	if !slices.Contains(SigningAlgorithms, a.IDTokenSigningAlg) {
		return fmt.Errorf("id_token_signed_response_alg must be one of: %s", strings.Join(SigningAlgorithms, ", "))
	}

	for _, grant := range a.GrantTypes {
		if !slices.Contains(GrantTypes, grant) {
			return fmt.Errorf("grant_types: %q is not offered (implicit and password grants are not supported)", grant)
		}
	}

	for _, scope := range a.Scopes {
		if !slices.Contains(Scopes, scope) {
			return fmt.Errorf("scopes: %q is not a scope", scope)
		}
	}

	code := slices.Contains(a.GrantTypes, GrantAuthorizationCode)
	device := slices.Contains(a.GrantTypes, GrantDeviceCode)

	switch a.Profile {
	case ProfileMachine:
	case ProfileWeb, ProfileSPA, ProfileNative, ProfileDesktop:
		if !code && !device {
			return fmt.Errorf("grant_types: a %s application signs users in, so it needs authorization_code or the device code grant", a.Profile)
		}
		if a.Profile.Public() && slices.Contains(a.GrantTypes, GrantClientCredentials) {
			return fmt.Errorf("grant_types: a public client has no secret, so it cannot use client_credentials")
		}
	}

	if slices.Contains(a.GrantTypes, GrantRefreshToken) && !code && !device {
		return fmt.Errorf("grant_types: refresh_token needs authorization_code or the device code grant")
	}

	if code && len(a.RedirectURIs) == 0 {
		return fmt.Errorf("redirect_uris: authorization_code needs at least one redirect URI")
	}

	for _, uri := range a.RedirectURIs {
		if err := a.checkRedirectURI(uri); err != nil {
			return fmt.Errorf("redirect_uris: %w", err)
		}
	}

	for _, uri := range a.PostLogoutRedirectURIs {
		if err := a.checkRedirectURI(uri); err != nil {
			return fmt.Errorf("post_logout_redirect_uris: %w", err)
		}
	}

	if a.BackchannelLogoutURI != "" {
		if err := checkLogoutURI(a.BackchannelLogoutURI); err != nil {
			return fmt.Errorf("backchannel_logout_uri: %w", err)
		}
	}

	// Plain http is fine for links, but logo_url is loaded as an image on an
	// https page, where browsers block http.
	links := []struct{ name, value string }{
		{"website_url", a.WebsiteURL},
		{"privacy_url", a.PrivacyURL},
		{"terms_url", a.TermsURL},
	}
	for _, link := range links {
		if link.value == "" {
			continue
		}
		if parsed, err := url.Parse(link.value); err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			return fmt.Errorf("%s must be an http or https URL", link.name)
		}
	}

	if a.LogoURL != "" {
		if parsed, err := url.Parse(a.LogoURL); err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("logo_url must be an https URL")
		}
	}

	lifetimes := []struct {
		name  string
		value int
		max   int
	}{
		{"access_token_lifetime", a.AccessTokenLifetime, a.maxAccessTokenLifetime()},
		{"id_token_lifetime", a.IDTokenLifetime, maxIDTokenLifetime},
		{"refresh_token_lifetime", a.RefreshTokenLifetime, maxRefreshTokenLifetime},
		{"refresh_token_idle_lifetime", a.RefreshTokenIdleLifetime, a.RefreshTokenLifetime},
	}
	for _, lifetime := range lifetimes {
		if lifetime.value < minTokenLifetime || lifetime.value > lifetime.max {
			return fmt.Errorf("%s must be between %d and %d seconds", lifetime.name, minTokenLifetime, lifetime.max)
		}
	}

	return nil
}

// checkRedirectURI enforces RFC 9700 and the profile: absolute, no fragment or
// wildcard; https, or http on loopback, for a web app or an SPA; a claimed
// https address or a private-use scheme for a mobile app (RFC 8252), or Expo
// Go's while it is developed; and a loopback address or a private-use scheme
// for a desktop app, never localhost, which a resolver may point elsewhere
// (RFC 8252 §8.3).
func (a Application) checkRedirectURI(raw string) error {
	kind := a.Profile

	if strings.Contains(raw, "*") {
		return fmt.Errorf("%q: wildcards are not allowed, redirect URIs are matched exactly", raw)
	}

	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() {
		return fmt.Errorf("%q is not an absolute URI", raw)
	}

	if parsed.Fragment != "" || strings.Contains(raw, "#") {
		return fmt.Errorf("%q must not have a fragment", raw)
	}

	switch parsed.Scheme {
	case "https":
		if parsed.Host == "" {
			return fmt.Errorf("%q has no host", raw)
		}
		if kind == ProfileDesktop {
			return fmt.Errorf("%q: a desktop app is sent back to a loopback address, such as http://127.0.0.1/callback, or a private-use scheme", raw)
		}
		return nil
	case "http":
		switch {
		case kind == ProfileDesktop && loopbackAddress(parsed.Hostname()):
			return nil
		case kind == ProfileDesktop && loopback(parsed.Hostname()):
			return fmt.Errorf("%q: use 127.0.0.1 or [::1], not localhost, which a resolver may point elsewhere", raw)
		case kind == ProfileNative && loopback(parsed.Hostname()):
			return fmt.Errorf("%q: a mobile app is sent back to https or a private-use scheme; a loopback address is a desktop app's", raw)
		case loopback(parsed.Hostname()):
			return nil
		}
		return fmt.Errorf("%q must use https, unless it is on localhost", raw)
	case "exp", "exps":
		// Expo Go, while a native app is developed: the scheme is Expo Go's
		// own, and PKCE, which a native app always uses, keeps a code that
		// reaches another app useless to it. The address is a developer's
		// machine, so only an app marked as in development may have one.
		switch {
		case kind != ProfileNative:
			return fmt.Errorf("%q must use https", raw)
		case !a.AllowDevelopmentRedirects:
			return fmt.Errorf("%q is Expo Go's, for development: allow development redirects on this application to register it, and turn them off before it is released", raw)
		}
		return nil
	default:
		// A private-use scheme is a reverse domain name, so it has a dot in
		// it and cannot be mistaken for javascript: or data:.
		if (kind == ProfileNative || kind == ProfileDesktop) && strings.Contains(parsed.Scheme, ".") {
			return nil
		}
		return fmt.Errorf("%q must use https", raw)
	}
}

// checkLogoutURI holds an address an application is told of a logout at:
// absolute, https except on loopback, and without a fragment (Back-Channel
// Logout 1.0 §2.2). A query is allowed.
func checkLogoutURI(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("%q is not an absolute URL", raw)
	}

	if parsed.Fragment != "" || strings.Contains(raw, "#") {
		return fmt.Errorf("%q must not have a fragment", raw)
	}

	switch {
	case parsed.Scheme == "https":
		return nil
	case parsed.Scheme == "http" && loopback(parsed.Hostname()):
		return nil
	default:
		return fmt.Errorf("%q must use https, unless it is on localhost", raw)
	}
}

func loopback(host string) bool {
	return host == "localhost" || loopbackAddress(host)
}

// loopbackAddress is a loopback address written out, which no resolver can
// point anywhere else.
func loopbackAddress(host string) bool {
	return host == "127.0.0.1" || host == "::1"
}

func NewClientID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate client id: %w", err)
	}

	return hex.EncodeToString(b[:]), nil
}

var ErrNoSecret = errors.New("a public client has no secret")

// MaxSecretOverlap is the longest a rotated secret keeps working beside its
// successor.
const MaxSecretOverlap = 7 * 24 * time.Hour

// IssueSecret makes a new secret. With an overlap, the one it replaces keeps
// working for that long, so the client can be given the new one first; with
// none it stops at once, and any older one kept from before goes too.
func (a *Application) IssueSecret(now time.Time, overlap time.Duration) (string, error) {
	if !a.HasSecret() {
		return "", ErrNoSecret
	}
	overlap = min(overlap, MaxSecretOverlap)

	if overlap > 0 && a.ClientSecretHash != "" {
		until := now.Add(overlap)
		a.PreviousSecretHash, a.PreviousSecretExpiresAt = a.ClientSecretHash, &until
	} else {
		a.PreviousSecretHash, a.PreviousSecretExpiresAt = "", nil
	}

	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate client secret: %w", err)
	}

	secret := base64.RawURLEncoding.EncodeToString(b[:])
	sum := sha256.Sum256([]byte(secret))

	a.ClientSecretHash = hex.EncodeToString(sum[:])
	a.SecretHint = secret[len(secret)-4:]
	a.SecretCreatedAt = &now

	return secret, nil
}

// CheckSecret reports whether a presented secret is the application's: its
// current one, or the one it replaced while that overlap lasts. Both are
// compared, in constant time.
func (a Application) CheckSecret(secret string, now time.Time) bool {
	sum := sha256.Sum256([]byte(secret))
	presented := []byte(hex.EncodeToString(sum[:]))

	current := a.ClientSecretHash != "" && subtle.ConstantTimeCompare(presented, []byte(a.ClientSecretHash)) == 1
	previous := a.PreviousSecretHash != "" && a.PreviousSecretExpiresAt != nil && now.Before(*a.PreviousSecretExpiresAt) &&
		subtle.ConstantTimeCompare(presented, []byte(a.PreviousSecretHash)) == 1

	return current || previous
}

// MaxMachineTokenLifetime is the longest a machine client's access token
// lives: it asks again rather than refresh, and a service's token is the one
// most likely to be copied into a log.
const MaxMachineTokenLifetime = 15 * 60

// AccessLifetimeFor is how long an access token for the API lives, in
// seconds: the API's own lifetime when it sets one, the application's
// otherwise, and never past a machine's limit either way.
func (a Application) AccessLifetimeFor(api *API) int {
	lifetime := a.AccessTokenLifetime
	if api != nil && api.TokenLifetime > 0 {
		lifetime = api.TokenLifetime
	}
	return min(lifetime, a.maxAccessTokenLifetime())
}

func (a Application) maxAccessTokenLifetime() int {
	if a.Profile == ProfileMachine {
		return MaxMachineTokenLifetime
	}
	return maxAccessTokenLifetime
}

// checkKeys holds a private_key_jwt client to one source of keys, each a
// signing key this server can verify with, within the RSA bounds.
func (a Application) checkKeys() error {
	switch {
	case len(a.JWKS) == 0 && a.JWKSURI == "":
		return fmt.Errorf("private_key_jwt needs the client's public keys: jwks, or jwks_uri")
	case len(a.JWKS) > 0 && a.JWKSURI != "":
		return fmt.Errorf("give jwks or jwks_uri, not both")
	case len(a.JWKS) > MaxClientKeys:
		return fmt.Errorf("jwks: at most %d keys", MaxClientKeys)
	}

	for i, jwk := range a.JWKS {
		if jwk.Use != "" && jwk.Use != "sig" {
			return fmt.Errorf("jwks: key %d is not a signing key (use %q)", i+1, jwk.Use)
		}
		if jwk.Algorithm != "" && !slices.Contains(jose.Algorithms, jwk.Algorithm) {
			return fmt.Errorf("jwks: key %d is for %s; ES256, PS256 and RS256 are accepted", i+1, jwk.Algorithm)
		}
		if _, err := jose.ParseJWK(jwk); err != nil {
			return fmt.Errorf("jwks: key %d: %w", i+1, err)
		}
	}

	if a.JWKSURI != "" {
		parsed, err := url.Parse(a.JWKSURI)
		if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.Fragment != "" {
			return fmt.Errorf("jwks_uri must be an absolute URL")
		}
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback(parsed.Hostname())) {
			return fmt.Errorf("jwks_uri must use https, unless it is on localhost")
		}
	}

	return nil
}

func ordered(values, catalog []string) []string {
	out := []string{}
	for _, known := range catalog {
		if slices.Contains(values, known) {
			out = append(out, known)
		}
	}

	for _, value := range values {
		if !slices.Contains(out, value) {
			out = append(out, value)
		}
	}

	return out
}

func tidyURIs(uris []string) []string {
	out := []string{}
	for _, uri := range uris {
		uri = strings.TrimSpace(uri)
		if uri != "" && !slices.Contains(out, uri) {
			out = append(out, uri)
		}
	}

	return out
}
