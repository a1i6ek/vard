// Package api builds the two HTTP servers and the table of which handler
// answers which path.
//
// The public server is the OAuth 2.0 / OpenID Connect provider and the account
// API, meant to face the internet. The admin server is the admin API the
// console calls, meant to be reachable only where administrators work. A path
// exists on only one of them.
//
// Handlers live one package per subject, each with handler.go, request.go,
// response.go and validation.go.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/a1i6ek/vard/internal/admin"
	"github.com/a1i6ek/vard/internal/api/account"
	"github.com/a1i6ek/vard/internal/api/activity"
	"github.com/a1i6ek/vard/internal/api/adminauth"
	"github.com/a1i6ek/vard/internal/api/adminroles"
	"github.com/a1i6ek/vard/internal/api/admins"
	"github.com/a1i6ek/vard/internal/api/apis"
	"github.com/a1i6ek/vard/internal/api/applications"
	"github.com/a1i6ek/vard/internal/api/audit"
	"github.com/a1i6ek/vard/internal/api/cors"
	"github.com/a1i6ek/vard/internal/api/csrf"
	"github.com/a1i6ek/vard/internal/api/fields"
	"github.com/a1i6ek/vard/internal/api/flows"
	"github.com/a1i6ek/vard/internal/api/keys"
	"github.com/a1i6ek/vard/internal/api/languages"
	"github.com/a1i6ek/vard/internal/api/mailsettings"
	"github.com/a1i6ek/vard/internal/api/mfa"
	"github.com/a1i6ek/vard/internal/api/middleware"
	"github.com/a1i6ek/vard/internal/api/oauth"
	"github.com/a1i6ek/vard/internal/api/organization"
	"github.com/a1i6ek/vard/internal/api/otp"
	"github.com/a1i6ek/vard/internal/api/profile"
	"github.com/a1i6ek/vard/internal/api/ratelimit"
	"github.com/a1i6ek/vard/internal/api/respond"
	"github.com/a1i6ek/vard/internal/api/roles"
	"github.com/a1i6ek/vard/internal/api/services"
	"github.com/a1i6ek/vard/internal/api/session"
	"github.com/a1i6ek/vard/internal/api/setup"
	"github.com/a1i6ek/vard/internal/api/social"
	"github.com/a1i6ek/vard/internal/api/sso"
	"github.com/a1i6ek/vard/internal/api/steps"
	"github.com/a1i6ek/vard/internal/api/twostep"
	"github.com/a1i6ek/vard/internal/api/users"
	"github.com/a1i6ek/vard/internal/api/usersessions"
	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/brand"
	"github.com/a1i6ek/vard/internal/cache"
	"github.com/a1i6ek/vard/internal/config"
	"github.com/a1i6ek/vard/internal/crypto/jose"
	"github.com/a1i6ek/vard/internal/model"
	"github.com/a1i6ek/vard/internal/oidc"
	"github.com/a1i6ek/vard/internal/passkey"
	"github.com/a1i6ek/vard/internal/store"
)

// tokenLimitMultiple loosens the provider's endpoints relative to the sign-in
// pages: one application backend may refresh tokens for a whole company from
// one address.
const tokenLimitMultiple = 10

func NewPublic(cfg config.Config, log *slog.Logger, provider *services.Provider, shared *cache.Redis) (*gin.Engine, error) {
	r, err := engine(cfg, log, cfg.Issuer)
	if err != nil {
		return nil, err
	}

	r.Use(session.CookieMode(cfg.SecureUserCookies))

	registerPublicRoutes(r, publicHandlers{
		oauth:   oauth.New(oauth.Deps{Requests: provider.Requests, SignIn: provider.SignIn, OAuth: provider.OAuth, OIDC: provider.OIDC, Federation: provider.Federation, Signing: provider.Signing, Log: log, SecureCookies: cfg.SecureUserCookies}),
		account: account.New(account.Deps{Requests: provider.Requests, SignIn: provider.SignIn, OAuth: provider.OAuth, OIDC: provider.OIDC, Federation: provider.Federation, Log: log, SecureCookies: cfg.SecureUserCookies}),
		twoStep: twostep.New(twostep.Deps{SignIn: provider.SignIn, Log: log}),
		profile: profile.New(profile.Deps{SignIn: provider.SignIn, Log: log}),
		steps:   steps.New(steps.Deps{OAuth: provider.OAuth, Log: log}),
		limit:   ratelimit.New(cfg.RateLimit).Shared(shared.SessionDB(), "public").Middleware(),
		tokens:  ratelimit.New(cfg.RateLimit*tokenLimitMultiple).Shared(shared.SessionDB(), "token").Middleware(),
		csrf:    csrf.New([]string{config.Origin(cfg.AccountURL)}),
		ready:   ready(provider.OIDC),
	})

	return r, nil
}

func NewAdmin(cfg config.Config, st *store.Store, sealer *jose.Sealer, log *slog.Logger, provider *services.Provider, shared *cache.Redis) (*gin.Engine, error) {
	r, err := engine(cfg, log, cfg.AdminURL)
	if err != nil {
		return nil, err
	}

	passkeys, err := passkey.RelyingParty(cfg.AdminURL)
	if err != nil {
		return nil, err
	}

	service := auth.New(st, sealer, log, adminIssuer(cfg.AdminURL), passkeys)
	service.UseBreachCheck(provider.SignIn.PasswordBreached)
	recorder := audit.New(st, log)

	r.Use(session.CookieMode(cfg.SecureAdminCookies))

	registerAdminRoutes(r, service, provider, log, adminHandlers{
		auth:         adminauth.New(adminauth.Deps{Admin: service, Store: st, Audit: recorder, Log: log, SecureCookies: cfg.SecureAdminCookies}),
		mfa:          mfa.New(mfa.Deps{Admin: service, Log: log, SecureCookies: cfg.SecureAdminCookies}),
		setup:        setup.New(setup.Deps{Store: st, Admin: service, Log: log}),
		users:        users.New(users.Deps{Store: st, Audit: recorder, Log: log, Breached: provider.SignIn.PasswordBreached}),
		userSessions: usersessions.New(usersessions.Deps{Store: st, Audit: recorder, Log: log}),
		fields:       fields.New(fields.Deps{Store: st, Audit: recorder, Log: log}),
		roles:        roles.New(roles.Deps{Store: st, Audit: recorder, Log: log}),
		admins:       admins.New(admins.Deps{Store: st, Admin: service, Audit: recorder, Log: log}),
		adminRoles:   adminroles.New(adminroles.Deps{Store: st, Audit: recorder, Log: log}),
		applications: applications.New(applications.Deps{Store: st, Audit: recorder, Log: log, Issuer: cfg.Issuer}),
		organization: organization.New(organization.Deps{Store: st, Audit: recorder, Log: log}),
		social:       social.New(social.Deps{Store: st, Sealer: sealer, Audit: recorder, Log: log, Issuer: cfg.Issuer}),
		sso:          sso.New(sso.Deps{Store: st, Sealer: sealer, Federation: provider.Federation, Audit: recorder, Log: log, Issuer: cfg.Issuer}),
		flows:        flows.New(flows.Deps{Store: st, Audit: recorder, Log: log}),
		languages:    languages.New(languages.Deps{Store: st, Audit: recorder, Log: log}),
		mail:         mailsettings.New(mailsettings.Deps{Store: st, Sealer: sealer, Audit: recorder, Log: log}),
		otp:          otp.New(otp.Deps{Store: st, Audit: recorder, Log: log}),
		apis:         apis.New(apis.Deps{Store: st, Audit: recorder, Log: log, Issuer: cfg.Issuer}),
		activity:     activity.New(activity.Deps{Store: st, Log: log}),
		keys:         keys.New(keys.Deps{Signing: provider.Signing, Audit: recorder, Log: log}),
		limit:        ratelimit.New(cfg.RateLimit).Shared(shared.SessionDB(), "admin").Middleware(),
		csrf:         csrf.New([]string{config.Origin(cfg.AdminURL)}),
	})

	return r, nil
}

func adminIssuer(adminURL string) string {
	if host := strings.TrimPrefix(strings.TrimPrefix(config.Origin(adminURL), "https://"), "http://"); host != "" {
		return brand.Name + " (" + host + ")"
	}
	return brand.Name
}

// engine is a listener's router; site is where browsers reach it, which says
// whether it is served over https.
func engine(cfg config.Config, log *slog.Logger, site string) (*gin.Engine, error) {
	r := gin.New()

	// Gin believes every X-Forwarded-For header unless told otherwise, which
	// would let any caller choose the address the log records for it.
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("trusted proxies: %w", err)
	}

	r.Use(middleware.Chain(log, cors.New(), strings.HasPrefix(site, "https://"))...)

	return r, nil
}

type publicHandlers struct {
	oauth   *oauth.Handler
	account *account.Handler
	twoStep *twostep.Handler
	profile *profile.Handler
	steps   *steps.Handler
	limit   gin.HandlerFunc
	tokens  gin.HandlerFunc
	csrf    gin.HandlerFunc
	ready   gin.HandlerFunc
}

type adminHandlers struct {
	auth         *adminauth.Handler
	mfa          *mfa.Handler
	setup        *setup.Handler
	users        *users.Handler
	userSessions *usersessions.Handler
	fields       *fields.Handler
	roles        *roles.Handler
	admins       *admins.Handler
	adminRoles   *adminroles.Handler
	applications *applications.Handler
	apis         *apis.Handler
	organization *organization.Handler
	social       *social.Handler
	sso          *sso.Handler
	flows        *flows.Handler
	languages    *languages.Handler
	mail         *mailsettings.Handler
	otp          *otp.Handler
	activity     *activity.Handler
	keys         *keys.Handler
	limit        gin.HandlerFunc
	csrf         gin.HandlerFunc
}

// registerPublicRoutes mounts every public route. What a path does is in the
// handler, but that a path exists — and on which server — is only ever here.
func registerPublicRoutes(r *gin.Engine, h publicHandlers) {
	r.GET("/healthz", health)
	r.GET("/readyz", h.ready)

	r.GET(authreq.PathDiscovery, h.oauth.Discovery)
	r.GET(authreq.PathJWKS, h.oauth.JWKS)
	// Authorize writes a row per request, so it shares the token endpoints'
	// loose per-address limit.
	r.GET(authreq.PathAuthorize, h.tokens, h.oauth.Authorize)
	r.POST(authreq.PathAuthorize, h.tokens, h.oauth.Authorize)
	r.POST(authreq.PathToken, h.tokens, h.oauth.Token)
	r.POST(authreq.PathPushedAuthorization, h.tokens, h.oauth.PushAuthorization)
	r.POST(authreq.PathDeviceAuthorization, h.tokens, h.oauth.StartDevice)
	r.POST(authreq.PathRegistration, h.tokens, h.oauth.RegisterClient)
	r.GET(authreq.PathUserInfo, h.oauth.UserInfo)
	r.POST(authreq.PathUserInfo, h.oauth.UserInfo)
	r.GET(authreq.PathLogout, h.oauth.Logout)
	r.POST(authreq.PathLogout, h.oauth.Logout)
	r.POST(authreq.PathRevoke, h.tokens, h.oauth.Revoke)
	r.POST(authreq.PathIntrospect, h.tokens, h.oauth.Introspect)

	// Social sign-in. The callback also answers POST and sits outside CSRF
	// because Apple posts its answer from its own form.
	r.GET(authreq.PathSocialStart, h.limit, h.oauth.SocialStart)
	r.GET(authreq.PathSocialCallback, h.limit, h.oauth.SocialCallback)
	r.POST(authreq.PathSocialCallback, h.limit, h.oauth.SocialCallback)

	// Enterprise SSO. The SAML ACS takes the provider's posted form, so it is
	// outside CSRF, and the browser then comes back to it with a GET; start
	// and ACS are rate limited because anyone can call them.
	r.GET(authreq.PathSSOStart, h.limit, h.oauth.SSOStart)
	r.GET(authreq.PathSSOCallback, h.limit, h.oauth.SSOCallback)
	r.POST(authreq.PathSSOACS, h.limit, h.oauth.SSOAssertion)
	r.GET(authreq.PathSSOACS, h.limit, h.oauth.SSOAssertionReturn)
	r.GET(authreq.PathSSOMetadata, h.oauth.SSOMetadata)

	// The organisation's logo, for the sign-in pages: anyone may see it, and
	// the path is the provider's, which puts it in their addresses.
	r.GET(authreq.PathOrganizationLogo, h.account.OrganizationLogo)

	// Everything under /api/v1 is called by the id app with the user's
	// session cookie, so it only takes changes from the id app's origin.
	v1 := r.Group("/api/v1", h.csrf)
	{
		accounts := v1.Group("/account")
		accounts.GET("/organization", h.account.Organization)
		accounts.GET("/social-providers", h.account.SocialProviders)
		accounts.GET("/sso", h.account.SSOButtons)
		accounts.POST("/sso/discover", h.limit, h.account.DiscoverSSO)
		accounts.GET("/requests/:handle", h.account.Request)
		accounts.GET("/login-options", h.account.LoginOptions)
		accounts.GET("/languages", h.account.Languages)
		accounts.GET("/languages/:code", h.account.LanguageText)
		accounts.GET("/applications/:client_id", h.account.Application)
		accounts.POST("/login", h.limit, h.account.Login)
		accounts.POST("/login/code", h.limit, h.account.Code)
		accounts.POST("/login/code/resend", h.limit, h.account.ResendCode)
		accounts.GET("/login/second-factor", h.account.SecondFactor)
		accounts.POST("/login/second-factor", h.limit, h.account.SubmitSecondFactor)
		accounts.POST("/login/second-factor/authenticator", h.limit, h.account.BeginEnrolAuthenticator)
		accounts.POST("/login/second-factor/authenticator/confirm", h.limit, h.account.EnrolAuthenticator)
		accounts.POST("/login/second-factor/passkeys", h.limit, h.account.BeginEnrolPasskey)
		accounts.POST("/login/second-factor/passkeys/finish", h.limit, h.account.EnrolPasskey)
		accounts.POST("/login/second-factor/leave", h.limit, h.account.LeaveSecondFactor)
		accounts.POST("/login/passkey", h.limit, h.account.BeginPasskey)
		accounts.POST("/login/passkey/finish", h.limit, h.account.FinishPasskey)
		accounts.POST("/register", h.limit, h.account.Register)
		accounts.POST("/forgot-password", h.limit, h.account.ForgotPassword)
		accounts.GET("/reset-password", h.account.CheckReset)
		accounts.POST("/reset-password", h.limit, h.account.ResetPassword)
		accounts.POST("/verify-email", h.limit, h.account.VerifyEmail)
		accounts.POST("/logout", h.account.Logout)

		own := accounts.Group("", h.account.RequireSession)
		own.GET("/me", h.profile.Me)
		own.PATCH("/me", h.profile.UpdateMe)
		own.POST("/password", h.limit, h.profile.ChangePassword)
		own.POST("/email", h.limit, h.profile.ChangeEmail)
		own.GET("/sessions", h.profile.Sessions)
		own.DELETE("/sessions/:id", h.profile.EndSession)
		own.GET("/connected-applications", h.profile.Applications)
		own.DELETE("/connected-applications/:client_id", h.profile.Disconnect)
		own.GET("/consent", h.steps.Consent)
		own.POST("/consent", h.steps.AnswerConsent)
		own.POST("/agreements", h.steps.AnswerAgreements)
		own.GET("/device", h.limit, h.steps.Device)
		own.POST("/device", h.limit, h.steps.AnswerDevice)
		own.POST("/step-up", h.limit, h.account.StepUp)
		own.GET("/two-step", h.twoStep.TwoStep)
		own.POST("/two-step/authenticator", h.limit, h.twoStep.BeginAuthenticator)
		own.POST("/two-step/authenticator/confirm", h.limit, h.twoStep.ConfirmAuthenticator)
		own.POST("/two-step/passkeys", h.limit, h.twoStep.BeginPasskey)
		own.POST("/two-step/passkeys/finish", h.twoStep.FinishPasskey)
		own.DELETE("/two-step/factors/:id", h.limit, h.twoStep.RemoveFactor)
		own.POST("/two-step/recovery-codes", h.limit, h.twoStep.NewRecoveryCodes)
	}

	r.NoRoute(notFound)
}

func registerAdminRoutes(r *gin.Engine, service *auth.Service, provider *services.Provider, log *slog.Logger, h adminHandlers) {
	r.GET("/healthz", health)
	r.GET("/readyz", ready(provider.OIDC))

	// Everything here is called by the console with the administrator's session
	// cookie, so it only takes changes from the console's origin.
	v1 := r.Group("/api/v1", h.csrf)
	{
		// Setup and sign-in cannot require a session. Setup is refused once any
		// administrator exists.
		v1.GET("/admin/setup", h.setup.Status)
		v1.POST("/admin/setup", h.limit, h.setup.Create)
		v1.POST("/admin/auth/login", h.limit, h.auth.Login)

		v1.GET("/admin/auth/session", h.auth.State)
		v1.POST("/admin/auth/mfa", h.limit, h.auth.VerifyMFA)
		v1.POST("/admin/auth/mfa/passkey", h.limit, h.auth.PasskeyChallenge)
		v1.POST("/admin/auth/logout", h.auth.Logout)

		setup := v1.Group("/admin/mfa", session.RequireSetup(service))
		setup.GET("", h.mfa.Status)
		setup.POST("/totp", h.limit, h.mfa.Begin)
		setup.POST("/totp/confirm", h.limit, h.mfa.Confirm)

		signedIn := v1.Group("/admin", session.Require(service))
		signedIn.GET("/me", h.auth.Me)
		// Any administrator: the header shows the organisation's logo.
		signedIn.GET("/organization/logo", h.organization.Logo)
		signedIn.PATCH("/me", h.limit, h.auth.UpdateMe)
		signedIn.POST("/me/password", h.limit, h.auth.ChangePassword)

		signedIn.DELETE("/mfa/totp", h.limit, h.mfa.Disable)
		signedIn.POST("/mfa/recovery-codes", h.limit, h.mfa.RecoveryCodes)
		signedIn.POST("/mfa/passkeys", h.limit, h.mfa.BeginPasskey)
		signedIn.POST("/mfa/passkeys/finish", h.limit, h.mfa.FinishPasskey)
		signedIn.DELETE("/mfa/passkeys/:id", h.limit, h.mfa.RemovePasskey)
		signedIn.GET("/sessions", h.auth.Sessions)
		signedIn.DELETE("/sessions/:id", h.auth.EndSession)
		signedIn.DELETE("/sessions", h.auth.EndOtherSessions)

		// Everything below is guarded by the administrator's permissions;
		// the panel hiding a control is not the check. These routes also
		// accept an admin API access token whose scopes stand in for roles;
		// routes about the signed-in person, and super-admin routes, stay
		// session-only.
		managed := v1.Group("/admin", session.RequireAny(service, provider.OAuth, log))

		activity := managed.Group("", session.Can(model.PermActivityRead))
		activity.GET("/overview", h.activity.Overview)
		activity.GET("/logs", h.activity.Logs)
		activity.GET("/logs/export", h.activity.Export)

		readUsers := managed.Group("", session.Can(model.PermUsersRead))
		readUsers.GET("/users", h.users.List)
		readUsers.GET("/users/:id", h.users.Get)
		readUsers.GET("/users/:id/roles", h.users.Roles)
		readUsers.GET("/users/:id/role-mappings", h.users.RoleMappings)
		readUsers.GET("/user-fields", h.fields.List)
		readUsers.GET("/user-sessions", h.userSessions.List)

		writeUsers := managed.Group("", session.Can(model.PermUsersWrite))
		writeUsers.POST("/users", h.users.Create)
		writeUsers.PATCH("/users/:id", h.users.Update)
		writeUsers.DELETE("/users/:id", h.users.Delete)
		writeUsers.DELETE("/users/:id/social-accounts/:identity", h.users.Disconnect)
		writeUsers.DELETE("/users/:id/two-step", h.users.ResetTwoStep)
		writeUsers.DELETE("/users/:id/sessions", h.userSessions.SignOutUser)
		writeUsers.DELETE("/user-sessions/:id", h.userSessions.End)

		writeFields := managed.Group("", session.Can(model.PermUserFieldsWrite))
		writeFields.POST("/user-fields", h.fields.Create)
		writeFields.PATCH("/user-fields/:id", h.fields.Update)
		writeFields.DELETE("/user-fields/:id", h.fields.Delete)

		managed.GET("/organization", session.Can(model.PermOrganizationRead), h.organization.Get)
		managed.PATCH("/organization", session.Can(model.PermOrganizationWrite), h.organization.Update)
		managed.PUT("/organization/logo", session.Can(model.PermOrganizationWrite), h.organization.SetLogo)
		managed.DELETE("/organization/logo", session.Can(model.PermOrganizationWrite), h.organization.RemoveLogo)

		readSocial := managed.Group("", session.Can(model.PermSocialRead))
		readSocial.GET("/social-providers", h.social.List)
		readSocial.GET("/social-providers/:id", h.social.Get)

		// Reading a secret back takes the permission that could replace
		// it, and is recorded like a change.
		writeSocial := managed.Group("", session.Can(model.PermSocialWrite))
		writeSocial.GET("/social-providers/:id/secret", h.social.Secret)
		writeSocial.POST("/social-providers", h.social.Create)
		writeSocial.PATCH("/social-providers/:id", h.social.Update)
		writeSocial.DELETE("/social-providers/:id", h.social.Delete)

		readSSO := managed.Group("", session.Can(model.PermSSORead))
		readSSO.GET("/sso-connections", h.sso.List)
		readSSO.GET("/sso-connections/:id", h.sso.Get)
		readSSO.GET("/sso-connections/:id/domains", h.sso.Domains)

		writeSSO := managed.Group("", session.Can(model.PermSSOWrite))
		writeSSO.POST("/sso-connections", h.sso.Create)
		writeSSO.POST("/sso-connections/test", h.sso.Test)
		writeSSO.PATCH("/sso-connections/:id", h.sso.Update)
		writeSSO.DELETE("/sso-connections/:id", h.sso.Delete)
		writeSSO.POST("/sso-connections/:id/refresh-metadata", h.sso.RefreshMetadata)
		writeSSO.POST("/sso-connections/:id/domains/:domain/check", h.limit, h.sso.CheckDomain)

		readFlows := managed.Group("", session.Can(model.PermLoginFlowsRead))
		readFlows.GET("/login-flows", h.flows.List)
		readFlows.GET("/login-flows/:id", h.flows.Get)

		writeFlows := managed.Group("", session.Can(model.PermLoginFlowsWrite))
		writeFlows.POST("/login-flows", h.flows.Create)
		writeFlows.PATCH("/login-flows/:id", h.flows.Update)
		writeFlows.DELETE("/login-flows/:id", h.flows.Delete)

		readLanguages := managed.Group("", session.Can(model.PermLanguagesRead))
		readLanguages.GET("/languages", h.languages.List)
		readLanguages.GET("/languages/:code/translations/:app", h.languages.Translation)

		writeLanguages := managed.Group("", session.Can(model.PermLanguagesWrite))
		writeLanguages.POST("/languages", h.languages.Create)
		writeLanguages.PATCH("/languages/:code", h.languages.Update)
		writeLanguages.DELETE("/languages/:code", h.languages.Delete)
		writeLanguages.PUT("/languages/:code/translations/:app", h.languages.SaveTranslation)

		// Applications and their roles. Routes only check the administrator
		// reaches some application; each handler checks the one requested.
		apps := managed.Group("", session.CanAnywhere(model.PermApplicationsRead))
		apps.GET("/applications", h.applications.List)
		apps.GET("/applications/:id", h.applications.Get)
		apps.PATCH("/applications/:id", h.applications.Update)
		apps.POST("/applications/:id/secret", h.applications.RotateSecret)
		apps.GET("/applications/:id/apis", h.applications.APIAccess)
		apps.PUT("/applications/:id/apis/:api", h.applications.AuthorizeAPI)
		apps.DELETE("/applications/:id/apis/:api", h.applications.RevokeAPI)
		apps.POST("/applications/:id/token-preview", h.applications.TokenPreview)

		registerApps := managed.Group("", session.Can(model.PermApplicationsWrite))
		registerApps.POST("/applications", h.applications.Create)
		registerApps.DELETE("/applications/:id", h.applications.Delete)

		// User roles, global and per application. Handlers narrow to the
		// scopes the administrator reaches and check writes against the
		// role's scope.
		roles := managed.Group("", session.CanAnywhere(model.PermUsersRead, model.PermApplicationsRead))
		roles.GET("/user-roles", h.roles.List)
		roles.GET("/user-roles/:id", h.roles.Get)

		writeRoles := managed.Group("", session.CanAnywhere(model.PermUserRolesWrite))
		writeRoles.POST("/user-roles", h.roles.Create)
		writeRoles.PATCH("/user-roles/:id", h.roles.Update)
		writeRoles.DELETE("/user-roles/:id", h.roles.Delete)

		assign := managed.Group("", session.CanAnywhere(model.PermRoleAssignmentsWrite))
		assign.POST("/users/:id/role-mappings", h.users.AssignRoles)
		assign.DELETE("/users/:id/role-mappings/:role", h.users.UnassignRole)

		readAPIs := managed.Group("", session.CanAnywhere(model.PermAPIsRead, model.PermApplicationsWrite))
		readAPIs.GET("/apis", h.apis.List)
		readAPIs.GET("/apis/:id", h.apis.Get)
		readAPIs.GET("/apis/:id/applications", h.apis.Applications)
		readAPIs.GET("/apis/:id/introspectors", h.apis.Introspectors)

		// An API's log names every application, visible or not, so it needs
		// apis.read itself.
		managed.GET("/apis/:id/logs", session.Can(model.PermAPIsRead), h.apis.Logs)

		writeAPIs := managed.Group("", session.Can(model.PermAPIsWrite))
		writeAPIs.POST("/apis", h.apis.Create)
		writeAPIs.PATCH("/apis/:id", h.apis.Update)
		writeAPIs.DELETE("/apis/:id", h.apis.Delete)
		writeAPIs.PUT("/apis/:id/introspectors", h.apis.SetIntrospectors)

		// The administrators themselves, and their roles: a super
		// admin's alone, whatever other roles grant.
		super := signedIn.Group("", session.RequireSuperAdmin())
		super.GET("/security", h.admins.Security)
		super.PATCH("/security", h.admins.UpdateSecurity)

		super.GET("/admins", h.admins.List)
		super.POST("/admins", h.admins.Create)
		super.GET("/admins/:id", h.admins.Get)
		super.PATCH("/admins/:id", h.admins.Update)
		super.DELETE("/admins/:id", h.admins.Delete)
		super.DELETE("/admins/:id/mfa", h.admins.ResetMFA)

		super.GET("/mail", h.mail.Get)
		super.PATCH("/mail", h.mail.Update)
		super.POST("/mail/test", h.limit, h.mail.Test)
		super.GET("/mail/content", h.mail.Content)
		super.PUT("/mail/content/:code", h.mail.SaveContent)

		super.GET("/otp", h.otp.Get)
		super.PATCH("/otp", h.otp.Update)

		super.GET("/signing-keys", h.keys.List)
		super.POST("/signing-keys/rotate", h.limit, h.keys.Rotate)

		super.GET("/admin-permissions", h.adminRoles.Permissions)
		super.GET("/admin-roles", h.adminRoles.List)
		super.POST("/admin-roles", h.adminRoles.Create)
		super.PATCH("/admin-roles/:id", h.adminRoles.Update)
		super.DELETE("/admin-roles/:id", h.adminRoles.Delete)
	}

	r.NoRoute(notFound)
}

// health says the server is up.
func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

var notReady = respond.Define(http.StatusServiceUnavailable, "not_ready", respond.Both)

// readyTimeout keeps a database that hangs from hanging the orchestrator's
// probe with it.
const readyTimeout = 2 * time.Second

// ready says the server can answer requests: its database answers.
func ready(provider *oidc.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readyTimeout)
		defer cancel()

		if err := provider.Ready(ctx); err != nil {
			respond.Fail(c, notReady)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	}
}

func notFound(c *gin.Context) {
	respond.Fail(c, respond.NotFoundAny)
}
