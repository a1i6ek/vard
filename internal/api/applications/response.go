package applications

import (
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/a1i6ek/vard/internal/model"
	"github.com/a1i6ek/vard/internal/store"
)

// applicationResponse is an application as the panel sees it. The secret's
// hash is never part of it.
type applicationResponse struct {
	ID          uuid.UUID     `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Profile     model.Profile `json:"profile"`
	LogoURL     string        `json:"logo_url"`
	WebsiteURL  string        `json:"website_url"`
	PrivacyURL  string        `json:"privacy_url"`
	TermsURL    string        `json:"terms_url"`

	ClientID         string     `json:"client_id"`
	ClientIDIssuedAt int64      `json:"client_id_issued_at"`
	HasSecret        bool       `json:"has_secret"`
	SecretHint       string     `json:"secret_hint"`
	SecretCreatedAt  *time.Time `json:"secret_created_at"`
	// PreviousSecretUntil is when the secret the last rotation replaced
	// stops working; nil when it already has.
	PreviousSecretUntil *time.Time `json:"previous_secret_until"`

	TokenAuthMethod           model.AuthMethod `json:"token_auth_method"`
	GrantTypes                []string         `json:"grant_types"`
	ResponseTypes             []string         `json:"response_types"`
	RedirectURIs              []string         `json:"redirect_uris"`
	PostLogoutRedirectURIs    []string         `json:"post_logout_redirect_uris"`
	AllowDevelopmentRedirects bool             `json:"allow_development_redirects"`
	IOSAppIDs                 []string         `json:"ios_app_ids"`
	AndroidApps               []androidApp     `json:"android_apps"`
	BackchannelLogoutURI      string           `json:"backchannel_logout_uri"`
	JWKS                      *jwkSet          `json:"jwks"`
	JWKSURI                   string           `json:"jwks_uri"`
	IDTokenSignedResponseAlg  string           `json:"id_token_signed_response_alg"`
	DPoPBoundAccessTokens     bool             `json:"dpop_bound_access_tokens"`
	Scopes                    []string         `json:"scopes"`
	RequirePushedRequests     bool             `json:"require_pushed_authorization_requests"`

	AccessTokenLifetime      int `json:"access_token_lifetime"`
	IDTokenLifetime          int `json:"id_token_lifetime"`
	RefreshTokenLifetime     int `json:"refresh_token_lifetime"`
	RefreshTokenIdleLifetime int `json:"refresh_token_idle_lifetime"`

	AssertRoles           bool `json:"assert_roles"`
	RequireRoleAssignment bool `json:"require_role_assignment"`
	IsEnabled             bool `json:"is_enabled"`
	AllowRegistration     bool `json:"allow_registration"`

	// LoginFlowID is the application's flow, null for the default.
	LoginFlowID *uuid.UUID `json:"login_flow_id"`

	// Issuer is what the application is configured with to find this server
	// (its discovery document is below it); the same for every application.
	Issuer string `json:"issuer"`

	// RoleCount is how many roles the application defines.
	RoleCount int64     `json:"role_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newApplicationResponse(app model.Application, roles int64, issuer string) applicationResponse {
	return applicationResponse{
		Issuer:                    issuer,
		ID:                        app.ID,
		Name:                      app.Name,
		Description:               app.Description,
		Profile:                   app.Profile,
		LogoURL:                   app.LogoURL,
		WebsiteURL:                app.WebsiteURL,
		PrivacyURL:                app.PrivacyURL,
		TermsURL:                  app.TermsURL,
		ClientID:                  app.ClientID,
		ClientIDIssuedAt:          app.CreatedAt.Unix(),
		HasSecret:                 app.HasSecret() && app.ClientSecretHash != "",
		SecretHint:                app.SecretHint,
		SecretCreatedAt:           app.SecretCreatedAt,
		PreviousSecretUntil:       previousSecretUntil(app),
		TokenAuthMethod:           app.TokenAuthMethod,
		GrantTypes:                nonNil(app.GrantTypes),
		ResponseTypes:             app.ResponseTypes(),
		RedirectURIs:              nonNil(app.RedirectURIs),
		PostLogoutRedirectURIs:    nonNil(app.PostLogoutRedirectURIs),
		AllowDevelopmentRedirects: app.AllowDevelopmentRedirects,
		IOSAppIDs:                 nonNil(app.IOSAppIDs),
		AndroidApps:               androidAppsOf(app.AndroidApps),
		BackchannelLogoutURI:      app.BackchannelLogoutURI,
		JWKS:                      jwksOf(app),
		JWKSURI:                   app.JWKSURI,
		IDTokenSignedResponseAlg:  app.IDTokenSigningAlg,
		DPoPBoundAccessTokens:     app.DPoPBoundAccessTokens,
		Scopes:                    nonNil(app.Scopes),
		RequirePushedRequests:     app.RequirePushedRequests,
		AccessTokenLifetime:       app.AccessTokenLifetime,
		IDTokenLifetime:           app.IDTokenLifetime,
		RefreshTokenLifetime:      app.RefreshTokenLifetime,
		RefreshTokenIdleLifetime:  app.RefreshTokenIdleLifetime,
		AssertRoles:               app.AssertRoles,
		RequireRoleAssignment:     app.RequireRoleAssignment,
		IsEnabled:                 app.IsEnabled,
		AllowRegistration:         app.AllowRegistration,
		LoginFlowID:               app.LoginFlowID,
		RoleCount:                 roles,
		CreatedAt:                 app.CreatedAt,
		UpdatedAt:                 app.UpdatedAt,
	}
}

// secretResponse is an application with a just-issued secret, the only time it
// is shown.
type secretResponse struct {
	Application  applicationResponse `json:"application"`
	ClientSecret string              `json:"client_secret,omitempty"`
}

func newSecretResponse(app applicationResponse, secret string) secretResponse {
	return secretResponse{Application: app, ClientSecret: secret}
}

type pageResponse struct {
	Applications []applicationResponse `json:"applications"`
	Total        int64                 `json:"total"`
	Limit        int                   `json:"limit"`
	Offset       int                   `json:"offset"`
}

func newPageResponse(apps []model.Application, counts map[uuid.UUID]int64, total int64, query store.ApplicationQuery, issuer string) pageResponse {
	out := make([]applicationResponse, 0, len(apps))
	for _, app := range apps {
		out = append(out, newApplicationResponse(app, counts[app.ID], issuer))
	}

	return pageResponse{Applications: out, Total: total, Limit: query.Limit, Offset: query.Offset}
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// accessScope is one scope of an API, and whether the application may ask for
// it.
type accessScope struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Allowed     bool      `json:"allowed"`
}

// accessAPI is what an application may do with one API.
type accessAPI struct {
	ID           uuid.UUID     `json:"id"`
	Name         string        `json:"name"`
	Identifier   string        `json:"identifier"`
	EnforceRoles bool          `json:"enforce_roles"`
	Authorized   bool          `json:"authorized"`
	Scopes       []accessScope `json:"scopes"`
}

// accessResponse is every API, with what the application may do with each.
type accessResponse struct {
	APIs []accessAPI `json:"apis"`
}

func newAccessResponse(access []store.APIAccess) accessResponse {
	out := accessResponse{APIs: make([]accessAPI, 0, len(access))}

	for _, it := range access {
		api := accessAPI{
			ID:           it.API.ID,
			Name:         it.API.Name,
			Identifier:   it.API.Identifier,
			EnforceRoles: it.API.EnforceRoles,
			Authorized:   it.Authorized,
			Scopes:       make([]accessScope, 0, len(it.API.Scopes)),
		}

		for _, scope := range it.API.Scopes {
			api.Scopes = append(api.Scopes, accessScope{
				ID:          scope.ID,
				Name:        scope.Name,
				Description: scope.Description,
				Allowed:     slices.Contains(it.Allowed, scope.ID),
			})
		}

		out.APIs = append(out.APIs, api)
	}

	return out
}

// jwksOf is the application's inline keys as a JWK Set, nil without any.
func jwksOf(app model.Application) *jwkSet {
	if len(app.JWKS) == 0 {
		return nil
	}
	return &jwkSet{Keys: app.JWKS}
}

// previousSecretUntil is when the replaced secret stops working, while it
// still does.
func previousSecretUntil(app model.Application) *time.Time {
	if app.PreviousSecretHash == "" || app.PreviousSecretExpiresAt == nil || !time.Now().Before(*app.PreviousSecretExpiresAt) {
		return nil
	}
	return app.PreviousSecretExpiresAt
}

// logoutDelivery is one back-channel logout notice and how it went. The user
// is named by id only: who they are is for whoever may read users.
type logoutDelivery struct {
	ID          uuid.UUID          `json:"id"`
	SessionID   uuid.UUID          `json:"session_id"`
	UserID      uuid.UUID          `json:"user_id"`
	Status      model.LogoutStatus `json:"status"`
	Tries       int                `json:"tries"`
	LastError   string             `json:"last_error"`
	CreatedAt   time.Time          `json:"created_at"`
	NextTryAt   *time.Time         `json:"next_try_at"`
	DeliveredAt *time.Time         `json:"delivered_at"`
}

type logoutDeliveriesResponse struct {
	Deliveries []logoutDelivery `json:"deliveries"`
}

func newLogoutDeliveriesResponse(deliveries []model.LogoutDelivery) logoutDeliveriesResponse {
	out := make([]logoutDelivery, 0, len(deliveries))
	for _, d := range deliveries {
		item := logoutDelivery{
			ID: d.ID, SessionID: d.SessionID, UserID: d.UserID, Status: d.Status, Tries: d.Tries,
			LastError: d.LastError, CreatedAt: d.CreatedAt, DeliveredAt: d.DeliveredAt,
		}
		if d.Status == model.LogoutPending {
			next := d.NextTryAt
			item.NextTryAt = &next
		}
		out = append(out, item)
	}
	return logoutDeliveriesResponse{Deliveries: out}
}

// androidApp is one Android build of a mobile app.
type androidApp struct {
	PackageName  string   `json:"package_name"`
	Fingerprints []string `json:"sha256_cert_fingerprints"`
}

func androidAppsOf(apps model.AndroidApps) []androidApp {
	out := make([]androidApp, 0, len(apps))
	for _, app := range apps {
		out = append(out, androidApp{PackageName: app.PackageName, Fingerprints: nonNil(app.Fingerprints)})
	}
	return out
}
