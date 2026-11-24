package authreq

import (
	"context"
	"errors"
	"net/url"

	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

func (s *Service) Pending(ctx context.Context, handle string) (*model.AuthorizationRequest, error) {
	if handle == "" {
		return nil, ErrRequestExpired
	}

	req, err := s.store.AuthorizationRequestByHandle(ctx, model.HashSecret(handle))
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, ErrRequestExpired
	case err != nil:
		return nil, err
	}

	if !req.Usable(s.now()) || req.Application == nil {
		return nil, ErrRequestExpired
	}

	return req, nil
}

func (s *Service) RedirectError(redirectURI, state, code, description string) string {
	return WithQuery(redirectURI, url.Values{
		"error":             {code},
		"error_description": {description},
		"state":             {state},
		"iss":               {s.issuer},
	})
}
