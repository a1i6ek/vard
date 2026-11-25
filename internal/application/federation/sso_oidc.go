package federation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/a1i6ek/vard/internal/application/authreq"
	"github.com/a1i6ek/vard/internal/application/signin"
	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/outbound"
	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
)

type oidcDiscovery struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	UserInfoEndpoint      string   `json:"userinfo_endpoint"`
	JWKSURI               string   `json:"jwks_uri"`
	TokenAuthMethods      []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported       []string `json:"scopes_supported"`
	// ResponseIssuer says the provider names itself in every authorization
	// response (RFC 9207), so one that does not is refused.
	ResponseIssuer bool `json:"authorization_response_iss_parameter_supported"`
}

// ssoCache holds what identity providers publish about themselves, for an
// hour, so a sign-in does not fetch their discovery document and keys again.
type ssoCache struct {
	mu        sync.Mutex
	discovery map[string]cached[*oidcDiscovery]
	keys      *outbound.KeySets
}

type cached[T any] struct {
	value   T
	fetched time.Time
}

const (
	ssoCacheLifetime = time.Hour
	// keyRefetchEvery is how often a token that does not verify may make
	// the server fetch the provider's keys again.
	keyRefetchEvery = outbound.KeyRefetchEvery
)

func newSSOCache() *ssoCache {
	return &ssoCache{
		discovery: map[string]cached[*oidcDiscovery]{},
		keys:      outbound.NewKeySets(ssoCacheLifetime, socialTimeout),
	}
}

// Discover reads an identity provider's discovery document. The issuer it
// names must be the connection's issuer exactly, byte for byte (OpenID
// Connect Discovery §4.3): a trailing slash is a different issuer, whose
// tokens are not this connection's. Only the document's address drops one,
// before the well-known path is added (§4).
func (s *Service) Discover(ctx context.Context, issuer string) (*oidcDiscovery, error) {
	s.ssoCache.mu.Lock()
	if hit, ok := s.ssoCache.discovery[issuer]; ok && s.now().Sub(hit.fetched) < ssoCacheLifetime {
		s.ssoCache.mu.Unlock()
		return hit.value, nil
	}
	s.ssoCache.mu.Unlock()

	var document oidcDiscovery
	if err := s.getJSON(ctx, strings.TrimSuffix(issuer, "/")+"/.well-known/openid-configuration", &document); err != nil {
		return nil, err
	}

	// The document must be the issuer's own (OpenID Connect Discovery 4.3), or
	// another issuer's tokens would be believed.
	switch {
	case document.Issuer != issuer:
		return nil, fmt.Errorf("the discovery document is for %q, not %q", document.Issuer, issuer)
	case document.AuthorizationEndpoint == "" || document.TokenEndpoint == "" || document.JWKSURI == "":
		return nil, fmt.Errorf("the discovery document is missing an endpoint")
	}

	s.ssoCache.mu.Lock()
	s.ssoCache.discovery[issuer] = cached[*oidcDiscovery]{value: &document, fetched: s.now()}
	s.ssoCache.mu.Unlock()

	return &document, nil
}

func (s *Service) startOIDC(ctx context.Context, connection *model.SSOConnection, login *model.SSOLogin, state, loginHint string) (string, error) {
	document, err := s.Discover(ctx, connection.Issuer)
	if err != nil {
		return "", upstream("discovery", err)
	}

	verifier, _, err := model.NewSecret()
	if err != nil {
		return "", err
	}
	nonce, _, err := model.NewSecret()
	if err != nil {
		return "", err
	}
	login.Verifier, login.Nonce = verifier, nonce

	values := url.Values{
		"response_type":         {"code"},
		"client_id":             {connection.ClientID},
		"redirect_uri":          {connection.CallbackURL(s.issuer)},
		"scope":                 {strings.Join(connection.AskedScopes(), " ")},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {challengeFor(verifier)},
		"code_challenge_method": {model.PKCES256},
	}
	if loginHint != "" {
		values.Set("login_hint", loginHint)
	}

	return authreq.WithQuery(document.AuthorizationEndpoint, values), nil
}

func (s *Service) completeOIDC(ctx context.Context, connection *model.SSOConnection, login *model.SSOLogin, code, iss string) (*signin.SSOPerson, error) {
	document, err := s.Discover(ctx, connection.Issuer)
	if err != nil {
		return nil, upstream("discovery", err)
	}

	// RFC 9207: a response naming another issuer was meant for another
	// connection (a mix-up attack); one that leaves out the issuer it
	// promised to name is refused as well.
	switch {
	case iss != "" && iss != document.Issuer:
		return nil, upstream("callback", fmt.Errorf("the response names issuer %q", iss))
	case iss == "" && document.ResponseIssuer:
		return nil, upstream("callback", errors.New("the response does not name its issuer, which the provider says it always does"))
	}

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {connection.CallbackURL(s.issuer)},
		"code_verifier": {login.Verifier},
	}

	secret := ""
	basic := false
	if connection.ClientAuth == model.SSOClientPrivateKey {
		signed, err := s.upstreamAssertion(connection, document.TokenEndpoint)
		if err != nil {
			return nil, err
		}
		form.Set("client_id", connection.ClientID)
		form.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
		form.Set("client_assertion", signed)
	} else {
		if len(connection.ClientSecret) > 0 {
			plain, err := s.sealer.OpenFor(connection.ClientSecretWhere(), connection.ClientSecret)
			if err != nil {
				return nil, err
			}
			secret = string(plain)
		}
		basic = len(document.TokenAuthMethods) == 0 || slices.Contains(document.TokenAuthMethods, "client_secret_basic")
		if !basic {
			form.Set("client_id", connection.ClientID)
			form.Set("client_secret", secret)
		}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, document.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	if basic {
		request.SetBasicAuth(url.QueryEscape(connection.ClientID), url.QueryEscape(secret))
	}

	var token struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	if err := s.doJSON(request, &token); err != nil {
		return nil, upstream("token", err)
	}

	claims, err := s.verifyIDToken(ctx, connection, document, login, token.IDToken)
	if err != nil {
		return nil, upstream("id_token", err)
	}

	// What the sign-in proved is the signed id_token's to say: read before
	// userinfo, unsigned, fills in what the token left out.
	proved := idTokenAMR(claims)

	if claimFirst(claims, connection.Attribute("email")) == "" && document.UserInfoEndpoint != "" && token.AccessToken != "" {
		if extra, err := s.oidcUserInfo(ctx, document.UserInfoEndpoint, token.AccessToken); err == nil &&
			claimString(extra, "sub") == claimString(claims, "sub") {
			for name, value := range extra {
				if _, has := claims[name]; !has {
					claims[name] = value
				}
			}
		}
	}

	// An address is proved only when the provider vouches for it: an account
	// is made, or linked, as the address's owner's.
	email, from := claimFirstNamed(claims, connection.Attribute("email"))
	if said := verifiedClaim(claims, from); email != "" && !vouchedFor(said, connection.AssumeEmailVerified) {
		switch {
		case from != "email":
			return nil, authreq.Refused(ErrSSONoEmail, "%s comes from %s, which email_verified does not speak for; the connection may trust addresses sent without it", email, from)
		case said == nil:
			return nil, authreq.Refused(ErrSSONoEmail, "the provider does not say it has verified %s (no email_verified); the connection may trust addresses sent without it", email)
		}
		return nil, authreq.Refused(ErrSSONoEmail, "the provider says it has not verified %s (email_verified is %v)", email, said)
	}

	return &signin.SSOPerson{
		Subject:   claimString(claims, "sub"),
		Email:     email,
		FirstName: claimFirst(claims, connection.Attribute("first_name")),
		LastName:  claimFirst(claims, connection.Attribute("last_name")),
		Groups:    claimList(claims, connection.Attribute("groups")),
		Proved:    proved,
		Sent:      slices.Sorted(maps.Keys(claims)),
	}, nil
}

// upstreamAssertion is this server's client assertion to the provider's
// token endpoint (RFC 7523 §2.2): signed with the connection's own key, for
// that endpoint, once, for a minute.
func (s *Service) upstreamAssertion(connection *model.SSOConnection, tokenEndpoint string) (string, error) {
	key, err := s.connectionKey(connection)
	if err != nil {
		return "", err
	}
	jti, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	now := s.now()
	return jose.Sign(key, "JWT", map[string]any{
		"iss": connection.ClientID,
		"sub": connection.ClientID,
		"aud": tokenEndpoint,
		"jti": jti.String(),
		"iat": now.Unix(),
		"exp": now.Add(time.Minute).Unix(),
	})
}

// connectionKey is the connection's own signing key, with its thumbprint
// as its kid, as JWKS publishes it.
func (s *Service) connectionKey(connection *model.SSOConnection) (jose.Key, error) {
	if len(connection.SPKey) == 0 {
		return jose.Key{}, errors.New("the connection has no key of its own")
	}
	private, err := s.sealer.Open(connection.SPKeyWhere(), connection.SPKey)
	if err != nil {
		return jose.Key{}, fmt.Errorf("open the connection's key: %w", err)
	}
	key := jose.Key{Algorithm: jose.RS256, Private: private}
	public, err := key.Public()
	if err != nil {
		return jose.Key{}, err
	}
	if key.ID, err = jose.Thumbprint(public); err != nil {
		return jose.Key{}, err
	}
	return key, nil
}

// SSOJWKS is the public key a connection that signs its client assertions
// publishes, for its provider to check them by.
func (s *Service) SSOJWKS(ctx context.Context, slug string) (map[string]any, error) {
	connection, err := s.ssoConnection(ctx, slug)
	if err != nil {
		return nil, err
	}
	if connection.Protocol != model.SSOProtocolOIDC || connection.ClientAuth != model.SSOClientPrivateKey {
		return nil, ErrSSOExpired
	}
	key, err := s.connectionKey(connection)
	if err != nil {
		return nil, err
	}
	public, err := key.Public()
	if err != nil {
		return nil, err
	}
	public.KeyID, public.Algorithm, public.Use = key.ID, key.Algorithm, "sig"
	return map[string]any{"keys": []jose.JWK{public}}, nil
}

// idTokenAMR is an id_token's amr: an array of strings (RFC 8176), and
// nothing else read as one.
func idTokenAMR(claims map[string]any) []string {
	values, _ := claims["amr"].([]any)
	var amr []string
	for _, value := range values {
		if text, ok := value.(string); ok && text != "" {
			amr = append(amr, text)
		}
	}
	return amr
}

// emailVerified reads email_verified as a boolean or, as Cognito and others
// send it, a string: "false" must not read as unsaid.
func emailVerified(value any) (said, verified bool) {
	switch v := value.(type) {
	case bool:
		return true, v
	case string:
		return true, strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false, false
	}
}

// verifiedClaim is the email_verified that speaks for an address read from
// the claim named from: OpenID Connect Core §5.1 makes it about the email
// claim alone, so an address mapped from any other (upn,
// preferred_username, …) has it unsaid.
func verifiedClaim(claims map[string]any, from string) any {
	if from != "email" {
		return nil
	}
	return claims["email_verified"]
}

// vouchedFor says whether a provider's email_verified proves the address:
// true does, false never does, and an unsaid one only for a connection that
// assumes so (model.SSOConnection.AssumeEmailVerified). A claim sent as
// anything but a boolean or its string is not unsaid: it proves nothing.
func vouchedFor(value any, assume bool) bool {
	if value == nil {
		return assume
	}
	_, verified := emailVerified(value)
	return verified
}

const idTokenLeeway = time.Minute

// verifyIDToken checks an id_token is the provider's, for this client, in
// time, and for this sign-in (OpenID Connect Core section 3.1.3.7).
func (s *Service) verifyIDToken(
	ctx context.Context,
	connection *model.SSOConnection,
	document *oidcDiscovery,
	login *model.SSOLogin,
	idToken string,
) (map[string]any, error) {
	if idToken == "" {
		return nil, fmt.Errorf("no id_token")
	}

	var claims map[string]any
	if err := s.verifyPublished(ctx, document.JWKSURI, idToken, &claims); err != nil {
		return nil, err
	}

	now := s.now()

	switch {
	case claimString(claims, "iss") != document.Issuer:
		return nil, fmt.Errorf("issued by %q", claimString(claims, "iss"))
	case !audienceHas(claims["aud"], connection.ClientID):
		return nil, fmt.Errorf("for another client")
	case claimString(claims, "nonce") != login.Nonce:
		return nil, fmt.Errorf("for another sign-in")
	}

	if azp := claimString(claims, "azp"); azp != "" && azp != connection.ClientID {
		return nil, fmt.Errorf("authorised for another client")
	}

	exp, ok := numberClaim(claims["exp"])
	if !ok || now.After(time.Unix(int64(exp), 0).Add(idTokenLeeway)) {
		return nil, fmt.Errorf("expired")
	}
	if iat, ok := numberClaim(claims["iat"]); ok && time.Unix(int64(iat), 0).After(now.Add(idTokenLeeway)) {
		return nil, fmt.Errorf("issued in the future")
	}

	return claims, nil
}

// verifyPublished checks a token's signature against the keys a provider
// publishes at jwksURI and decodes its claims, refetching them at most once
// a keyRefetchEvery for a token naming a key not held (outbound.KeySets).
func (s *Service) verifyPublished(ctx context.Context, jwksURI, token string, claims any) error {
	return s.ssoCache.keys.Verify(ctx, s.social, s.now, jwksURI, token, claims)
}

// providerKeys is the key set published at jwksURI, from the cache while it
// is fresh; refresh fetches it again at most once a keyRefetchEvery.
func (s *Service) providerKeys(ctx context.Context, jwksURI string, refresh bool) ([]jose.Key, error) {
	return s.ssoCache.keys.Keys(ctx, s.social, s.now, jwksURI, refresh)
}

func (s *Service) oidcUserInfo(ctx context.Context, endpoint, accessToken string) (map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/json")

	var claims map[string]any
	return claims, s.doJSON(request, &claims)
}

func (s *Service) getJSON(ctx context.Context, address string, into any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")

	return s.doJSON(request, into)
}

func (s *Service) doJSON(request *http.Request, into any) error {
	ctx, cancel := context.WithTimeout(request.Context(), socialTimeout)
	defer cancel()

	response, err := s.social.Do(request.WithContext(ctx))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %d: %.200s", request.URL.Host, response.StatusCode, body)
	}

	return json.Unmarshal(body, into)
}

func claimFirst(claims map[string]any, names []string) string {
	value, _ := claimFirstNamed(claims, names)
	return value
}

// claimFirstNamed is claimFirst with the name of the claim it came from.
func claimFirstNamed(claims map[string]any, names []string) (value, name string) {
	for _, name := range names {
		if value := claimString(claims, name); value != "" {
			return value, name
		}
	}

	return "", ""
}

func claimList(claims map[string]any, names []string) []string {
	for _, name := range names {
		switch value := claimAt(claims, name).(type) {
		case []any:
			var out []string
			for _, item := range value {
				if text, ok := item.(string); ok && text != "" {
					out = append(out, text)
				}
			}
			return out
		case string:
			if value != "" {
				return []string{value}
			}
		}
	}

	return nil
}
