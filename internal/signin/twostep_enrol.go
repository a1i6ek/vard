package signin

import (
	"context"

	"github.com/go-webauthn/webauthn/protocol"

	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/model"
)

// Setting up a second factor while signing in, for an application whose flow
// requires one, by a person who has none yet. The waiting sign-in stands in
// for the account page's password: its first factor was proved minutes ago,
// and it is named only by an httpOnly cookie, so an application's token never
// reaches it. Setting the factor up answers it: the sign-in finishes with the
// factor as its second.

// BeginEnrolAuthenticator starts setting up an authenticator app in a
// waiting sign-in.
func (s *Service) BeginEnrolAuthenticator(ctx context.Context, handle string, current *Session, client authreq.Client) (*AuthenticatorSetup, error) {
	owner, factors, err := s.enrolling(ctx, handle, current, client)
	if err != nil {
		return nil, err
	}

	return s.startAuthenticator(ctx, owner, factors)
}

// EnrolAuthenticator finishes setting up the authenticator with a code from
// it, and with it the sign-in. The result carries the recovery codes, the
// only time they are shown.
func (s *Service) EnrolAuthenticator(ctx context.Context, handle string, current *Session, code string, client authreq.Client) (*SignInResult, error) {
	owner, _, err := s.enrolling(ctx, handle, current, client)
	if err != nil {
		return nil, err
	}

	factor, err := s.checkAuthenticator(ctx, owner, code)
	if err != nil {
		return nil, err
	}

	return s.finishEnrolment(ctx, owner, current, factor, MethodAuthenticator, codeAMR, client)
}

// BeginEnrolPasskey starts adding a passkey in a waiting sign-in.
func (s *Service) BeginEnrolPasskey(ctx context.Context, handle string, current *Session, client authreq.Client) (*protocol.PublicKeyCredentialCreationOptions, error) {
	owner, factors, err := s.enrolling(ctx, handle, current, client)
	if err != nil {
		return nil, err
	}

	return s.startPasskey(ctx, owner, factors)
}

// EnrolPasskey keeps the passkey the browser made, and finishes the sign-in.
func (s *Service) EnrolPasskey(ctx context.Context, handle string, current *Session, label string, response []byte, client authreq.Client) (*SignInResult, error) {
	owner, _, err := s.enrolling(ctx, handle, current, client)
	if err != nil {
		return nil, err
	}

	factor, proved, err := s.checkNewPasskey(ctx, owner, label, response)
	if err != nil {
		return nil, err
	}

	return s.finishEnrolment(ctx, owner, current, factor, MethodPasskey, proved, client)
}

// enrolling is the waiting sign-in a factor may be set up in: one this
// browser may finish, for a flow that requires a second factor, by an account
// that has none, at a proved address. A sign-in holding a single sign-on
// identity to link is not one: that link waits for a factor the account
// already had.
func (s *Service) enrolling(ctx context.Context, handle string, current *Session, client authreq.Client) (factorOwner, []model.UserFactor, error) {
	pending, err := s.signInFor(ctx, handle, current)
	if err != nil {
		return factorOwner{}, nil, err
	}

	user := pending.User
	if !user.CanSignIn() {
		return factorOwner{}, nil, authreq.ErrInvalidCredentials
	}
	if pending.LinkConnectionID != nil {
		return factorOwner{}, nil, ErrTwoStepNotOffered
	}

	flow, err := s.flowOf(ctx, pending)
	if err != nil {
		return factorOwner{}, nil, err
	}
	if !flow.RequireSecondFactor || !flow.Offers(model.StepTOTP) {
		return factorOwner{}, nil, ErrTwoStepNotOffered
	}

	factors, err := s.confirmedFactors(ctx, user.ID)
	if err != nil {
		return factorOwner{}, nil, err
	}
	if len(factors) > 0 {
		return factorOwner{}, nil, ErrTwoStepNotOffered
	}

	if !user.IsEmailVerified {
		if err := s.SendVerification(ctx, user, pending.Request, client); err != nil {
			return factorOwner{}, nil, err
		}
		return factorOwner{}, nil, authreq.ErrEmailNotVerified
	}

	return factorOwner{user: user, signIn: pending}, factors, nil
}

// finishEnrolment spends the waiting sign-in, keeps the factor, and opens the
// session — or, for a step-up, adds the factor to the session the browser
// has — with the new factor as the sign-in's second.
func (s *Service) finishEnrolment(ctx context.Context, owner factorOwner, current *Session, factor *model.UserFactor, method string, proved []string, client authreq.Client) (*SignInResult, error) {
	pending := owner.signIn

	used, err := s.store.ConsumePendingSignIn(ctx, pending, s.now())
	if err != nil {
		return nil, err
	}
	if !used {
		return nil, ErrSecondFactorExpired
	}

	codes, err := s.confirmFactor(ctx, owner, factor, client)
	if err != nil {
		return nil, err
	}

	flow, err := s.flowOf(ctx, pending)
	if err != nil {
		return nil, err
	}
	if err := s.Admit(ctx, owner.user, flow, pending.Request, client); err != nil {
		return nil, err
	}

	var result *SignInResult
	if pending.StepUpSessionID != nil {
		result, err = s.stepUp(ctx, current, pending.Request, proved, method, client)
	} else {
		methods := model.WithSecondFactor(pending.Methods, proved...)
		result, err = s.openSession(ctx, owner.user, flow, pending.Request, pending.RememberMe, methods, client, "user.login")
	}
	if err != nil {
		return nil, err
	}

	result.RecoveryCodes = codes
	return result, nil
}
