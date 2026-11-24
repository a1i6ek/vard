package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/a1i6ek/vard/internal/domain/model"
)

// queueLogoutsIn writes a notice owed to each application, in the
// transaction that ended the sessions.
func queueLogoutsIn(tx *gorm.DB, notices []LogoutNotice, at time.Time) error {
	if len(notices) == 0 {
		return nil
	}

	rows := make([]model.LogoutDelivery, len(notices))
	for i, notice := range notices {
		rows[i] = model.LogoutDelivery{
			ApplicationID: notice.ApplicationID,
			SessionID:     notice.SessionID,
			UserID:        notice.UserID,
			Email:         notice.Email,
			Status:        model.LogoutPending,
			NextTryAt:     at,
			ExpiresAt:     at.Add(model.LogoutKept),
		}
	}
	if err := tx.Omit(clause.Associations).Create(&rows).Error; err != nil {
		return err
	}
	for i := range notices {
		notices[i].ID, notices[i].CreatedAt = rows[i].ID, rows[i].CreatedAt
	}
	return nil
}

// ClaimLogouts takes up to `limit` notices due by `now`, each counted as
// tried and put off by `lease`: another instance skips the rows this one
// locked (SKIP LOCKED) and, once the lease is written, the rows themselves,
// and a notice whose sender stopped mid-try is taken again when the lease
// runs out.
func (s *Store) ClaimLogouts(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]LogoutNotice, error) {
	var claimed []LogoutNotice
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`
			SELECT deliveries.id, deliveries.created_at, deliveries.tries + 1 AS tries,
				deliveries.session_id, deliveries.user_id, deliveries.email,
				apps.id AS application_id, apps.name AS application, apps.client_id, apps.is_enabled AS enabled,
				apps.backchannel_logout_uri AS uri, apps.id_token_signed_response_alg AS signing_alg
			FROM logout_deliveries deliveries
			JOIN applications apps ON apps.id = deliveries.application_id
			WHERE deliveries.status = ? AND deliveries.next_try_at <= ?
			ORDER BY deliveries.next_try_at
			LIMIT ?
			FOR UPDATE OF deliveries SKIP LOCKED`, model.LogoutPending, now, limit).
			Scan(&claimed).Error; err != nil || len(claimed) == 0 {
			return err
		}

		ids := make([]uuid.UUID, len(claimed))
		for i, notice := range claimed {
			ids[i] = notice.ID
		}
		return tx.Model(&model.LogoutDelivery{}).Where("id IN ?", ids).
			Updates(map[string]any{"tries": gorm.Expr("tries + 1"), "next_try_at": now.Add(lease), "updated_at": now}).Error
	})
	return claimed, err
}

// LogoutDelivered records that the application took the notice.
func (s *Store) LogoutDelivered(ctx context.Context, id uuid.UUID, at time.Time) error {
	return s.db.WithContext(ctx).Model(&model.LogoutDelivery{}).Where("id = ? AND status = ?", id, model.LogoutPending).
		Updates(map[string]any{"status": model.LogoutDelivered, "delivered_at": at, "last_error": "", "updated_at": at}).Error
}

// RetryLogout puts the notice off to `next`, saying why this try failed.
func (s *Store) RetryLogout(ctx context.Context, id uuid.UUID, next time.Time, reason string, at time.Time) error {
	return s.db.WithContext(ctx).Model(&model.LogoutDelivery{}).Where("id = ? AND status = ?", id, model.LogoutPending).
		Updates(map[string]any{"next_try_at": next, "last_error": model.Truncate(reason, 255), "updated_at": at}).Error
}

// LogoutFailed gives the notice up, and writes `entry` to the activity log
// in the same transaction, so a failure is recorded once.
func (s *Store) LogoutFailed(ctx context.Context, id uuid.UUID, reason string, entry *model.AuditLog, at time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.LogoutDelivery{}).Where("id = ? AND status = ?", id, model.LogoutPending).
			Updates(map[string]any{"status": model.LogoutFailed, "last_error": model.Truncate(reason, 255), "updated_at": at})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		entry.ActorEmail = model.Truncate(entry.ActorEmail, 255)
		return tx.Create(entry).Error
	})
}

// LogoutDeliveries lists an application's notices, newest first.
func (s *Store) LogoutDeliveries(ctx context.Context, applicationID uuid.UUID, limit int) ([]model.LogoutDelivery, error) {
	var deliveries []model.LogoutDelivery
	err := s.db.WithContext(ctx).Where("application_id = ?", applicationID).
		Order("created_at DESC, id DESC").Limit(limit).Find(&deliveries).Error
	return deliveries, err
}
