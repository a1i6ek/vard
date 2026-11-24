package applications

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/a1i6ek/vard/internal/transport/http/respond"
	"github.com/a1i6ek/vard/internal/transport/http/validate"
	"github.com/a1i6ek/vard/internal/domain/model"
)

var (
	profileFixed           = respond.Define(http.StatusBadRequest, "application_profile_fixed", respond.Admin)
	noSecret               = respond.Define(http.StatusBadRequest, "application_no_secret", respond.Admin)
	foreignScope           = respond.Define(http.StatusBadRequest, "application_api_scope_foreign", respond.Admin)
	previewUserUnknown     = respond.Define(http.StatusBadRequest, "token_preview_user_unknown", respond.Admin)
	previewAudienceUnknown = respond.Define(http.StatusBadRequest, "token_preview_audience_unknown", respond.Admin)
	tooManyURIs            = respond.Define(http.StatusBadRequest, "application_too_many_uris", respond.Admin)
	profileInvalid         = respond.Define(http.StatusBadRequest, "application_profile_invalid", respond.Admin)
	overlapInvalid         = respond.Define(http.StatusBadRequest, "application_secret_overlap_invalid", respond.Admin)
	loginFlowUnknown       = respond.Define(http.StatusBadRequest, "application_login_flow_unknown", respond.Admin)
)

func (r *applicationRequest) applyTo(app *model.Application, creating bool) error {
	r.clean()

	if err := validate.Struct(r); err != nil {
		return err
	}

	if len(r.RedirectURIs) > maxURIs || len(r.PostLogoutRedirectURIs) > maxURIs {
		return tooManyURIs.With("max", maxURIs)
	}

	if creating && !model.Profile(r.Profile).Valid() {
		return profileInvalid.With()
	}

	app.Name = r.Name
	app.Description = r.Description
	app.LogoURL = r.LogoURL
	app.WebsiteURL = r.WebsiteURL
	app.PrivacyURL = r.PrivacyURL
	app.TermsURL = r.TermsURL
	app.TokenAuthMethod = model.AuthMethod(r.TokenAuthMethod)
	app.GrantTypes = r.GrantTypes
	app.RedirectURIs = r.RedirectURIs
	app.PostLogoutRedirectURIs = r.PostLogoutRedirectURIs
	app.AllowDevelopmentRedirects = validate.Flag(r.AllowDevelopmentRedirects, app.AllowDevelopmentRedirects)
	if r.IOSAppIDs != nil {
		app.IOSAppIDs = *r.IOSAppIDs
	}
	if r.AndroidApps != nil {
		app.AndroidApps = *r.AndroidApps
	}
	app.BackchannelLogoutURI = r.BackchannelLogoutURI
	app.JWKS = nil
	if r.JWKS != nil {
		app.JWKS = r.JWKS.Keys
	}
	app.JWKSURI = r.JWKSURI
	app.DPoPBoundAccessTokens = validate.Flag(r.DPoPBoundAccessTokens, app.DPoPBoundAccessTokens)
	if r.IDTokenSignedResponseAlg != "" {
		app.IDTokenSigningAlg = r.IDTokenSignedResponseAlg
	}
	app.Scopes = r.Scopes
	app.RequirePushedRequests = validate.Flag(r.RequirePushedRequests, app.RequirePushedRequests)
	app.AccessTokenLifetime = r.AccessTokenLifetime
	app.IDTokenLifetime = r.IDTokenLifetime
	app.RefreshTokenLifetime = r.RefreshTokenLifetime
	app.RefreshTokenIdleLifetime = r.RefreshTokenIdleLifetime
	app.AssertRoles = validate.Flag(r.AssertRoles, app.AssertRoles)
	app.RequireRoleAssignment = validate.Flag(r.RequireRoleAssignment, app.RequireRoleAssignment)
	app.IsEnabled = validate.Flag(r.IsEnabled, app.IsEnabled)
	app.AllowRegistration = validate.Flag(r.AllowRegistration, app.AllowRegistration)

	if r.LoginFlowID != nil {
		flow, err := flowID(*r.LoginFlowID)
		if err != nil {
			return err
		}

		app.LoginFlowID = flow
	}

	app.Normalize()

	if !app.HasSecret() {
		app.ClientSecretHash = ""
		app.SecretHint = ""
		app.SecretCreatedAt = nil
	}

	if err := app.Validate(); err != nil {
		return respond.Invalid(err.Error())
	}

	return nil
}

func flowID(sent string) (*uuid.UUID, error) {
	trimmed := strings.TrimSpace(sent)
	if trimmed == "" {
		return nil, nil
	}

	id, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, loginFlowUnknown.With()
	}

	return &id, nil
}
