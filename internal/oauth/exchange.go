package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/crypto/jose"
	"github.com/a1i6ek/vard/internal/model"
	"github.com/a1i6ek/vard/internal/store"
)

// The claims an exchanged token carries beside RFC 8693's act: the tokens it
// was exchanged from, and the client the subject first signed in to, so
// revoking any of them, or that client's grant, ends it too.
const (
	claimExchangedFrom   = "exchanged_from"
	claimSubjectClientID = "subject_client_id"
)

// Caller is who sent a request, for the activity log.
type Caller struct {
	IP        string
	UserAgent string
}

// exchangeSubject is what a subject token (RFC 8693 §2.1) says, once
// checked.
type exchangeSubject struct {
	subject       string
	user          *model.User
	app           *model.Application
	originClient  string
	scopes        []string
	expires       time.Time
	act           any
	session       string
	family        string
	exchangedFrom []string
}

var errNoExchangeRule = authreq.OAuthError(authreq.ErrUnauthorizedClient,
	"no rule lets this application exchange that application's tokens for this API")

// exchangeToken is RFC 8693's token exchange: a confidential client acting
// for the subject of a token another client was issued, for one API. Only a
// rule an administrator made allows it, and the token issued holds no more
// than the subject token, lives no longer, and ends with it.
func (s *Service) exchangeToken(ctx context.Context, app *model.Application, p TokenParams) (*TokenResponse, error) {
	switch {
	case !app.Confidential():
		return nil, authreq.OAuthError(authreq.ErrUnauthorizedClient, "a public client cannot exchange tokens")
	case p.SubjectToken == "" || p.SubjectTokenType == "":
		return nil, authreq.OAuthError(authreq.ErrInvalidRequest, "subject_token and subject_token_type are required")
	case p.RequestedTokenType != "" && p.RequestedTokenType != model.TokenTypeAccess:
		return nil, authreq.OAuthError(authreq.ErrInvalidRequest, "requested_token_type: only "+model.TokenTypeAccess+" is issued")
	case (p.ActorToken == "") != (p.ActorTokenType == ""):
		return nil, authreq.OAuthError(authreq.ErrInvalidRequest, "actor_token and actor_token_type are sent together")
	case p.Resource == "":
		return nil, authreq.OAuthError(authreq.ErrInvalidTarget, "resource is required: name the API the token is for")
	}

	subject, err := s.exchangeSubject(ctx, p.SubjectToken, p.SubjectTokenType)
	if err != nil {
		return nil, err
	}
	if p.ActorToken != "" {
		if err := s.checkActor(ctx, app, p.ActorToken, p.ActorTokenType); err != nil {
			return nil, err
		}
	}

	rule, api, err := s.store.TokenExchangeRule(ctx, p.Resource, app.ID, subject.app.ID)
	switch {
	case errors.Is(err, store.ErrNotFound) && api == nil:
		return nil, errUnknownResource
	case errors.Is(err, store.ErrNotFound):
		return nil, errNoExchangeRule
	case err != nil:
		return nil, err
	}

	if model.DelegationDepth(subject.act) >= model.MaxDelegationDepth {
		return nil, authreq.OAuthError(authreq.ErrInvalidRequest, "the subject token was already exchanged as many times as a token may be")
	}

	var roles []model.UserRole
	if subject.user != nil {
		if roles, err = s.store.EffectiveRoles(ctx, subject.user); err != nil {
			return nil, err
		}
	}
	allowed := model.ExchangeScopes(*api, *rule, subject.scopes, subject.user, roles)
	granted := allowed
	if p.Scope != "" {
		granted = []string{}
		for _, scope := range strings.Fields(p.Scope) {
			if !slices.Contains(allowed, scope) {
				return nil, authreq.OAuthError(authreq.ErrInvalidScope, scope+" is more than the subject token holds or the rule allows")
			}
			if !slices.Contains(granted, scope) {
				granted = append(granted, scope)
			}
		}
	}

	jti, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := s.now()
	expires := now.Add(time.Duration(app.AccessLifetimeFor(api)) * time.Second)
	if subject.expires.Before(expires) {
		expires = subject.expires
	}

	act := map[string]any{"sub": app.ClientID}
	if subject.act != nil {
		act["act"] = subject.act
	}
	access := map[string]any{
		"iss":                s.issuer,
		"sub":                subject.subject,
		"aud":                []string{api.Identifier},
		"azp":                app.ClientID,
		"client_id":          app.ClientID,
		"iat":                now.Unix(),
		"exp":                expires.Unix(),
		"jti":                jti.String(),
		"scope":              strings.Join(granted, " "),
		"act":                act,
		claimSubjectClientID: subject.originClient,
	}
	if len(subject.exchangedFrom) > 0 {
		access[claimExchangedFrom] = subject.exchangedFrom
	}
	if subject.session != "" {
		access[claimSession] = subject.session
	}
	if subject.family != "" {
		access[claimFamily] = subject.family
	}
	if p.dpopKey != "" {
		access["cnf"] = map[string]any{"jkt": p.dpopKey}
	}

	key, ok := s.keys.Signing(api.AccessTokenAlgorithm())
	if !ok {
		return nil, errors.New("oauth: no signing key for " + api.AccessTokenAlgorithm())
	}
	token, err := jose.Sign(key, "at+jwt", access)
	if err != nil {
		return nil, err
	}

	s.recordExchange(ctx, app, api, subject, granted, p.From)

	return &TokenResponse{
		AccessToken:     token,
		IssuedTokenType: model.TokenTypeAccess,
		TokenType:       tokenType(p.dpopKey),
		ExpiresIn:       expires.Unix() - now.Unix(),
		Scope:           strings.Join(granted, " "),
	}, nil
}

var errBadSubject = authreq.OAuthError(authreq.ErrInvalidRequest, "subject_token is invalid, expired or revoked")

// exchangeSubject checks a subject token: an access token this server
// issued, or an ID token, live and not revoked.
func (s *Service) exchangeSubject(ctx context.Context, token, kind string) (*exchangeSubject, error) {
	var subject *exchangeSubject
	var err error
	switch kind {
	case model.TokenTypeAccess:
		subject, err = s.accessSubject(ctx, token)
	case model.TokenTypeID:
		subject, err = s.idTokenSubject(ctx, token)
	default:
		return nil, authreq.OAuthError(authreq.ErrInvalidRequest, "subject_token_type must be "+model.TokenTypeAccess+" or "+model.TokenTypeID)
	}
	if err != nil {
		return nil, err
	}
	// A turned-off application's tokens speak for nobody any more.
	if !subject.app.IsEnabled {
		return nil, errBadSubject
	}

	// A user's token speaks for them only while they are active.
	if subject.subject != subject.originClient {
		id, err := uuid.Parse(subject.subject)
		if err != nil {
			return nil, errBadSubject
		}
		user, err := s.store.User(ctx, id)
		if errors.Is(err, store.ErrNotFound) {
			return nil, errBadSubject
		}
		if err != nil {
			return nil, err
		}
		if !user.IsActive {
			return nil, errBadSubject
		}
		subject.user = user
	}
	return subject, nil
}

func (s *Service) accessSubject(ctx context.Context, token string) (*exchangeSubject, error) {
	claims, raw, err := s.verifyAccessToken(ctx, token)
	if errors.Is(err, jose.ErrInvalid) {
		return nil, errBadSubject
	}
	if err != nil {
		return nil, err
	}

	app, err := s.store.ApplicationByClientID(ctx, claims.ClientID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, errBadSubject
	}
	if err != nil {
		return nil, err
	}

	exp, _ := claims.Expiry.Int64()
	origin := claims.SubjectClientID
	if origin == "" {
		origin = claims.ClientID
	}
	return &exchangeSubject{
		subject:       claims.Subject,
		app:           app,
		originClient:  origin,
		scopes:        strings.Fields(claims.Scope),
		expires:       time.Unix(exp, 0),
		act:           raw["act"],
		session:       claims.Session,
		family:        claims.Family,
		exchangedFrom: append(slices.Clone(claims.ExchangedFrom), claims.JTI),
	}, nil
}

// idTokenSubject checks an ID token this server issued: it names the user
// and the client they signed in to, and holds no scope, so a token
// exchanged from it holds none either.
func (s *Service) idTokenSubject(ctx context.Context, token string) (*exchangeSubject, error) {
	var claims struct {
		Issuer   string          `json:"iss"`
		Subject  string          `json:"sub"`
		Audience json.RawMessage `json:"aud"`
		Party    string          `json:"azp"`
		IssuedAt int64           `json:"iat"`
		Expiry   int64           `json:"exp"`
		Session  string          `json:"sid"`
	}
	header, err := jose.Verify(token, s.keys.Lookup(ctx), &claims)
	if err != nil || header.Type != IDTokenType || claims.Issuer != s.issuer || s.now().Unix() >= claims.Expiry {
		return nil, errBadSubject
	}

	clientID := claims.Party
	if clientID == "" {
		_ = json.Unmarshal(claims.Audience, &clientID)
	}
	user, err := uuid.Parse(claims.Subject)
	if err != nil || clientID == "" {
		return nil, errBadSubject
	}

	keys := []string{model.RevokedGrant(user, clientID), model.RevokedUser(user)}
	if session, err := uuid.Parse(claims.Session); err == nil {
		keys = append(keys, model.RevokedSession(session))
	}
	revoked, err := s.store.AccessTokenRevoked(ctx, keys, time.Unix(claims.IssuedAt, 0))
	if err != nil {
		return nil, err
	}
	if revoked {
		return nil, errBadSubject
	}

	app, err := s.store.ApplicationByClientID(ctx, clientID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, errBadSubject
	}
	if err != nil {
		return nil, err
	}

	return &exchangeSubject{
		subject:      claims.Subject,
		app:          app,
		originClient: clientID,
		scopes:       []string{},
		expires:      time.Unix(claims.Expiry, 0),
		session:      claims.Session,
	}, nil
}

// checkActor checks an actor token (RFC 8693 §2.1): one the acting client
// was issued for itself, live. The client has already authenticated; the
// token only has to agree.
func (s *Service) checkActor(ctx context.Context, app *model.Application, token, kind string) error {
	bad := authreq.OAuthError(authreq.ErrInvalidRequest, "actor_token must be a live access token this client was issued for itself")
	if kind != model.TokenTypeAccess {
		return bad
	}
	claims, _, err := s.verifyAccessToken(ctx, token)
	if errors.Is(err, jose.ErrInvalid) {
		return bad
	}
	if err != nil {
		return err
	}
	if claims.ClientID != app.ClientID || claims.Subject != app.ClientID {
		return bad
	}
	return nil
}

// recordExchange writes the exchange to the activity log, under the acting
// client's id, as a token-authenticated call to the admin API is.
func (s *Service) recordExchange(ctx context.Context, app *model.Application, api *model.API, subject *exchangeSubject, scopes []string, from Caller) {
	metadata := map[string]any{
		"application":    app.Name,
		"api":            api.Identifier,
		"subject_client": subject.originClient,
		"for_client":     subject.app.ClientID,
		"scope":          strings.Join(scopes, " "),
		"depth":          model.DelegationDepth(subject.act) + 1,
	}
	if subject.user != nil {
		metadata["user_id"] = subject.user.ID.String()
	}
	entry := model.AuditLog{
		ActorEmail: app.ClientID,
		Action:     "token.exchanged",
		TargetType: "application",
		TargetID:   app.ID.String(),
		Metadata:   metadata,
		IP:         from.IP,
		UserAgent:  from.UserAgent,
	}
	if err := s.store.WriteAudit(ctx, &entry); err != nil {
		s.log.Error("writing the activity log failed", "error", err, "action", entry.Action)
	}
}
