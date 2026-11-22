// Package authreq is what every part of the provider shares: the waiting
// authorization request (reading it, refusing it back to the client, the
// grant it becomes), the problems a person or client is answered with, who
// is calling, the provider's paths and pages, and the organisation and
// applications as a sign-in page shows them.
//
// It sits below sign-in and the protocol, so that neither has to import the
// other to read a request both of them handle.
package authreq

import (
	"log/slog"
	"time"

	"github.com/a1i6ek/vard/internal/store"
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
