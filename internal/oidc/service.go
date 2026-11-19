// Package oidc is OpenID Connect on top of the authorization server:
// discovery, userinfo, and logout, RP-initiated and back-channel.
package oidc

import (
	"log/slog"
	"time"

	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/config"
	"github.com/a1i6ek/vard/internal/outbound"
	"github.com/a1i6ek/vard/internal/signin"
	"github.com/a1i6ek/vard/internal/signing"
	"github.com/a1i6ek/vard/internal/store"
)

type Service struct {
	store      *store.Store
	keys       *signing.KeySet
	log        *slog.Logger
	issuer     string
	accountURL string

	backchannel *backchannelLogout

	authreq *authreq.Service
	signin  *signin.Service

	now func() time.Time
}

func New(cfg config.Config, st *store.Store, keys *signing.KeySet, log *slog.Logger, requests *authreq.Service, signIn *signin.Service) *Service {
	s := &Service{
		store:      st,
		keys:       keys,
		log:        log,
		issuer:     cfg.Issuer,
		accountURL: cfg.AccountURL,
		backchannel: &backchannelLogout{
			client:   newBackchannelClient(outbound.OnLoopback(cfg.Issuer)),
			stopping: make(chan struct{}),
		},
		authreq: requests,
		signin:  signIn,
		now:     time.Now,
	}

	// Every path that ends sessions goes through the store; it finds the
	// applications to tell and hands them here after the commit.
	st.OnLogout(s.TellApplications)

	return s
}
