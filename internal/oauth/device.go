package oauth

import (
	"context"
	"encoding/hex"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/model"
	"github.com/a1i6ek/vard/internal/signin"
	"github.com/a1i6ek/vard/internal/store"
)

// The device authorization grant (RFC 8628).

const (
	userCodePurpose = "device user code"
)

// The answers a polling device gets while it waits (RFC 8628 §3.5).
const (
	ErrAuthorizationPending = "authorization_pending"
	ErrSlowDown             = "slow_down"
	ErrExpiredToken         = "expired_token"
)

// ErrDeviceCodeInvalid is a user code that names no device waiting: mistyped,
// expired or already answered. One answer for all, so it says nothing about
// which codes exist.
var ErrDeviceCodeInvalid = authreq.NewProblem("device_code_invalid")

// ErrAgreementsRequired refuses allowing a device for an application whose
// flow asks for the organization's terms while the person has not accepted
// them: the device page asks, as authorize's Agreements page does.
var ErrAgreementsRequired = authreq.NewProblem("agreements_required")

// ErrDeviceRefused is a device the person allowed but cannot use: the
// application's rules give their account no token. The device is refused.
var ErrDeviceRefused = authreq.NewProblem("device_refused")

// DeviceStart is the answer to a device asking to sign someone in.
type DeviceStart struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// StartDevice begins a device's sign-in.
func (s *Service) StartDevice(ctx context.Context, client ClientAuth, scope, audience string, from Caller) (*DeviceStart, error) {
	app, err := s.authenticate(ctx, client)
	if err != nil {
		return nil, err
	}

	switch {
	case !app.IsEnabled:
		return nil, authreq.OAuthError(authreq.ErrUnauthorizedClient, "the application is disabled")
	case !slices.Contains(app.GrantTypes, model.GrantDeviceCode):
		return nil, authreq.OAuthError(authreq.ErrUnauthorizedClient, "the application does not have the device code grant")
	case strings.TrimSpace(scope) == "":
		return nil, authreq.OAuthError(authreq.ErrInvalidScope, "scope is required")
	}

	if audience != "" {
		if _, err := s.store.APIByIdentifier(ctx, audience); errors.Is(err, store.ErrNotFound) {
			return nil, errUnknownResource
		} else if err != nil {
			return nil, err
		}
	}

	deviceCode, deviceHash, err := model.NewSecret()
	if err != nil {
		return nil, err
	}

	// A code already waiting is drawn again; with this many, rarely more than once.
	for range 5 {
		userCode, err := model.NewUserCode()
		if err != nil {
			return nil, err
		}

		device := model.DeviceAuthorization{
			DeviceCodeHash: deviceHash,
			UserCodeHash:   s.userCodeHash(userCode),
			ApplicationID:  app.ID,
			Scope:          strings.Join(strings.Fields(scope), " "),
			Audience:       audience,
			RequestIP:      model.Truncate(from.IP, 45),
			RequestAgent:   model.Truncate(from.UserAgent, 255),
			Status:         model.DevicePending,
			Interval:       model.DeviceInterval,
			ExpiresAt:      s.now().Add(model.DeviceAuthorizationLifetime),
		}
		err = s.store.CreateDeviceAuthorization(ctx, &device)
		if errors.Is(err, store.ErrDuplicate) {
			continue
		}
		if err != nil {
			return nil, err
		}

		verify := s.accountURL + authreq.PageDevice
		return &DeviceStart{
			DeviceCode:              deviceCode,
			UserCode:                userCode,
			VerificationURI:         verify,
			VerificationURIComplete: authreq.WithQuery(verify, url.Values{"user_code": {userCode}}),
			ExpiresIn:               int(model.DeviceAuthorizationLifetime.Seconds()),
			Interval:                model.DeviceInterval,
		}, nil
	}

	return nil, errors.New("oidc: no free user code after five tries")
}

// deviceToken is the device asking whether the person has answered yet.
func (s *Service) deviceToken(ctx context.Context, app *model.Application, p TokenParams) (*TokenResponse, error) {
	if p.DeviceCode == "" {
		return nil, authreq.OAuthError(authreq.ErrInvalidRequest, "device_code is required")
	}

	device, err := s.store.DeviceByCode(ctx, model.HashSecret(p.DeviceCode))
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the device code is not valid")
	case err != nil:
		return nil, err
	case device.ApplicationID != app.ID:
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the device code was issued to another client")
	}

	now := s.now()

	switch {
	case device.UsedAt != nil:
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the device code has already been used")
	case device.Status == model.DeviceDenied:
		return nil, authreq.OAuthError(authreq.ErrAccessDenied, "the user did not allow the device")
	case !now.Before(device.ExpiresAt):
		return nil, authreq.OAuthError(ErrExpiredToken, "the device code has expired; start again")
	case device.Status == model.DevicePending:
		tooSoon := device.LastPolledAt != nil && now.Sub(*device.LastPolledAt) < time.Duration(device.Interval)*time.Second
		if err := s.store.PolledDevice(ctx, device.ID, now, tooSoon); err != nil {
			return nil, err
		}
		if tooSoon {
			return nil, authreq.OAuthError(ErrSlowDown, "ask less often")
		}
		return nil, authreq.OAuthError(ErrAuthorizationPending, "the user has not answered yet")
	}

	claimed, err := s.store.ClaimDevice(ctx, device.ID, now)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the device code has already been used")
	}

	user, err := s.store.User(ctx, *device.UserID)
	if errors.Is(err, store.ErrNotFound) || err == nil && !user.CanSignIn() {
		return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the user can no longer sign in")
	}
	if err != nil {
		return nil, err
	}

	// The session the device was allowed in must still be alive, as for a
	// code: signing out, or a password reset, between the approval and the
	// poll must not leave the device a fresh refresh-token family. Checked
	// after the claim, so a refused device stays spent.
	if device.SessionID != nil {
		session, err := s.store.UserSession(ctx, *device.SessionID)
		switch {
		case errors.Is(err, store.ErrNotFound):
			return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the session the device was allowed in has ended")
		case err != nil:
			return nil, err
		case !session.Active(now):
			return nil, authreq.OAuthError(authreq.ErrInvalidGrant, "the session the device was allowed in has ended")
		}
	}

	response, err := s.issue(ctx, authreq.Grant{
		App:      app,
		User:     user,
		Scopes:   strings.Fields(device.Scope),
		Audience: device.Audience,
		AuthTime: *device.AuthenticatedAt,
		Methods:  device.Methods,
		Session:  device.SessionID,
		DPoPKey:  p.dpopKey,
		Refusal:  authreq.ErrInvalidGrant,
	})
	if err != nil {
		return nil, err
	}

	if device.SessionID != nil && response.IDToken != "" {
		if err := s.store.RememberSessionApplication(ctx, *device.SessionID, app.ID); err != nil {
			s.log.Error("recording the application a session signed in failed", "error", err)
		}
	}

	return response, nil
}

// DevicePage is what the person sees before allowing a device: the
// application, what it asks for, where the device asked from, and whether
// allowing it also accepts the organization's terms.
type DevicePage struct {
	ConsentPage
	Agreements    bool         `json:"agreements"`
	RequestedFrom DeviceOrigin `json:"requested_from"`
}

// DeviceOrigin is where a device asked to be signed in from: its address,
// the user agent it sent, and when.
type DeviceOrigin struct {
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	At        time.Time `json:"at"`
}

// DeviceFor is what the person sees before allowing a device.
func (s *Service) DeviceFor(ctx context.Context, userCode string, session *signin.Session) (*DevicePage, error) {
	if !session.InBrowser() {
		return nil, authreq.ErrBrowserRequired
	}

	device, err := s.waitingDevice(ctx, userCode)
	if err != nil {
		return nil, err
	}

	described, err := s.describeRequest(ctx, device.Application, device.Scope, device.Audience)
	if err != nil {
		return nil, err
	}

	flow, err := s.store.EffectiveLoginFlow(ctx, device.Application)
	if err != nil {
		return nil, err
	}
	needed, err := s.agreementsNeeded(ctx, flow, session.User)
	if err != nil {
		return nil, err
	}

	return &DevicePage{
		ConsentPage:   *described,
		Agreements:    needed,
		RequestedFrom: DeviceOrigin{IP: device.RequestIP, UserAgent: device.RequestAgent, At: device.CreatedAt},
	}, nil
}

// AnswerDevice allows or refuses a device for the signed-in person. Allowing
// it is checked as a sign-in would be, so a user who could not get a token is
// refused here rather than by the device's next ask. acceptTerms is the
// person accepting the organization's terms with it, which a flow with the
// Agreements step asks for while they have not.
func (s *Service) AnswerDevice(ctx context.Context, userCode string, session *signin.Session, allow, acceptTerms bool, client authreq.Client) error {
	if !session.InBrowser() {
		return authreq.ErrBrowserRequired
	}

	device, err := s.waitingDevice(ctx, userCode)
	if err != nil {
		return err
	}

	refused := false
	if allow {
		// Allowing a device is a sign-in to its application, held to that
		// application's flow like any other.
		if err := s.sessionMeetsFlowOf(ctx, device.Application, session); err != nil {
			return err
		}
		if err := s.deviceAgreements(ctx, device.Application, session, acceptTerms, client); err != nil {
			return err
		}

		token, err := s.evaluate(ctx, device.Application, session.User, strings.Fields(device.Scope), device.Audience)
		if err != nil {
			return err
		}
		refused = !token.Issued
		allow = token.Issued
	}

	answered, err := s.store.AnswerDevice(ctx, device, allow, session.User.ID, &session.Record, s.now())
	if err != nil {
		return err
	}
	if !answered {
		return ErrDeviceCodeInvalid
	}

	action := "user.device_denied"
	if allow {
		action = "user.device_allowed"
	}
	s.authreq.Record(ctx, session.User, session.User.Email, action, client, map[string]any{"application": device.Application.Name})

	if refused {
		return ErrDeviceRefused
	}

	return nil
}

// deviceAgreements holds allowing a device to the Agreements step of its
// application's flow, as finish holds a code to it: refused until the terms
// are accepted, and accepted here when the person said so.
func (s *Service) deviceAgreements(ctx context.Context, app *model.Application, session *signin.Session, accept bool, client authreq.Client) error {
	flow, err := s.store.EffectiveLoginFlow(ctx, app)
	if err != nil {
		return err
	}
	needed, err := s.agreementsNeeded(ctx, flow, session.User)
	if err != nil || !needed {
		return err
	}
	if !accept {
		return ErrAgreementsRequired
	}

	if err := s.store.AcceptAgreements(ctx, session.User, s.now()); err != nil {
		return err
	}
	s.authreq.Record(ctx, session.User, session.User.Email, "user.agreements_accepted", client, map[string]any{"application": app.Name})

	return nil
}

func (s *Service) waitingDevice(ctx context.Context, userCode string) (*model.DeviceAuthorization, error) {
	device, err := s.store.DeviceByUserCode(ctx, s.userCodeHash(userCode), s.now())
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, ErrDeviceCodeInvalid
	case err != nil:
		return nil, err
	case device.Application == nil || !device.Application.IsEnabled:
		return nil, ErrDeviceCodeInvalid
	}

	return device, nil
}

// userCodeHash keys the code with the server's secret: eight letters are far
// too few for a bare hash to keep.
func (s *Service) userCodeHash(code string) string {
	return hex.EncodeToString(s.sealer.Tag(userCodePurpose, []byte(model.NormalizeUserCode(code))))
}

// ErrStepUpRequired asks a signed-in person to pass a second factor in their
// session (or set one up) before allowing a device for an application whose
// flow requires one: POST /api/v1/account/step-up starts it.
var ErrStepUpRequired = authreq.NewProblem("step_up_required")

// sessionMeetsFlowOf says whether the session meets the application's flow,
// as authorize asks it: ErrSignInAgain or ErrStepUpRequired when it does not.
func (s *Service) sessionMeetsFlowOf(ctx context.Context, app *model.Application, session *signin.Session) error {
	flow, err := s.store.EffectiveLoginFlow(ctx, app)
	if err != nil {
		return err
	}

	switch sessionMeets(flow, session, s.now()) {
	case sessionSignInAgain:
		return signin.ErrSignInAgain
	case sessionStepUp:
		return ErrStepUpRequired
	}

	return nil
}

// StepUpForDevice holds a step-up of the browser's session for the
// application a device asks for, and returns the handle for the browser's
// cookie; "" when the session needs none. The person passes a second factor,
// or sets one up, on the second-factor page and comes back to allow the
// device.
func (s *Service) StepUpForDevice(ctx context.Context, userCode string, session *signin.Session, client authreq.Client) (string, error) {
	if !session.InBrowser() {
		return "", authreq.ErrBrowserRequired
	}

	device, err := s.waitingDevice(ctx, userCode)
	if err != nil {
		return "", err
	}

	err = s.sessionMeetsFlowOf(ctx, device.Application, session)
	switch {
	case err == nil:
		return "", nil
	case !errors.Is(err, ErrStepUpRequired):
		return "", err
	}

	// Setting a factor up needs a proved address: the email goes first.
	has, err := s.store.HasUserFactor(ctx, session.User.ID)
	if err != nil {
		return "", err
	}
	if !has && !session.User.IsEmailVerified {
		if err := s.signin.SendVerification(ctx, session.User, "", client); err != nil {
			return "", err
		}
		return "", authreq.ErrEmailNotVerified
	}

	return s.signin.WaitForStepUp(ctx, session, "", &device.Application.ID, &device.ID)
}
