package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/a1i6ek/vard/internal/application/authreq"
	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
)

// IDTokenAlgorithm signs an application's ID tokens unless it asks for
// another (id_token_signed_response_alg). RS256 is the one algorithm every
// OpenID Connect client must accept; access tokens follow their API's choice.
const IDTokenAlgorithm = jose.RS256

// IDTokenType is the typ an ID token is signed with, which is what tells it
// apart from an access token ("at+jwt") or a logout token signed by the same
// keys.
const IDTokenType = "JWT"

type ClientAuth struct {
	ID     string
	Secret string
	// Assertion is a private_key_jwt client's signed assertion.
	Assertion string
	Method    model.AuthMethod
	// Endpoint is the path the client called, which an assertion's aud may
	// name.
	Endpoint string
}

func (s *Service) authenticate(ctx context.Context, auth ClientAuth) (*model.Application, error) {
	if auth.ID == "" && auth.Method == model.AuthPrivateKeyJWT {
		auth.ID = assertedClient(auth.Assertion)
	}
	if auth.ID == "" {
		return nil, authreq.OAuthError(authreq.ErrInvalidClient, "client authentication is required")
	}

	app, err := s.store.ApplicationByClientID(ctx, auth.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, authreq.OAuthError(authreq.ErrInvalidClient, "unknown client")
	}
	if err != nil {
		return nil, err
	}

	registered := app.TokenAuthMethod
	switch {
	case registered == model.AuthNone && auth.Method != model.AuthNone:
		return nil, authreq.OAuthError(authreq.ErrInvalidClient, "this is a public client: send client_id alone, with no secret")
	case registered != model.AuthNone && auth.Method != registered:
		return nil, authreq.OAuthError(authreq.ErrInvalidClient, "this client authenticates with "+string(registered))
	case registered == model.AuthPrivateKeyJWT:
		if err := s.verifyAssertion(ctx, app, auth.Assertion, auth.Endpoint); err != nil {
			return nil, err
		}
	case registered != model.AuthNone && !app.CheckSecret(auth.Secret, s.now()):
		return nil, authreq.OAuthError(authreq.ErrInvalidClient, "client authentication failed")
	}

	if !app.IsEnabled {
		return nil, authreq.OAuthError(authreq.ErrUnauthorizedClient, "the application is disabled")
	}

	return app, nil
}

type TokenParams struct {
	Client       ClientAuth
	GrantType    string
	Code         string
	RedirectURI  string
	CodeVerifier string
	RefreshToken string
	DeviceCode   string
	Scope        string
	Resource     string
	// DPoP is the request's DPoP proof, "" when it sent none.
	DPoP string

	// A token exchange's (RFC 8693 §2.1).
	SubjectToken       string
	SubjectTokenType   string
	ActorToken         string
	ActorTokenType     string
	RequestedTokenType string
	// From is who sent the request, for the activity log.
	From Caller

	// dpopKey is the thumbprint of the key the proof showed, once checked.
	dpopKey string
}

// TokenResponse is a successful token response (RFC 6749 section 5.1).
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	// IssuedTokenType is what a token exchange issued (RFC 8693 §2.2.1).
	IssuedTokenType string `json:"issued_token_type,omitempty"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int64  `json:"expires_in"`
	RefreshToken    string `json:"refresh_token,omitempty"`
	IDToken         string `json:"id_token,omitempty"`
	Scope           string `json:"scope"`
}

func (s *Service) Token(ctx context.Context, p TokenParams) (*TokenResponse, error) {
	app, err := s.authenticate(ctx, p.Client)
	if err != nil {
		return nil, err
	}

	// An exchange is allowed by an administrator's rule, not by a grant on
	// the application: the rule names the API and whose tokens.
	exchanging := p.GrantType == model.GrantTokenExchange
	if !exchanging && !slices.Contains(model.GrantTypes, p.GrantType) {
		return nil, authreq.OAuthError(authreq.ErrUnsupportedGrantType, "grant_type must be one of: "+strings.Join(model.GrantTypes, ", "))
	}
	if !exchanging && !slices.Contains(app.GrantTypes, p.GrantType) {
		return nil, authreq.OAuthError(authreq.ErrUnauthorizedClient, "the application does not have the "+p.GrantType+" grant")
	}

	if p.DPoP != "" {
		if p.dpopKey, err = s.checkDPoP(ctx, p.DPoP, "POST", s.issuer+authreq.PathToken, "", true); err != nil {
			return nil, err
		}
	} else if app.DPoPBoundAccessTokens {
		// RFC 9449 §5.2: such a client's tokens are always bound to a key.
		return nil, authreq.OAuthError(ErrInvalidDPoPProof, "this client's tokens are bound to a DPoP key: send a DPoP proof")
	}

	switch p.GrantType {
	case model.GrantAuthorizationCode:
		return s.exchangeCode(ctx, app, p)
	case model.GrantRefreshToken:
		return s.refresh(ctx, app, p)
	case model.GrantDeviceCode:
		return s.deviceToken(ctx, app, p)
	case model.GrantTokenExchange:
		return s.exchangeToken(ctx, app, p)
	default:
		return s.clientCredentials(ctx, app, p)
	}
}

func (s *Service) exchangeCode(ctx context.Context, app *model.Application, p TokenParams) (*TokenResponse, error) {
	if p.Code == "" {
		return nil, authreq.OAuthError(authreq.ErrInvalidRequest, "code is required")
	}

	now := s.now()
	code, err := s.store.ClaimAuthorizationCode(ctx, model.HashSecret(p.Code), app.ID, now)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the authorization code is not valid")
	case errors.Is(err, store.ErrAlreadyUsed):
		// Found but not claimed: another client's code is left alone so its
		// owner can still spend it; this client's own code presented twice
		// means it leaked, so what the first exchange issued is revoked.
		if code.ApplicationID != app.ID {
			return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the authorization code was issued to another client")
		}
		if err := s.store.RevokeCodeGrant(ctx, code, app.ClientID, now); err != nil {
			return nil, err
		}
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the authorization code has already been used")
	case err != nil:
		return nil, err
	}

	switch {
	case !now.Before(code.ExpiresAt):
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the authorization code has expired")
	case p.RedirectURI != code.RedirectURI:
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "redirect_uri does not match the authorization request")
	case !app.AllowsRedirect(code.RedirectURI):
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "redirect_uri is no longer registered for this application")
	case code.CodeChallenge == "":
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the code was issued without PKCE, which every client uses now")
	case !model.VerifyPKCE(code.CodeChallenge, p.CodeVerifier):
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "code_verifier does not match the code_challenge")
	case code.DPoPKey != "" && p.dpopKey != code.DPoPKey:
		// Checked after the claim, as PKCE is, so a code tried without its
		// key is spent and cannot be tried again with a stolen one.
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the authorization code is bound to a DPoP key: send a DPoP proof from that key")
	}

	user, err := s.store.User(ctx, code.UserID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the user no longer exists")
	}
	if err != nil {
		return nil, err
	}

	// The session the code came from must still be alive, or a code minted just
	// before a password reset would start a fresh refresh-token family for
	// whoever the reset shut out.
	if code.SessionID != nil {
		session, err := s.store.UserSession(ctx, *code.SessionID)
		switch {
		case errors.Is(err, store.ErrNotFound):
			return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the session the code was issued in has ended")
		case err != nil:
			return nil, err
		case !session.Active(now):
			return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the session the code was issued in has ended")
		}
	}

	response, err := s.issue(ctx, authreq.Grant{
		App:      app,
		User:     user,
		Scopes:   strings.Fields(code.Scope),
		Audience: code.Audience,
		Nonce:    code.Nonce,
		AuthTime: code.AuthenticatedAt,
		Methods:  code.Methods,
		Session:  code.SessionID,
		CodeID:   &code.ID,
		DPoPKey:  p.dpopKey,
		Refusal:  authreq.ErrInvalidGrant,
	})
	if err != nil {
		return nil, err
	}

	// The tokens are already issued and the code spent, so failing here would
	// only lose them; the application is then just not told of the logout.
	if code.SessionID != nil && response.IDToken != "" {
		if err := s.store.RememberSessionApplication(ctx, *code.SessionID, app.ID); err != nil {
			s.log.Error("recording the application a session signed in failed", "error", err)
		}
	}

	return response, nil
}

func (s *Service) refresh(ctx context.Context, app *model.Application, p TokenParams) (*TokenResponse, error) {
	if p.RefreshToken == "" {
		return nil, authreq.OAuthError(authreq.ErrInvalidRequest, "refresh_token is required")
	}

	now := s.now()
	old, err := s.store.RefreshTokenByHash(ctx, model.HashSecret(p.RefreshToken))
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the refresh token is not valid")
	case err != nil:
		return nil, err
	}

	if old.ApplicationID != app.ID {
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the refresh token was issued to another client")
	}

	var again string
	if old.RevokedAt != nil {
		if again, err = s.retried(ctx, old, now); err != nil {
			return nil, err
		}
		if again == "" {
			// A replaced token presented again: one of the two presenters is
			// not the client. The whole family goes, so the thief's copy dies
			// too.
			if err := s.store.RevokeRefreshFamily(ctx, old.FamilyID, now); err != nil {
				return nil, err
			}
			return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the refresh token has been revoked")
		}
	}

	if !now.Before(old.ExpiresAt) {
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the refresh token has expired")
	}
	if old.Idle(now) && again == "" {
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the refresh token has expired: it was not used for too long")
	}

	// A family bound to a key goes on only with a proof of it.
	if old.DPoPKey != "" && p.dpopKey != old.DPoPKey {
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the refresh token is bound to a DPoP key; send a proof from it")
	}

	// A flow that has come to require a second factor since the family began
	// ends a family that never passed one, a retry included: the person signs
	// in again, and nothing is revoked meanwhile.
	flow, err := s.store.EffectiveLoginFlow(ctx, app)
	if err != nil {
		return nil, err
	}
	if flow.RequireSecondFactor && !model.ProvedSecondFactor(old.Methods) {
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the application now requires a second factor; sign in again")
	}

	original := strings.Fields(old.Scope)
	scopes := original
	if p.Scope != "" {
		scopes = strings.Fields(p.Scope)
		for _, scope := range scopes {
			if !slices.Contains(original, scope) {
				return nil, authreq.OAuthError(authreq.ErrInvalidScope, "a refresh can only narrow the original scope; "+scope+" was not granted")
			}
		}
	}

	user, err := s.store.User(ctx, old.UserID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the user no longer exists")
	}
	if err != nil {
		return nil, err
	}

	return s.issue(ctx, authreq.Grant{
		App:      app,
		User:     user,
		Scopes:   scopes,
		Audience: old.Audience,
		AuthTime: old.AuthenticatedAt,
		Methods:  old.Methods,
		Session:  old.SessionID,
		Replaces: old,
		Again:    again,
		DPoPKey:  p.dpopKey,
		Refusal:  authreq.ErrInvalidGrant,
	})
}

func (s *Service) clientCredentials(ctx context.Context, app *model.Application, p TokenParams) (*TokenResponse, error) {
	if app.TokenAuthMethod == model.AuthNone {
		return nil, authreq.OAuthError(authreq.ErrUnauthorizedClient, "a public client cannot use client_credentials")
	}
	// A service's token is for one named API (RFC 8707), never a default.
	if p.Resource == "" {
		return nil, authreq.OAuthError(authreq.ErrInvalidTarget, "resource is required: name the API the token is for")
	}

	return s.issue(ctx, authreq.Grant{
		App:      app,
		Scopes:   strings.Fields(p.Scope),
		Audience: p.Resource,
		DPoPKey:  p.dpopKey,
		Refusal:  authreq.ErrUnauthorizedClient,
	})
}

func (s *Service) issue(ctx context.Context, g authreq.Grant) (*TokenResponse, error) {
	evaluated, err := s.evaluate(ctx, g.App, g.User, g.Scopes, g.Audience)
	if err != nil {
		return nil, err
	}
	if !evaluated.Issued {
		return nil, authreq.OAuthError(g.Refusal, evaluated.Reason)
	}

	jti, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	// A refresh token is decided now, so the access token can name the family
	// it belongs to: revoking the family ends it too.
	refreshes := g.Again != "" || g.User != nil &&
		(g.Replaces != nil || slices.Contains(strings.Fields(scopeOf(evaluated.AccessToken)), model.ScopeOfflineAccess))
	switch {
	case g.Replaces != nil:
		g.Family = &g.Replaces.FamilyID
	case refreshes:
		family, err := uuid.NewV7()
		if err != nil {
			return nil, err
		}
		g.Family = &family
	}

	access := evaluated.AccessToken
	access["jti"] = jti.String()
	if g.Session != nil {
		access[claimSession] = g.Session.String()
	}
	if g.Family != nil {
		access[claimFamily] = g.Family.String()
	}
	if cnf := authreq.Confirmation(g); len(cnf) > 0 {
		access["cnf"] = cnf
	}
	if g.User != nil {
		access["auth_time"] = g.AuthTime.Unix()
	}

	algorithm, _ := evaluated.AccessTokenHeader["alg"].(string)
	accessKey, ok := s.keys.Signing(algorithm)
	if !ok {
		return nil, errors.New("oidc: no signing key for " + algorithm)
	}

	accessToken, err := jose.Sign(accessKey, "at+jwt", access)
	if err != nil {
		return nil, err
	}

	granted, _ := access["scope"].(string)
	iat, _ := access["iat"].(int64)
	exp, _ := access["exp"].(int64)

	response := &TokenResponse{
		AccessToken: accessToken,
		TokenType:   tokenType(g.DPoPKey),
		ExpiresIn:   exp - iat,
		Scope:       granted,
	}

	if evaluated.IDToken != nil {
		id := evaluated.IDToken
		id["auth_time"] = g.AuthTime.Unix()
		if amr := model.AMR(g.Methods); amr != nil {
			id["amr"] = amr
		}
		id["at_hash"] = jose.HalfHash(accessToken)
		id["azp"] = g.App.ClientID
		if g.Nonce != "" {
			id["nonce"] = g.Nonce
		}
		if g.Session != nil {
			id["sid"] = g.Session.String()
		}

		idKey, ok := s.keys.Signing(g.App.IDTokenSigningAlg)
		if !ok {
			return nil, errors.New("oidc: no signing key for " + g.App.IDTokenSigningAlg)
		}

		if response.IDToken, err = jose.Sign(idKey, IDTokenType, id); err != nil {
			return nil, err
		}
	}

	// A refresh always rotates, even if offline_access was narrowed away, so a
	// stolen token cannot be replayed without reaching reuse detection. Other
	// grants get a refresh token only with offline_access.
	switch {
	case g.Again != "":
		response.RefreshToken = g.Again
	case refreshes:
		token, err := s.newRefreshToken(ctx, g, granted)
		if err != nil {
			return nil, err
		}
		response.RefreshToken = token
	}

	return response, nil
}

func (s *Service) newRefreshToken(ctx context.Context, g authreq.Grant, scope string) (string, error) {
	now := s.now()

	token, hash, err := model.NewSecret()
	if err != nil {
		return "", err
	}

	next := &model.RefreshToken{
		TokenHash:       hash,
		ApplicationID:   g.App.ID,
		UserID:          g.User.ID,
		CodeID:          g.CodeID,
		SessionID:       g.Session,
		DPoPKey:         refreshBinding(g, g.DPoPKey),
		Scope:           scope,
		Audience:        g.Audience,
		AuthenticatedAt: g.AuthTime,
		Methods:         g.Methods,
		ExpiresAt:       now.Add(time.Duration(g.App.RefreshTokenLifetime) * time.Second),
	}
	next.IdleExpiresAt = idleUntil(now, g.App, next.ExpiresAt)

	if g.Replaces == nil {
		next.FamilyID = *g.Family
		return token, s.store.CreateRefreshToken(ctx, next)
	}

	next.FamilyID = g.Replaces.FamilyID
	next.CodeID = g.Replaces.CodeID
	next.DPoPKey = g.Replaces.DPoPKey
	next.ExpiresAt = g.Replaces.ExpiresAt
	next.IdleExpiresAt = idleUntil(now, g.App, next.ExpiresAt)

	if next.ID, err = uuid.NewV7(); err != nil {
		return "", err
	}
	sealed, err := s.sealer.SealFor(g.Replaces.SuccessorWhere(), []byte(token))
	if err != nil {
		return "", err
	}

	err = s.store.RotateRefreshToken(ctx, g.Replaces, next, sealed, now)
	if !errors.Is(err, store.ErrAlreadyUsed) {
		return token, err
	}

	// Another request rotated it first: a retry racing the first, which gets
	// what the first got, or the same replay as above.
	current, err := s.store.RefreshToken(ctx, g.Replaces.ID)
	if err != nil {
		return "", err
	}
	again, err := s.retried(ctx, current, now)
	if err != nil || again != "" {
		return again, err
	}
	if err := s.store.RevokeRefreshFamily(ctx, g.Replaces.FamilyID, now); err != nil {
		return "", err
	}
	return "", authreq.OAuthError(authreq.ErrInvalidGrant, "the refresh token has been revoked")
}

// maxRetryHops bounds how far retried follows a chain of replacements.
const maxRetryHops = 8

// retried is the refresh token a replaced one is answered with when it comes
// within the grace: the token that replaced it, or, if that was itself
// replaced within the grace, its replacement, as long as one is still live.
// "" when there is none, which makes the presentation a replay.
func (s *Service) retried(ctx context.Context, replaced *model.RefreshToken, now time.Time) (string, error) {
	current := replaced
	for range maxRetryHops {
		if !current.InGrace(now) {
			// Read past its grace, a sealed copy is of no more use.
			if len(current.SuccessorSealed) > 0 {
				if err := s.store.ForgetRefreshSuccessor(ctx, current.ID); err != nil {
					return "", err
				}
			}
			return "", nil
		}

		next, err := s.store.RefreshToken(ctx, *current.SuccessorID)
		if errors.Is(err, store.ErrNotFound) {
			return "", nil
		}
		if err != nil {
			return "", err
		}

		if next.Usable(now) {
			plain, err := s.sealer.OpenFor(current.SuccessorWhere(), current.SuccessorSealed)
			if err != nil {
				return "", err
			}
			return string(plain), nil
		}
		current = next
	}

	return "", nil
}

// tokenType is Bearer, or DPoP for a token bound to a key.
func tokenType(dpopKey string) string {
	if dpopKey != "" {
		return TokenTypeDPoP
	}
	return "Bearer"
}

// refreshBinding is what a new family is bound to, of the key the request
// proved: a public client's, since a confidential one is bound by its own
// credentials (RFC 9449 §5).
func refreshBinding(g authreq.Grant, proved string) string {
	if g.App.Profile.Public() {
		return proved
	}
	return ""
}

// The claims an access token names what it was issued in by: the browser
// session, and the refresh family. Revoking either ends the token at this
// server's endpoints.
const (
	claimSession = "sid"
	claimFamily  = "fid"
)

type accessClaims struct {
	Issuer   string      `json:"iss"`
	Subject  string      `json:"sub"`
	Audience []string    `json:"aud"`
	ClientID string      `json:"client_id"`
	Scope    string      `json:"scope"`
	IssuedAt json.Number `json:"iat"`
	Expiry   json.Number `json:"exp"`
	JTI      string      `json:"jti"`
	Session  string      `json:"sid"`
	Family   string      `json:"fid"`

	ExchangedFrom   []string `json:"exchanged_from"`
	SubjectClientID string   `json:"subject_client_id"`
}

// revocationKeys are the keys an access token is looked up by in the
// revocations: itself, its session, its family, and its user's grant.
func (c accessClaims) revocationKeys() []string {
	keys := []string{model.RevokedToken(c.JTI)}
	if id, err := uuid.Parse(c.Session); err == nil {
		keys = append(keys, model.RevokedSession(id))
	}
	if id, err := uuid.Parse(c.Family); err == nil {
		keys = append(keys, model.RevokedFamily(id))
	}
	if user, err := uuid.Parse(c.Subject); err == nil && c.Subject != c.ClientID && c.Subject != c.SubjectClientID {
		keys = append(keys, model.RevokedGrant(user, c.ClientID), model.RevokedUser(user))
		if c.SubjectClientID != "" {
			keys = append(keys, model.RevokedGrant(user, c.SubjectClientID))
		}
	}
	// An exchanged token ends with every token it was exchanged from.
	for _, jti := range c.ExchangedFrom {
		keys = append(keys, model.RevokedToken(jti))
	}
	return keys
}

// verifyAccessToken checks an access token this server issued: signed by one of
// its keys, an access token and not an ID token, from this issuer, unexpired.
func (s *Service) verifyAccessToken(ctx context.Context, token string) (*accessClaims, map[string]any, error) {
	var raw map[string]any
	header, err := jose.Verify(token, s.keys.Lookup(ctx), &raw)
	if err != nil || header.Type != "at+jwt" {
		return nil, nil, jose.ErrInvalid
	}

	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, nil, err
	}

	var claims accessClaims
	if err := json.Unmarshal(encoded, &claims); err != nil {
		return nil, nil, jose.ErrInvalid
	}

	exp, err := claims.Expiry.Int64()
	if err != nil || claims.Issuer != s.issuer || s.now().Unix() >= exp {
		return nil, nil, jose.ErrInvalid
	}

	iat, err := claims.IssuedAt.Int64()
	if err != nil {
		return nil, nil, jose.ErrInvalid
	}
	revoked, err := s.store.AccessTokenRevoked(ctx, claims.revocationKeys(), time.Unix(iat, 0))
	if err != nil {
		return nil, nil, err
	}
	if revoked {
		return nil, nil, jose.ErrInvalid
	}

	return &claims, raw, nil
}

func (s *Service) UserInfo(ctx context.Context, accessToken string, how Presentation) (map[string]any, error) {
	if accessToken == "" {
		return nil, authreq.OAuthError(authreq.ErrInvalidToken, "an access token is required")
	}

	claims, raw, err := s.verifyAccessToken(ctx, accessToken)
	if err != nil {
		return nil, authreq.OAuthError(authreq.ErrInvalidToken, "the access token is invalid or has expired")
	}
	if err := s.presented(ctx, accessToken, raw, how, true); err != nil {
		return nil, err
	}

	scopes := strings.Fields(claims.Scope)
	if !slices.Contains(scopes, model.ScopeOpenID) {
		return nil, authreq.OAuthError(authreq.ErrInsufficientScope, "the access token was not granted the openid scope")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, authreq.OAuthError(authreq.ErrInvalidToken, "the access token is not for a user")
	}

	app, err := s.store.ApplicationByClientID(ctx, claims.ClientID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, authreq.OAuthError(authreq.ErrInvalidToken, "the application no longer exists")
	}
	if err != nil {
		return nil, err
	}

	user, err := s.store.User(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, authreq.OAuthError(authreq.ErrInvalidToken, "the user no longer exists")
	}
	if err != nil {
		return nil, err
	}

	evaluated, err := s.evaluate(ctx, app, user, scopes, "")
	if err != nil {
		return nil, err
	}
	if !evaluated.Issued || evaluated.IDToken == nil {
		return nil, authreq.OAuthError(authreq.ErrInvalidToken, evaluated.Reason)
	}

	info := evaluated.IDToken
	for _, claim := range []string{"iss", "aud", "iat", "exp"} {
		delete(info, claim)
	}

	return info, nil
}

// Revoke revokes a refresh token and its family, or one access token (RFC
// 7009). An access token is a JWT an API may check on its own, so it is
// refused from then on by this server's endpoints and lives at most its
// lifetime elsewhere. An unknown token still succeeds, as the RFC requires.
func (s *Service) Revoke(ctx context.Context, client ClientAuth, token string) error {
	app, err := s.authenticate(ctx, client)
	if err != nil {
		return err
	}

	if token == "" {
		return authreq.OAuthError(authreq.ErrInvalidRequest, "token is required")
	}

	if claims, _, err := s.verifyAccessToken(ctx, token); err == nil {
		// Another client's token is left alone, and the answer does not
		// say it exists.
		if claims.ClientID != app.ClientID || claims.JTI == "" {
			return nil
		}
		return s.store.RevokeAccessToken(ctx, claims.JTI, s.now())
	}

	refresh, err := s.store.RefreshTokenByHash(ctx, model.HashSecret(token))
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	// Another client's token is left alone, and the answer does not say it
	// exists.
	if refresh.ApplicationID != app.ID {
		return nil
	}

	return s.store.RevokeRefreshFamily(ctx, refresh.FamilyID, s.now())
}

// Introspect reports whether a token is active (RFC 7662). Only confidential
// clients may ask, and only about their own tokens; any other client's token
// reads as inactive.
func (s *Service) Introspect(ctx context.Context, client ClientAuth, token string) (map[string]any, error) {
	_, answer, err := s.introspect(ctx, client, token)
	return answer, err
}

// IntrospectionJWTType is the typ of a signed introspection answer, and the
// media type a resource server asks for it with (RFC 9701).
const IntrospectionJWTType = "token-introspection+jwt"

// IntrospectAsJWT is Introspect's answer signed by this server (RFC 9701),
// for a resource server that would rather verify the answer than trust the
// connection it came over: addressed to the client that asked, under
// token_introspection.
func (s *Service) IntrospectAsJWT(ctx context.Context, client ClientAuth, token string) (string, error) {
	app, answer, err := s.introspect(ctx, client, token)
	if err != nil {
		return "", err
	}

	key, ok := s.keys.Signing(model.AlgRS256)
	if !ok {
		return "", errors.New("oauth: no signing key for " + model.AlgRS256)
	}
	return jose.Sign(key, IntrospectionJWTType, map[string]any{
		"iss":                 s.issuer,
		"aud":                 app.ClientID,
		"iat":                 s.now().Unix(),
		"token_introspection": answer,
	})
}

func (s *Service) introspect(ctx context.Context, client ClientAuth, token string) (*model.Application, map[string]any, error) {
	app, err := s.authenticate(ctx, client)
	if err != nil {
		return nil, nil, err
	}
	if !app.Confidential() {
		return nil, nil, authreq.OAuthError(authreq.ErrInvalidClient, "introspection needs a confidential client")
	}

	inactive := map[string]any{"active": false}
	if token == "" {
		return app, inactive, nil
	}

	if claims, raw, err := s.verifyAccessToken(ctx, token); err == nil {
		// Another client's token reads as inactive, unless the caller is
		// listed for an API the token is for: the answer carries the
		// subject, scope and roles, so only the API's own backend, named by
		// an administrator, may read them.
		if claims.ClientID != app.ClientID {
			listed, err := s.store.MayIntrospect(ctx, app.ID, claims.Audience)
			if err != nil {
				return nil, nil, err
			}
			if !listed {
				return app, inactive, nil
			}
		}

		// A user turned off or deleted since is no longer vouched for, as
		// userinfo and the refresh grant already say; a service token's
		// subject is the client itself.
		if claims.Subject != claims.ClientID {
			id, err := uuid.Parse(claims.Subject)
			if err != nil {
				return app, inactive, nil
			}
			user, err := s.store.User(ctx, id)
			if errors.Is(err, store.ErrNotFound) {
				return app, inactive, nil
			}
			if err != nil {
				return nil, nil, err
			}
			if !user.IsActive {
				return app, inactive, nil
			}
		}

		out := map[string]any{"active": true, "token_type": tokenType(boundKey(raw))}
		maps.Copy(out, raw)
		return app, out, nil
	}

	refresh, err := s.store.RefreshTokenByHash(ctx, model.HashSecret(token))
	if errors.Is(err, store.ErrNotFound) {
		return app, inactive, nil
	}
	if err != nil {
		return nil, nil, err
	}

	if refresh.ApplicationID != app.ID || !refresh.Usable(s.now()) {
		return app, inactive, nil
	}

	// A token for a user who has since been turned off is spent, not live:
	// the refresh grant itself would refuse it, so introspection says so too.
	// Only a user who is gone or off: a database that does not answer is an
	// error the caller can retry, not every token reported dead.
	user, err := s.store.User(ctx, refresh.UserID)
	if errors.Is(err, store.ErrNotFound) {
		return app, inactive, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if !user.IsActive {
		return app, inactive, nil
	}

	return app, map[string]any{
		"active":     true,
		"token_type": "refresh_token",
		"client_id":  app.ClientID,
		"sub":        refresh.UserID.String(),
		"scope":      refresh.Scope,
		"exp":        refresh.Lasts().Unix(),
		"iat":        refresh.CreatedAt.Unix(),
		"iss":        s.issuer,
	}, nil
}

func scopeOf(claims map[string]any) string {
	scope, _ := claims["scope"].(string)
	return scope
}

// idleUntil is when a refresh token issued now ends unused: the
// application's idle lifetime from now, never past the family's end.
func idleUntil(now time.Time, app *model.Application, familyEnds time.Time) *time.Time {
	until := now.Add(time.Duration(app.RefreshTokenIdleLifetime) * time.Second)
	if app.RefreshTokenIdleLifetime <= 0 || until.After(familyEnds) {
		until = familyEnds
	}
	return &until
}
