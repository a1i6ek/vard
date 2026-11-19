package oidc

import (
	"context"

	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/model"
	"github.com/a1i6ek/vard/internal/oauth"
)

// Discovery is the OpenID Connect Discovery 1.0 document.
type Discovery struct {
	Issuer                                    string   `json:"issuer"`
	AuthorizationEndpoint                     string   `json:"authorization_endpoint"`
	TokenEndpoint                             string   `json:"token_endpoint"`
	UserInfoEndpoint                          string   `json:"userinfo_endpoint"`
	EndSessionEndpoint                        string   `json:"end_session_endpoint"`
	RevocationEndpoint                        string   `json:"revocation_endpoint"`
	IntrospectionEndpoint                     string   `json:"introspection_endpoint"`
	JWKSURI                                   string   `json:"jwks_uri"`
	ResponseTypesSupported                    []string `json:"response_types_supported"`
	ResponseModesSupported                    []string `json:"response_modes_supported"`
	GrantTypesSupported                       []string `json:"grant_types_supported"`
	SubjectTypesSupported                     []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported          []string `json:"id_token_signing_alg_values_supported"`
	ScopesSupported                           []string `json:"scopes_supported"`
	ClaimsSupported                           []string `json:"claims_supported"`
	TokenEndpointAuthMethodsSupported         []string `json:"token_endpoint_auth_methods_supported"`
	RevocationEndpointAuthMethodsSupported    []string `json:"revocation_endpoint_auth_methods_supported"`
	IntrospectionEndpointAuthMethodsSupported []string `json:"introspection_endpoint_auth_methods_supported"`
	// private_key_jwt assertions are accepted signed with these, at every
	// endpoint a client authenticates at (RFC 8414 §2).
	TokenEndpointAuthSigningAlgValuesSupported         []string `json:"token_endpoint_auth_signing_alg_values_supported"`
	RevocationEndpointAuthSigningAlgValuesSupported    []string `json:"revocation_endpoint_auth_signing_alg_values_supported"`
	IntrospectionEndpointAuthSigningAlgValuesSupported []string `json:"introspection_endpoint_auth_signing_alg_values_supported"`
	CodeChallengeMethodsSupported                      []string `json:"code_challenge_methods_supported"`
	PromptValuesSupported                              []string `json:"prompt_values_supported"`
	AuthorizationResponseIssParameterSupported         bool     `json:"authorization_response_iss_parameter_supported"`

	// Back-Channel Logout 1.0: applications with a backchannel_logout_uri
	// are told when a session ends, and the logout token always has a sid.
	BackchannelLogoutSupported        bool `json:"backchannel_logout_supported"`
	BackchannelLogoutSessionSupported bool `json:"backchannel_logout_session_supported"`

	// RFC 9126: requests may be pushed first; each application says whether
	// it must, so the server as a whole does not require it.
	PushedAuthorizationRequestEndpoint string `json:"pushed_authorization_request_endpoint"`
	DeviceAuthorizationEndpoint        string `json:"device_authorization_endpoint"`
	// RegistrationEndpoint takes clients' metadata (RFC 7591), with an
	// initial access token.
	RegistrationEndpoint string `json:"registration_endpoint"`

	// RFC 9449: proofs are accepted signed with these.
	DPoPSigningAlgValuesSupported      []string `json:"dpop_signing_alg_values_supported"`
	RequirePushedAuthorizationRequests bool     `json:"require_pushed_authorization_requests"`

	// The organisation's terms and privacy links (op_tos_uri, op_policy_uri),
	// omitted when unset.
	OpTosURI    string `json:"op_tos_uri,omitempty"`
	OpPolicyURI string `json:"op_policy_uri,omitempty"`

	ClaimsParameterSupported     bool `json:"claims_parameter_supported"`
	RequestParameterSupported    bool `json:"request_parameter_supported"`
	RequestURIParameterSupported bool `json:"request_uri_parameter_supported"`
}

// Discovery describes the provider from the model, so it cannot claim anything
// the server does not offer. Terms and privacy links come from the database; if
// it is unreachable they are left out and the rest is still served.
func (s *Service) Discovery(ctx context.Context) Discovery {
	clientMethods := []string{string(model.AuthPrivateKeyJWT), string(model.AuthClientSecretBasic), string(model.AuthClientSecretPost)}

	var tos, policy string
	if organization, err := s.store.Organization(ctx); err == nil {
		tos, policy = organization.TermsURL, organization.PrivacyURL
	} else {
		s.log.Error("reading the organization for the discovery document failed", "error", err)
	}

	return Discovery{
		Issuer:                                             s.issuer,
		AuthorizationEndpoint:                              s.issuer + authreq.PathAuthorize,
		TokenEndpoint:                                      s.issuer + authreq.PathToken,
		UserInfoEndpoint:                                   s.issuer + authreq.PathUserInfo,
		EndSessionEndpoint:                                 s.issuer + authreq.PathLogout,
		RevocationEndpoint:                                 s.issuer + authreq.PathRevoke,
		IntrospectionEndpoint:                              s.issuer + authreq.PathIntrospect,
		JWKSURI:                                            s.issuer + authreq.PathJWKS,
		ResponseTypesSupported:                             []string{"code"},
		ResponseModesSupported:                             []string{"query"},
		GrantTypesSupported:                                model.GrantTypes,
		SubjectTypesSupported:                              []string{"public"},
		IDTokenSigningAlgValuesSupported:                   []string{oauth.IDTokenAlgorithm},
		ScopesSupported:                                    model.Scopes,
		ClaimsSupported:                                    claimsSupported,
		TokenEndpointAuthMethodsSupported:                  append(clientMethods, string(model.AuthNone)),
		RevocationEndpointAuthMethodsSupported:             append(append([]string{}, clientMethods...), string(model.AuthNone)),
		IntrospectionEndpointAuthMethodsSupported:          clientMethods,
		TokenEndpointAuthSigningAlgValuesSupported:         oauth.ClientAssertionAlgorithms,
		RevocationEndpointAuthSigningAlgValuesSupported:    oauth.ClientAssertionAlgorithms,
		IntrospectionEndpointAuthSigningAlgValuesSupported: oauth.ClientAssertionAlgorithms,
		CodeChallengeMethodsSupported:                      []string{model.PKCES256},
		PromptValuesSupported:                              []string{"none", "login", "consent"},
		AuthorizationResponseIssParameterSupported:         true,
		BackchannelLogoutSupported:                         true,
		BackchannelLogoutSessionSupported:                  true,
		PushedAuthorizationRequestEndpoint:                 s.issuer + authreq.PathPushedAuthorization,
		DeviceAuthorizationEndpoint:                        s.issuer + authreq.PathDeviceAuthorization,
		RegistrationEndpoint:                               s.issuer + authreq.PathRegistration,
		DPoPSigningAlgValuesSupported:                      oauth.DPoPAlgorithms,
		OpTosURI:                                           tos,
		OpPolicyURI:                                        policy,
	}
}

var claimsSupported = []string{
	"iss", "sub", "aud", "exp", "iat", "auth_time", "amr", "nonce", "at_hash", "sid",
	"azp", "client_id", "scope", "jti",
	"name", "given_name", "family_name", "email", "email_verified",
	model.ClaimApplicationRoles, model.ClaimGlobalRoles,
}

// Ready says the server can answer: its database does.
func (s *Service) Ready(ctx context.Context) error {
	return s.store.Ready(ctx)
}
