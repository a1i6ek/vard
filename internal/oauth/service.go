// Package oauth is the OAuth 2.0 authorization server: authorize, pushed
// requests, the token endpoint, the device flow, DPoP, dynamic registration,
// introspection and revocation, and the Consent and Agreements pages a flow
// passes through. What a token carries is decided by model.EvaluateToken
// alone.
package oauth

import (
	"log/slog"
	"time"

	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/config"
	"github.com/a1i6ek/vard/internal/crypto/jose"
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

	sealer *jose.Sealer

	authreq *authreq.Service
	signin  *signin.Service

	now func() time.Time
}

func New(cfg config.Config, st *store.Store, sealer *jose.Sealer, keys *signing.KeySet, log *slog.Logger, requests *authreq.Service, signIn *signin.Service) *Service {
	return &Service{
		store:      st,
		keys:       keys,
		log:        log,
		issuer:     cfg.Issuer,
		accountURL: cfg.AccountURL,
		sealer:     sealer,
		authreq:    requests,
		signin:     signIn,
		now:        time.Now,
	}
}
