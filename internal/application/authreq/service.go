// Package authreq is what every part of the provider shares: the waiting
// authorization request, the problems answered with, the caller, the paths
// and pages, and applications as a sign-in page shows them. It sits below
// signin and oauth so neither imports the other.
package authreq

import (
	"log/slog"
	"time"

	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

type Service struct {
	store      *store.Store
	log        *slog.Logger
	issuer     string
	accountURL string

	now func() time.Time
}

func New(st *store.Store, issuer, accountURL string, log *slog.Logger) *Service {
	return &Service{store: st, log: log, issuer: issuer, accountURL: accountURL, now: time.Now}
}
