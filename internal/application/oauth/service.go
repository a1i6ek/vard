// Package oauth is the OAuth 2.0 authorization server: authorize, pushed
// requests, the token endpoint, the device flow, DPoP, dynamic registration,
// introspection and revocation, and the Consent and Agreements pages a flow
// passes through. What a token carries is decided by model.EvaluateToken
// alone.
package oauth

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/a1i6ek/vard/internal/application/authreq"
	"github.com/a1i6ek/vard/internal/platform/config"
	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
	"github.com/a1i6ek/vard/internal/infrastructure/outbound"
	"github.com/a1i6ek/vard/internal/application/signin"
	"github.com/a1i6ek/vard/internal/infrastructure/signing"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

// A client's published keys are held for an hour, and fetched within
// fifteen seconds.
const (
	clientKeysLifetime = time.Hour
	clientKeysTimeout  = 15 * time.Second
)

type Service struct {
	store      *store.Store
	keys       *signing.KeySet
	log        *slog.Logger
	issuer     string
	accountURL string

	sealer *jose.Sealer

	// requirePushed refuses every authorization request not pushed first,
	// whatever the application says (VARD_REQUIRE_PAR).
	requirePushed bool

	// outbound reaches a client's jwks_uri, held to the outbound rules;
	// clientKeys holds what it found there.
	outbound   *http.Client
	clientKeys *outbound.KeySets

	authreq *authreq.Service
	signin  *signin.Service

	now func() time.Time
}

func New(cfg config.Config, st *store.Store, sealer *jose.Sealer, keys *signing.KeySet, log *slog.Logger, requests *authreq.Service, signIn *signin.Service) *Service {
	return &Service{
		store:         st,
		keys:          keys,
		log:           log,
		issuer:        cfg.Issuer,
		accountURL:    cfg.AccountURL,
		sealer:        sealer,
		requirePushed: cfg.RequirePushedRequests,
		outbound:      outbound.NewFederationClient(clientKeysTimeout, outbound.OnLoopback(cfg.Issuer)),
		clientKeys:    outbound.NewKeySets(clientKeysLifetime, clientKeysTimeout),
		authreq:       requests,
		signin:        signIn,
		now:           time.Now,
	}
}
