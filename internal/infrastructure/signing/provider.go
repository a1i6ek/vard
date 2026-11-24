package signing

import (
	"context"
	"time"

	"github.com/a1i6ek/vard/internal/crypto/jose"
)

func (s *Service) MaintainKeys(ctx context.Context) {
	ticker := time.NewTicker(keyMaintenanceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.keys.maintain(ctx); err != nil && ctx.Err() == nil {
				s.log.Error("maintaining signing keys failed", "error", err)
			}
		}
	}
}

func (s *Service) SigningKeys() []KeyInfo {
	return s.keys.describe()
}

func (s *Service) RotateKeys(ctx context.Context, immediate, revoke bool) error {
	return s.keys.Rotate(ctx, immediate, revoke)
}

// JWKS is the JSON Web Key Set.
type JWKS struct {
	Keys []jose.JWK `json:"keys"`
}

func (s *Service) JWKS(ctx context.Context) (JWKS, error) {
	keys, err := s.keys.public(ctx)
	return JWKS{Keys: keys}, err
}
