package federation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/a1i6ek/vard/internal/application/authreq"
	"github.com/a1i6ek/vard/internal/application/signin"
	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

// The account a social sign-in lands on: the one already linked to the
// identity, one with the same verified address, or a new one where the flow
// allows it.

func (s *Service) signInWithIdentity(
	ctx context.Context,
	provider *model.SocialProvider,
	who *socialIdentity,
	request string,
	client authreq.Client,
) (*signin.SignInResult, error) {
	now := s.now()
	email := strings.ToLower(strings.TrimSpace(who.Email))

	flow, err := s.signin.FlowFor(ctx, request)
	if err != nil {
		return nil, err
	}

	// Closed means closed before any account is made, identity linked or role
	// synced — StartSession would refuse at the end, but only after those.
	if !flow.AllowSignIn {
		return nil, authreq.ErrSignInClosed
	}

	identity, err := s.store.SocialIdentity(ctx, provider.ID, who.Subject)
	switch {
	case err == nil:
		user, err := s.store.User(ctx, identity.UserID)
		if err != nil {
			return nil, err
		}
		// The account's own address decides SSO enforcement, not the
		// provider's.
		if err := s.socialSSORequired(ctx, user.Email); err != nil {
			return nil, err
		}
		if !user.CanSignIn() {
			s.authreq.Record(ctx, user, user.Email, "user.login_blocked", client, map[string]any{
				"reason": "inactive", "provider": provider.Name,
			})
			return nil, authreq.ErrInvalidCredentials
		}

		if err := s.store.MarkIdentityUsed(ctx, identity, email, now); err != nil {
			return nil, err
		}

		return s.startSocialSession(ctx, user, provider, flow, request, client)
	case !errors.Is(err, store.ErrNotFound):
		return nil, err
	}

	if email == "" {
		return nil, authreq.ErrSocialNoEmail
	}

	if err := s.socialSSORequired(ctx, email); err != nil {
		return nil, err
	}

	// Linking to an existing account needs the address proved by the provider,
	// the provider allowed to link (LinksByAddress), and the account's own
	// address verified; otherwise whoever pre-registered the address would keep a
	// password to it.
	user, err := s.store.UserByEmail(ctx, email)
	switch {
	case err == nil:
		if !who.EmailVerified || !provider.LinksByAddress() || !user.IsEmailVerified {
			return nil, authreq.ErrSocialLinkRefused
		}
		if !user.CanSignIn() {
			return nil, authreq.ErrInvalidCredentials
		}

		if err := s.connectIdentity(ctx, user, provider, who, email, now); err != nil {
			return nil, err
		}
		s.authreq.Record(ctx, user, user.Email, "user.identity_connected", client, map[string]any{
			"provider": provider.Name,
		})

		return s.startSocialSession(ctx, user, provider, flow, request, client)
	case !errors.Is(err, store.ErrNotFound):
		return nil, err
	}

	if !provider.AllowRegistration || !flow.AllowRegistration {
		return nil, authreq.ErrSocialRegistrationClosed
	}
	// An account made for an address nobody proved would be held by whoever
	// controls the provider's account, before the address's owner arrives.
	if !who.EmailVerified {
		return nil, authreq.ErrSocialEmailUnverified
	}
	if err := s.socialRegistrationAllowed(ctx, request); err != nil {
		return nil, err
	}

	user = &model.User{
		Email:           email,
		IsEmailVerified: true, // the provider said so: who.EmailVerified, above
		FirstName:       model.Truncate(who.FirstName, 100),
		LastName:        model.Truncate(who.LastName, 100),
		IsActive:        true,
		Data:            map[string]any{},
	}

	roles, err := s.store.DefaultUserRoles(ctx)
	if err != nil {
		return nil, err
	}
	user.Roles = roles

	if err := s.store.CreateUser(ctx, user); errors.Is(err, store.ErrDuplicate) {
		return nil, authreq.ErrSocialLinkRefused
	} else if err != nil {
		return nil, err
	}

	if err := s.connectIdentity(ctx, user, provider, who, email, now); err != nil {
		return nil, err
	}

	s.authreq.Record(ctx, user, user.Email, "user.registered", client, map[string]any{"provider": provider.Name})

	// As a password registration does. A flow that requires the address
	// mails the link itself as it refuses the session (Admit), so it is
	// sent here only when it would not be.
	if !user.IsEmailVerified && flow.VerifyEmailOnRegister && !flow.RequireVerifiedEmail {
		if err := s.signin.SendVerification(ctx, user, request, client); err != nil {
			s.log.Error("sending a verification email failed", "error", err, "user", user.ID)
		}
	}

	return s.startSocialSession(ctx, user, provider, flow, request, client)
}

func (s *Service) socialSSORequired(ctx context.Context, email string) error {
	err := s.signin.SSORequiredFor(ctx, email)

	if _, ok := errors.AsType[*signin.SSORequired](err); ok {
		return authreq.ErrSocialSSORequired
	}

	return err
}

func (s *Service) socialRegistrationAllowed(ctx context.Context, request string) error {
	if request == "" {
		return nil
	}

	// The application named decides, so a request that cannot be read makes
	// no account: an expired one is a sign-in to start again, anything else
	// a failure, never a way past a closed registration.
	req, err := s.authreq.Pending(ctx, request)
	switch {
	case errors.Is(err, authreq.ErrRequestExpired):
		return authreq.ErrSocialExpired
	case err != nil:
		return err
	}

	if !req.Application.AllowRegistration || !req.Application.IsEnabled {
		return authreq.ErrSocialRegistrationClosed
	}

	return nil
}

func (s *Service) connectIdentity(
	ctx context.Context,
	user *model.User,
	provider *model.SocialProvider,
	who *socialIdentity,
	email string,
	now time.Time,
) error {
	identity := &model.SocialIdentity{
		UserID:      user.ID,
		ProviderID:  provider.ID,
		Subject:     who.Subject,
		Email:       email,
		LastLoginAt: &now,
	}

	return s.store.CreateSocialIdentity(ctx, identity)
}

func (s *Service) startSocialSession(
	ctx context.Context,
	user *model.User,
	provider *model.SocialProvider,
	flow *model.LoginFlow,
	request string,
	client authreq.Client,
) (*signin.SignInResult, error) {
	result, err := s.signin.StartSession(ctx, user, flow, request, true, nil, client, "user.login")
	if err != nil {
		return nil, err
	}

	s.log.Info("a user signed in with a social provider",
		"provider", provider.Slug, "user", user.ID)

	return result, nil
}
