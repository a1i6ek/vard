package oidc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/application/oauth"
	"github.com/a1i6ek/vard/internal/infrastructure/outbound"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

// OpenID Connect Back-Channel Logout 1.0.
const (
	backchannelLogoutEvent = "http://schemas.openid.net/event/backchannel-logout"
	logoutTokenType        = "logout+jwt"

	// logoutTokenLifetime is short: the token is posted at once and an
	// application only has to check it then.
	logoutTokenLifetime = 2 * time.Minute

	backchannelTimeout = 5 * time.Second

	// undelivered is the activity log's word for a logout no try got through.
	undelivered = "application.logout_undelivered"
	// backchannelSenders caps the posts under way at once, so signing one
	// user out of many sessions does not open a connection for each.
	backchannelSenders = 8
)

// backchannelLook is how often the worker looks for notices due when
// nothing woke it; backchannelLease is how long a claimed notice is left to
// its sender before another instance may take it again.
const (
	backchannelLook  = 2 * time.Second
	backchannelLease = time.Minute
)

// backchannelLogout tells applications, server to server, that sessions they
// signed someone in through have ended. The notices are queued in the
// transaction that ended the session (logout_deliveries); KeepTelling sends
// them, on any instance, until each is taken, refused, or
// model.LogoutRetryFor passes, and writes one never taken to the activity
// log.
type backchannelLogout struct {
	client *http.Client
	// wake is nudged after a commit that queued notices, so they go at once
	// rather than at the next look.
	wake chan struct{}
}

// errRefused is an answer another try would not change.
type errRefused struct{ status int }

func (e errRefused) Error() string { return fmt.Sprintf("the application answered %d", e.status) }

// newBackchannelClient goes through the same address checks as every other
// outbound request to an administrator's URL, and follows no redirect: a
// logout token is for the URI the application registered and nowhere else.
func newBackchannelClient(allowLoopback bool) *http.Client {
	client := outbound.NewFederationClient(backchannelTimeout, allowLoopback)
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return client
}

// wakeTeller is the store's OnLogout: notices were queued, send them now.
func (s *Service) wakeTeller() {
	select {
	case s.backchannel.wake <- struct{}{}:
	default:
	}
}

// KeepTelling sends the notices owed until ctx ends, then waits for the
// posts under way. Safe on several instances at once: each claims its own
// (store.ClaimLogouts).
func (s *Service) KeepTelling(ctx context.Context) {
	look := time.NewTicker(backchannelLook)
	defer look.Stop()

	for {
		for s.tellDue(ctx) {
			if ctx.Err() != nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-s.backchannel.wake:
		case <-look.C:
		}
	}
}

// tellDue sends one batch of the notices due, and says whether it was full,
// so more may be waiting.
func (s *Service) tellDue(ctx context.Context) bool {
	notices, err := s.store.ClaimLogouts(ctx, s.now(), backchannelLease, backchannelSenders)
	if err != nil {
		if ctx.Err() == nil {
			s.log.Error("claiming back-channel logouts failed", "error", err)
		}
		return false
	}

	// A post under way finishes even as the server stops: it is short, and
	// its outcome is what keeps it from being sent twice.
	sending := context.WithoutCancel(ctx)
	var posts sync.WaitGroup
	for _, notice := range notices {
		posts.Go(func() { s.deliverLogout(sending, notice) })
	}
	posts.Wait()

	return len(notices) == backchannelSenders
}

// deliverLogout tries the notice once and records how it went: taken;
// refused, or owed past model.LogoutRetryFor, and so given up and written
// to the activity log; or put off, each time longer.
func (s *Service) deliverLogout(ctx context.Context, notice store.LogoutNotice) {
	now := s.now()
	err := errNoLongerTold
	if notice.Enabled && notice.URI != "" {
		err = s.sendLogout(ctx, notice)
	}

	if err == nil {
		if err := s.store.LogoutDelivered(ctx, notice.ID, s.now()); err != nil {
			s.log.Error("recording a delivered logout failed", "error", err)
		}
		return
	}

	next := now.Add(model.LogoutRetryAfter(notice.Tries))
	retry := !errors.As(err, new(errRefused)) && !errors.Is(err, errNoLongerTold) &&
		next.Before(notice.CreatedAt.Add(model.LogoutRetryFor))
	if retry {
		if err := s.store.RetryLogout(ctx, notice.ID, next, err.Error(), now); err != nil {
			s.log.Error("putting off a logout failed", "error", err)
		}
		return
	}

	s.log.Warn("telling an application of a logout failed",
		"client_id", notice.ClientID, "session", notice.SessionID, "tries", notice.Tries, "error", err)

	entry := &model.AuditLog{
		ActorEmail: notice.Email,
		Action:     undelivered,
		TargetType: "application",
		TargetID:   notice.ApplicationID.String(),
		Metadata: map[string]any{
			"application": notice.Application, "user": notice.UserID.String(),
			"tries": notice.Tries, "reason": err.Error(),
		},
	}
	if err := s.store.LogoutFailed(ctx, notice.ID, err.Error(), entry, now); err != nil {
		s.log.Error("giving up a logout failed", "error", err)
	}
}

// errNoLongerTold is a notice whose application has since been turned off or
// stopped taking back-channel logouts.
var errNoLongerTold = errors.New("the application no longer takes back-channel logouts")

func (s *Service) sendLogout(ctx context.Context, notice store.LogoutNotice) error {
	token, err := s.logoutToken(notice)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, backchannelTimeout)
	defer cancel()

	body := url.Values{"logout_token": {token}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notice.URI, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.backchannel.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))

	// §2.8: 200 says the logout succeeded; 204 is common enough to accept.
	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent:
		return nil
	case resp.StatusCode >= 500:
		return fmt.Errorf("the application answered %d", resp.StatusCode)
	default:
		return errRefused{resp.StatusCode}
	}

}

// logoutToken is the JWT of §2.4: who and which session, for which
// application, with the logout event and never a nonce, signed like an ID
// token but typed so it cannot be taken for one.
func (s *Service) logoutToken(notice store.LogoutNotice) (string, error) {
	jti, err := uuid.NewV7()
	if err != nil {
		return "", err
	}

	algorithm := notice.SigningAlg
	if algorithm == "" {
		algorithm = oauth.IDTokenAlgorithm
	}
	key, ok := s.keys.Signing(algorithm)
	if !ok {
		return "", errors.New("oidc: no signing key for " + algorithm)
	}

	now := s.now()

	return jose.Sign(key, logoutTokenType, map[string]any{
		"iss":    s.issuer,
		"sub":    notice.UserID.String(),
		"aud":    notice.ClientID,
		"iat":    now.Unix(),
		"exp":    now.Add(logoutTokenLifetime).Unix(),
		"jti":    jti.String(),
		"sid":    notice.SessionID.String(),
		"events": map[string]any{backchannelLogoutEvent: map[string]any{}},
	})
}
