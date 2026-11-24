package federation

import (
	"context"
	"fmt"
	"time"

	"github.com/a1i6ek/vard/internal/domain/model"
)

// samlRefreshLook is how often the worker looks for metadata due a read.
const samlRefreshLook = 10 * time.Minute

// metadataRefreshFailed is the activity log's word for a read that failed
// where the last one had not.
const metadataRefreshFailed = "sso_connection.metadata_refresh_failed"

// KeepMetadataFresh reads each SAML connection's metadata again from its
// address once a day, until ctx ends: a provider rolls its certificate over
// in it, and a connection holding the old one stops signing anybody in. A
// read that fails, or that names another provider, keeps the last good
// copy, is tried again within the hour, and is written to the activity log
// the first time. Each read is claimed once across instances
// (ClaimMetadataCheck).
func (s *Service) KeepMetadataFresh(ctx context.Context) {
	look := time.NewTicker(samlRefreshLook)
	defer look.Stop()

	for {
		s.RefreshDueMetadata(ctx)
		select {
		case <-ctx.Done():
			return
		case <-look.C:
		}
	}
}

// RefreshDueMetadata is one pass of KeepMetadataFresh: every connection due
// a read, read.
func (s *Service) RefreshDueMetadata(ctx context.Context) {
	due, err := s.store.SAMLMetadataDue(ctx, s.now())
	if err != nil {
		if ctx.Err() == nil {
			s.log.Error("listing SAML metadata to refresh failed", "error", err)
		}
		return
	}

	for i := range due {
		if ctx.Err() != nil {
			return
		}
		s.refreshMetadata(ctx, &due[i])
	}
}

func (s *Service) refreshMetadata(ctx context.Context, connection *model.SSOConnection) {
	now := s.now()
	claimed, err := s.store.ClaimMetadataCheck(ctx, connection, now)
	if err != nil || !claimed {
		return
	}

	metadata, err := s.FetchSAMLMetadata(ctx, connection.MetadataURL)
	if err == nil {
		err = sameProvider(connection.Metadata, metadata)
	}

	failure := ""
	if err != nil {
		failure = err.Error()
	}
	if err := s.store.MetadataRead(ctx, connection.ID, metadata, failure, now); err != nil {
		s.log.Error("recording a SAML metadata read failed", "error", err, "connection", connection.Slug)
		return
	}
	if failure == "" || connection.MetadataError != "" {
		return
	}

	s.log.Warn("refreshing SAML metadata failed; the last good copy is kept", "connection", connection.Slug, "error", failure)
	entry := &model.AuditLog{
		Action:     metadataRefreshFailed,
		TargetType: "sso_connection",
		TargetID:   connection.ID.String(),
		Metadata:   map[string]any{"connection": connection.Name, "reason": failure},
	}
	if err := s.store.WriteAudit(ctx, entry); err != nil {
		s.log.Error("writing the activity log failed", "error", err, "action", metadataRefreshFailed)
	}
}

// sameProvider refuses metadata naming another provider than the copy held:
// accepting it would hand the connection, and every identity at it, to
// whoever now answers at the address. An administrator does that by hand.
func sameProvider(held, read string) error {
	before, err := ParseSAMLMetadata([]byte(held))
	if err != nil {
		return nil
	}
	after, err := ParseSAMLMetadata([]byte(read))
	if err != nil {
		return err
	}
	if after.EntityID != before.EntityID {
		return fmt.Errorf("the metadata now names another provider (%s); refresh it by hand to accept it", after.EntityID)
	}
	return nil
}
