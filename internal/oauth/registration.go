package oauth

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/crypto/jose"
	"github.com/a1i6ek/vard/internal/model"
)

// Dynamic client registration (RFC 7591). Registering is not open: it takes
// an initial access token, which is an admin API token with
// applications.write — the same an operator's tooling uses to manage
// applications — so whoever registers clients is an administrator's service.
// A registered client is an ordinary application, held to the same rules as
// one made in the panel.

// Registration errors (RFC 7591 §3.2.2).
const (
	ErrInvalidRedirectURI    = "invalid_redirect_uri"
	ErrInvalidClientMetadata = "invalid_client_metadata"
)

// ClientMetadata is what a client registers with (RFC 7591 §2, with the
// logout URIs of OpenID Connect's logout specifications).
type ClientMetadata struct {
	RedirectURIs            []string `json:"redirect_uris"`
	PostLogoutRedirectURIs  []string `json:"post_logout_redirect_uris,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	ApplicationType         string   `json:"application_type,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	ClientURI               string   `json:"client_uri,omitempty"`
	LogoURI                 string   `json:"logo_uri,omitempty"`
	TosURI                  string   `json:"tos_uri,omitempty"`
	PolicyURI               string   `json:"policy_uri,omitempty"`
	Scope                   string   `json:"scope,omitempty"`
	BackchannelLogoutURI    string   `json:"backchannel_logout_uri,omitempty"`

	// A private_key_jwt client's public keys: inline, or where it publishes
	// them (RFC 7591 §2).
	JWKS    *ClientJWKS `json:"jwks,omitempty"`
	JWKSURI string      `json:"jwks_uri,omitempty"`

	RequirePushedAuthorizationRequests bool `json:"require_pushed_authorization_requests,omitempty"`
}

// ClientJWKS is a JWK Set, as the jwks metadata carries it.
type ClientJWKS struct {
	Keys []jose.JWK `json:"keys"`
}

// RegisteredClient is the answer: the client's id, its secret when it has
// one (shown this once), and its metadata as the server now holds it.
type RegisteredClient struct {
	ClientID              string `json:"client_id"`
	ClientSecret          string `json:"client_secret,omitempty"`
	ClientIDIssuedAt      int64  `json:"client_id_issued_at"`
	ClientSecretExpiresAt int64  `json:"client_secret_expires_at"`
	ClientMetadata
}

// RegisterClient makes an application from a client's metadata. The token
// is the initial access token; who registered is in the activity log.
func (s *Service) RegisterClient(ctx context.Context, token string, m ClientMetadata) (*RegisteredClient, error) {
	caller, err := s.AdminCaller(ctx, token)
	if errors.Is(err, ErrTokenRefused) {
		return nil, authreq.OAuthError(authreq.ErrInvalidToken, "the initial access token is not valid")
	}
	if err != nil {
		return nil, err
	}
	if !slices.Contains(caller.Permissions, string(model.PermApplicationsWrite)) {
		return nil, authreq.OAuthError(authreq.ErrInsufficientScope, "the initial access token needs "+string(model.PermApplicationsWrite))
	}

	app, err := applicationFrom(m)
	if err != nil {
		return nil, err
	}

	if app.ClientID, err = model.NewClientID(); err != nil {
		return nil, err
	}

	var secret string
	if app.HasSecret() {
		if secret, err = app.IssueSecret(s.now(), 0); err != nil {
			return nil, err
		}
	}

	if err := s.store.CreateApplication(ctx, app); err != nil {
		return nil, err
	}

	entry := model.AuditLog{
		ActorEmail: caller.Application.Name,
		Action:     "application.registered",
		TargetType: "application",
		TargetID:   app.ID.String(),
		Metadata:   map[string]any{"by": caller.Application.ClientID},
	}
	if err := s.store.WriteAudit(ctx, &entry); err != nil {
		s.log.Error("writing the activity log failed", "error", err, "action", entry.Action)
	}

	return &RegisteredClient{
		ClientID:         app.ClientID,
		ClientSecret:     secret,
		ClientIDIssuedAt: app.CreatedAt.Unix(),
		ClientMetadata:   metadataOf(app),
	}, nil
}

// applicationFrom turns registered metadata into an application, refusing
// what the panel would refuse.
func applicationFrom(m ClientMetadata) (*model.Application, error) {
	grants := m.GrantTypes
	if len(grants) == 0 {
		grants = []string{model.GrantAuthorizationCode}
	}

	for _, kind := range m.ResponseTypes {
		if kind != "code" {
			return nil, authreq.OAuthError(ErrInvalidClientMetadata, "response_types: only code is offered")
		}
	}

	kind, err := profileOf(m, grants)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(m.ClientName)
	if name == "" {
		name = "Registered client"
	}

	app := &model.Application{
		Name:                   model.Truncate(name, 100),
		Profile:                kind,
		LogoURL:                m.LogoURI,
		WebsiteURL:             m.ClientURI,
		TermsURL:               m.TosURI,
		PrivacyURL:             m.PolicyURI,
		TokenAuthMethod:        model.AuthMethod(m.TokenEndpointAuthMethod),
		GrantTypes:             grants,
		RedirectURIs:           m.RedirectURIs,
		PostLogoutRedirectURIs: m.PostLogoutRedirectURIs,
		Scopes:                 strings.Fields(m.Scope),
		BackchannelLogoutURI:   m.BackchannelLogoutURI,
		JWKSURI:                m.JWKSURI,
		RequirePushedRequests:  m.RequirePushedAuthorizationRequests,
		AssertRoles:            true,
		IsEnabled:              true,
		AccessTokenLifetime:    model.DefaultAccessTokenLifetime,
		IDTokenLifetime:        model.DefaultIDTokenLifetime,
		RefreshTokenLifetime:   model.DefaultRefreshTokenLifetime,
	}
	if m.JWKS != nil {
		app.JWKS = m.JWKS.Keys
	}
	if len(app.Scopes) == 0 && kind != model.ProfileMachine {
		app.Scopes = []string{model.ScopeOpenID}
	}

	app.Normalize()
	if err := app.Validate(); err != nil {
		code := ErrInvalidClientMetadata
		if strings.Contains(err.Error(), "redirect_uris") {
			code = ErrInvalidRedirectURI
		}
		return nil, authreq.OAuthError(code, err.Error())
	}

	return app, nil
}

// profileOf is the profile a client's metadata describes (ADR 0005). The
// metadata has no word for it, so it is read from what the client says it
// does; a combination no profile allows is refused, never quietly changed
// into one that is.
func profileOf(m ClientMetadata, grants []string) (model.Profile, error) {
	refused := func(why string) (model.Profile, error) {
		return "", authreq.OAuthError(ErrInvalidClientMetadata, why)
	}
	method := model.AuthMethod(m.TokenEndpointAuthMethod)
	machine := len(grants) == 1 && grants[0] == model.GrantClientCredentials

	switch {
	case m.ApplicationType != "" && m.ApplicationType != "web" && m.ApplicationType != "native":
		return refused("application_type must be web or native")
	case machine && m.ApplicationType == "native":
		return refused("a native application signs users in; client_credentials alone is a service's")
	case machine && method != model.AuthPrivateKeyJWT:
		return refused("a service with no user authenticates with private_key_jwt and its keys")
	case machine:
		return model.ProfileMachine, nil
	case m.ApplicationType == "native" && method != "" && method != model.AuthNone:
		return refused("a native application cannot keep a credential: token_endpoint_auth_method must be none")
	case m.ApplicationType == "native" && desktopRedirects(m.RedirectURIs):
		return model.ProfileDesktop, nil
	case m.ApplicationType == "native":
		return model.ProfileNative, nil
	case method == model.AuthNone:
		return model.ProfileSPA, nil
	}
	return model.ProfileWeb, nil
}

// desktopRedirects is whether a native client is sent back to loopback,
// which is a desktop app's way (RFC 8252 §7.3), or has nowhere to be sent at
// all, signing in with the device grant.
func desktopRedirects(uris []string) bool {
	if len(uris) == 0 {
		return true
	}
	for _, uri := range uris {
		if strings.HasPrefix(uri, "http://") {
			return true
		}
	}
	return false
}

func metadataOf(app *model.Application) ClientMetadata {
	kind := "web"
	if app.Profile == model.ProfileNative || app.Profile == model.ProfileDesktop {
		kind = "native"
	}

	m := ClientMetadata{
		RedirectURIs:            app.RedirectURIs,
		PostLogoutRedirectURIs:  app.PostLogoutRedirectURIs,
		TokenEndpointAuthMethod: string(app.TokenAuthMethod),
		GrantTypes:              app.GrantTypes,
		ResponseTypes:           app.ResponseTypes(),
		ApplicationType:         kind,
		ClientName:              app.Name,
		ClientURI:               app.WebsiteURL,
		LogoURI:                 app.LogoURL,
		TosURI:                  app.TermsURL,
		PolicyURI:               app.PrivacyURL,
		Scope:                   strings.Join(app.Scopes, " "),
		BackchannelLogoutURI:    app.BackchannelLogoutURI,

		RequirePushedAuthorizationRequests: app.RequirePushedRequests,
	}
	if len(app.JWKS) > 0 {
		m.JWKS = &ClientJWKS{Keys: app.JWKS}
	}
	m.JWKSURI = app.JWKSURI

	return m
}
