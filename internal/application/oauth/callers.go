package oauth

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/a1i6ek/vard/internal/application/authreq"
	"github.com/a1i6ek/vard/internal/platform/brand"
	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/application/signin"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

var ErrTokenRefused = errors.New("oidc: the access token is not accepted by this API")

type AdminCaller struct {
	Application *model.Application
	Permissions []string
}

// AdminCaller checks a token presented to the admin API. It must be for the
// admin API and a service's token; a user's token is refused whatever its
// scopes.
func (s *Service) AdminCaller(ctx context.Context, token string, how Presentation) (*AdminCaller, error) {
	claims, app, err := s.caller(ctx, token, how, brand.AdminAPIIdentifier)
	if err != nil {
		return nil, err
	}
	if claims.Subject != claims.ClientID {
		return nil, ErrTokenRefused
	}

	audience, err := s.store.AudienceFor(ctx, app.ID, brand.AdminAPIIdentifier)
	if err != nil {
		return nil, err
	}
	if !audience.Authorized {
		return nil, ErrTokenRefused
	}

	catalog := model.AdminPermissionNames()
	permissions := []string{}
	for scope := range strings.FieldsSeq(claims.Scope) {
		if slices.Contains(audience.Allowed, scope) && slices.Contains(catalog, scope) {
			permissions = append(permissions, scope)
		}
	}

	return &AdminCaller{Application: app, Permissions: permissions}, nil
}

type AccountCaller struct {
	Session  *signin.Session
	ClientID string
	Scopes   []string
}

func (s *Service) AccountCaller(ctx context.Context, token string, how Presentation) (*AccountCaller, error) {
	claims, app, err := s.caller(ctx, token, how, brand.AccountAPIIdentifier)
	if err != nil {
		return nil, err
	}

	// Access and scopes are rechecked on every call, so revoking them applies
	// at once.
	audience, err := s.store.AudienceFor(ctx, app.ID, brand.AccountAPIIdentifier)
	if err != nil {
		return nil, err
	}
	if !audience.Authorized {
		return nil, ErrTokenRefused
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil || claims.Subject == claims.ClientID {
		return nil, ErrTokenRefused
	}

	user, err := s.store.User(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrTokenRefused
	}
	if err != nil {
		return nil, err
	}
	// Active, and not unlocked: a token was issued to a signed-in user, and
	// the throttle on wrong passwords is on signing in (see SessionFor).
	if !user.IsActive {
		return nil, ErrTokenRefused
	}

	scopes := []string{}
	for scope := range strings.FieldsSeq(claims.Scope) {
		if (scope == model.ScopeAccountRead || scope == model.ScopeAccountWrite) && slices.Contains(audience.Allowed, scope) {
			scopes = append(scopes, scope)
		}
	}

	return &AccountCaller{Session: &signin.Session{User: user}, ClientID: claims.ClientID, Scopes: scopes}, nil
}

func (s *Service) caller(ctx context.Context, token string, how Presentation, audience string) (*accessClaims, *model.Application, error) {
	claims, raw, err := s.verifyAccessToken(ctx, token)
	if err != nil || !slices.Contains(claims.Audience, audience) {
		return nil, nil, ErrTokenRefused
	}

	// A token bound to a key is taken only as DPoP, with a proof from that
	// key for this request (RFC 9449 §7), never as a plain bearer token,
	// which would undo the binding; one not bound is taken only as Bearer.
	if err := s.presented(ctx, token, raw, how, false); err != nil {
		var refused *authreq.Error
		if errors.As(err, &refused) {
			return nil, nil, ErrTokenRefused
		}
		return nil, nil, err
	}

	app, err := s.store.ApplicationByClientID(ctx, claims.ClientID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil, ErrTokenRefused
	}
	if err != nil {
		return nil, nil, err
	}
	if !app.IsEnabled {
		return nil, nil, ErrTokenRefused
	}

	return claims, app, nil
}
