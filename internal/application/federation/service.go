// Package federation signs people in through somebody else: social
// providers, and enterprise SSO over OpenID Connect and SAML, with the
// domains an SSO connection proves it holds.
package federation

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/a1i6ek/vard/internal/application/authreq"
	"github.com/a1i6ek/vard/internal/application/oauth"
	"github.com/a1i6ek/vard/internal/application/signin"
	"github.com/a1i6ek/vard/internal/infrastructure/outbound"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
	"github.com/a1i6ek/vard/internal/platform/config"
	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
)

type Service struct {
	store      *store.Store
	log        *slog.Logger
	issuer     string
	accountURL string

	sealer   *jose.Sealer
	social   *http.Client
	ssoCache *ssoCache

	// lookupTXT reads a domain's TXT records, for proving SSO domains.
	lookupTXT func(ctx context.Context, name string) ([]string, error)

	authreq *authreq.Service
	oauth   *oauth.Service
	signin  *signin.Service

	now func() time.Time
}

func New(cfg config.Config, st *store.Store, sealer *jose.Sealer, log *slog.Logger, requests *authreq.Service, protocol *oauth.Service, signIn *signin.Service) *Service {
	return &Service{
		store:      st,
		log:        log,
		issuer:     cfg.Issuer,
		accountURL: cfg.AccountURL,
		sealer:     sealer,
		social:     outbound.NewFederationClient(socialTimeout, outbound.OnLoopback(cfg.Issuer)),
		ssoCache:   newSSOCache(),
		lookupTXT:  lookupTXT,
		authreq:    requests,
		oauth:      protocol,
		signin:     signIn,
		now:        time.Now,
	}
}
