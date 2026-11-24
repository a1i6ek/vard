package signin

import (
	"context"
	"errors"

	"github.com/a1i6ek/vard/internal/application/authreq"
	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

type SSORequired struct {
	SSOButton
}

func (e *SSORequired) Error() string { return authreq.ErrSSORequired.Error() }

func (e *SSORequired) Unwrap() error { return authreq.ErrSSORequired }

// SSOButton is a connection as a sign-in page offers it.
type SSOButton struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
}

func NewSSOButton(connection *model.SSOConnection) SSOButton {
	return SSOButton{Slug: connection.Slug, Name: connection.Name, Protocol: string(connection.Protocol)}
}

func (s *Service) SSORequiredFor(ctx context.Context, email string) error {
	connection, err := s.store.SSOConnectionForEmail(ctx, email)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil
	case err != nil:
		return err
	case !connection.EnforceDomains:
		return nil
	}

	return &SSORequired{NewSSOButton(connection)}
}

type SSOPerson struct {
	Subject   string
	Email     string
	FirstName string
	LastName  string
	Groups    []string

	// Proved is what the provider says its sign-in proved, read only from
	// what its signature covers: an id_token's amr, or the verified SAML
	// assertion's AuthnContextClassRef values. A connection that trusts its
	// provider's second factor matches it against its own list.
	Proved []string

	Sent []string
}
