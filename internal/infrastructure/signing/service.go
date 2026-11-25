// Package signing holds the provider's signing keys: loaded at startup,
// rotated on schedule or by an administrator, and published as the JWKS.
package signing

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"time"

	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
)

type Service struct {
	keys *KeySet
	log  *slog.Logger
}

// New gives the sealer the installation's tag key, then loads or makes the
// signing keys, so a wrong VARD_SECRET_KEY stops startup here.
func New(ctx context.Context, st *store.Store, sealer *jose.Sealer, rotation time.Duration, log *slog.Logger) (*Service, error) {
	if err := loadTagKey(ctx, st, sealer); err != nil {
		return nil, err
	}

	keys, err := loadKeys(ctx, st, sealer, rotation, log)
	if err != nil {
		return nil, err
	}

	return &Service{keys: keys, log: log}, nil
}

// Keys is the key set the services that sign and verify tokens hold.
func (s *Service) Keys() *KeySet {
	return s.keys
}

// loadTagKey gives the sealer the installation's own key for keyed hashes,
// making it the first time.
func loadTagKey(ctx context.Context, st *store.Store, sealer *jose.Sealer) error {
	where := model.ServerSecret{Name: model.ServerSecretTag}.ValueWhere()
	sealed, err := st.ServerSecret(ctx, model.ServerSecretTag, func() ([]byte, error) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		return sealer.SealFor(where, key)
	})
	if err != nil {
		return err
	}

	key, err := sealer.OpenFor(where, sealed)
	if err != nil {
		return fmt.Errorf("the installation's tag key: %w", err)
	}
	sealer.UseTagKey(key)

	return nil
}
