// Package services builds the provider's services, each from those below
// it, for the handlers to take what they call.
package services

import (
	"context"
	"log/slog"

	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/config"
	"github.com/a1i6ek/vard/internal/crypto/jose"
	"github.com/a1i6ek/vard/internal/federation"
	"github.com/a1i6ek/vard/internal/mail"
	"github.com/a1i6ek/vard/internal/oauth"
	"github.com/a1i6ek/vard/internal/oidc"
	"github.com/a1i6ek/vard/internal/signin"
	"github.com/a1i6ek/vard/internal/signing"
	"github.com/a1i6ek/vard/internal/store"
)

// Provider is every service the provider is made of.
type Provider struct {
	Requests   *authreq.Service
	Signing    *signing.Service
	SignIn     *signin.Service
	OAuth      *oauth.Service
	OIDC       *oidc.Service
	Federation *federation.Service
}

// New loads or makes the installation's keys before anything else, so a
// wrong VARD_SECRET_KEY stops startup here.
func New(ctx context.Context, cfg config.Config, st *store.Store, sealer *jose.Sealer, mailer mail.Sender, log *slog.Logger) (*Provider, error) {
	keys, err := signing.New(ctx, st, sealer, cfg.KeyRotation, log)
	if err != nil {
		return nil, err
	}

	requests := authreq.New(st, cfg.Issuer, cfg.AccountURL, log)

	signIn, err := signin.New(cfg, st, sealer, mailer, log, requests)
	if err != nil {
		return nil, err
	}

	protocol := oauth.New(cfg, st, sealer, keys.Keys(), log, requests, signIn)

	return &Provider{
		Requests:   requests,
		Signing:    keys,
		SignIn:     signIn,
		OAuth:      protocol,
		OIDC:       oidc.New(cfg, st, keys.Keys(), log, requests, signIn),
		Federation: federation.New(cfg, st, sealer, log, requests, protocol, signIn),
	}, nil
}
