package authreq

import (
	"context"
	"net/url"

	"github.com/a1i6ek/vard/internal/domain/model"
)

const (
	PathDiscovery  = "/.well-known/openid-configuration"
	PathJWKS       = "/.well-known/jwks.json"
	PathAuthorize  = "/oauth2/authorize"
	PathToken      = "/oauth2/token"
	PathUserInfo   = "/oauth2/userinfo"
	PathLogout     = "/oauth2/logout"
	PathRevoke     = "/oauth2/revoke"
	PathIntrospect = "/oauth2/introspect"
)

const (
	PageLogin     = "/login"
	PageReset     = "/reset-password"
	PageVerify    = "/verify-email"
	PageError     = "/error"
	PageLoggedOut = "/logged-out"
	PageLogout    = "/logout"
)

// Issuer is the server's issuer, the origin clients address it at.
func (s *Service) Issuer() string {
	return s.issuer
}

type Client struct {
	IP        string
	UserAgent string
	Language  string
	// KnownBrowser is the browser's known-browser cookie, "" for none
	// (model.KnownBrowser).
	KnownBrowser string
}

// WithQuery adds the non-empty values to an address's query. Whatever query
// the address already has is kept byte for byte, in its order: a redirect
// URI is registered exactly, and a client may compare what comes back to it
// the same way (OAuth 2.1 §2.3.1, RFC 6749 §3.1.2).
func WithQuery(base string, values url.Values) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}

	added := url.Values{}
	for key, vals := range values {
		for _, v := range vals {
			if v != "" {
				added.Add(key, v)
			}
		}
	}

	switch encoded := added.Encode(); {
	case encoded == "":
	case u.RawQuery == "":
		u.RawQuery = encoded
	default:
		u.RawQuery += "&" + encoded
	}

	return u.String()
}

// ErrorPage is for authorization errors that cannot be redirected (unknown
// client, unregistered redirect URI), since redirecting would hand the user to
// whoever wrote the link.
func (s *Service) ErrorPage(code, description string) string {
	return WithQuery(s.accountURL+PageError, url.Values{"error": {code}, "error_description": {description}})
}

func (s *Service) Record(ctx context.Context, user *model.User, email, action string, client Client, metadata map[string]any) {
	entry := model.AuditLog{
		ActorEmail: email,
		Action:     action,
		TargetType: "user",
		IP:         client.IP,
		UserAgent:  client.UserAgent,
		Metadata:   metadata,
	}
	if user != nil {
		entry.TargetID = user.ID.String()
	}

	if err := s.store.WriteAudit(ctx, &entry); err != nil {
		s.log.Error("writing the activity log failed", "error", err, "action", action)
	}
}
