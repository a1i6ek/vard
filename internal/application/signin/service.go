// Package signin is how users sign themselves in, and what they do with
// their own account: sessions and sign-in flows, passwords, registration,
// resets and email verification, login codes, second factors and passkeys,
// and the throttle every answer is counted against. Administrators sign in
// through internal/admin instead.
//
// It knows nothing of HTTP: handlers in api call it.
package signin

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/a1i6ek/vard/internal/application/authreq"
	"github.com/a1i6ek/vard/internal/platform/config"
	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
	"github.com/a1i6ek/vard/internal/infrastructure/mail"
	"github.com/a1i6ek/vard/internal/infrastructure/outbound"
	"github.com/a1i6ek/vard/internal/infrastructure/passkey"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

type Service struct {
	store      *store.Store
	mail       mail.Sender
	log        *slog.Logger
	accountURL string

	sealer   *jose.Sealer
	webauthn *webauthn.WebAuthn

	// pwnedURL is the breached-password range API, "" when the check is
	// off; breachClient reaches it, held to the outbound rules.
	pwnedURL     string
	breachClient *http.Client

	authreq *authreq.Service

	now func() time.Time
}

func New(cfg config.Config, st *store.Store, sealer *jose.Sealer, mailer mail.Sender, log *slog.Logger, requests *authreq.Service) (*Service, error) {
	passkeys, err := passkey.RelyingParty(cfg.AccountURL, cfg.PasskeyRPID)
	if err != nil {
		return nil, err
	}

	return &Service{
		store:        st,
		mail:         mailer,
		log:          log,
		accountURL:   cfg.AccountURL,
		sealer:       sealer,
		webauthn:     passkeys,
		pwnedURL:     cfg.PwnedPasswordsURL,
		breachClient: outbound.NewFederationClient(breachTimeout, outbound.OnLoopback(cfg.Issuer)),
		authreq:      requests,
		now:          time.Now,
	}, nil
}
