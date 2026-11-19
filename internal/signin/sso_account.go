package signin

import (
	"context"

	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/model"
)

// LinkSSOIdentity links a provider's identity to an account, and records it
// with whether the account's own second factor was asked for first.
func (s *Service) LinkSSOIdentity(ctx context.Context, user *model.User, connection string, identity *model.SSOIdentity, stepped bool, client authreq.Client) error {
	if err := s.store.CreateSSOIdentity(ctx, identity); err != nil {
		return err
	}

	s.authreq.Record(ctx, user, user.Email, "user.identity_connected", client, map[string]any{
		"provider": connection, "second_factor": stepped,
	})

	return nil
}
