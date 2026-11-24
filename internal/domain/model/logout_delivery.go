package model

import (
	"time"

	"github.com/google/uuid"
)

// LogoutStatus is where a back-channel logout notice stands.
type LogoutStatus string

const (
	LogoutPending   LogoutStatus = "pending"
	LogoutDelivered LogoutStatus = "delivered"
	LogoutFailed    LogoutStatus = "failed"
)

var LogoutStatuses = []LogoutStatus{LogoutPending, LogoutDelivered, LogoutFailed}

const (
	// LogoutRetryFor is how long a notice the application did not take is
	// tried again, waiting longer each time (LogoutRetryAfter).
	LogoutRetryFor = 24 * time.Hour
	// LogoutKept is how long a notice is kept after it was written, for
	// the panel to show how it went: the tries, and a week after.
	LogoutKept = LogoutRetryFor + 7*24*time.Hour
)

// LogoutRetryAfter is the wait before the next try once `tries` have
// failed: five seconds, doubling, at most an hour.
func LogoutRetryAfter(tries int) time.Duration {
	wait := 5 * time.Second
	for range max(tries-1, 0) {
		wait *= 2
		if wait >= time.Hour {
			return time.Hour
		}
	}
	return wait
}

// LogoutDelivery is a back-channel logout notice (OpenID Connect
// Back-Channel Logout 1.0) owed to an application: written in the
// transaction that ended the session, so it is never lost to a restart,
// then sent by a worker until the application takes it, refuses it, or
// LogoutRetryFor passes. The session and user are not foreign keys: a
// deleted user's applications are still told.
type LogoutDelivery struct {
	Base

	ApplicationID uuid.UUID    `gorm:"type:uuid;not null;index" json:"application_id"`
	Application   *Application `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	SessionID     uuid.UUID    `gorm:"type:uuid;not null" json:"session_id"`
	UserID        uuid.UUID    `gorm:"type:uuid;not null" json:"user_id"`
	// Email is the user's, for the activity log should the notice fail.
	Email string `gorm:"size:255;not null;default:''" json:"-"`

	Status      LogoutStatus `gorm:"size:16;not null;check:chk_logout_deliveries_status,status IN ('pending','delivered','failed');index:idx_logout_deliveries_due,priority:1" json:"status"`
	Tries       int          `gorm:"not null" json:"tries"`
	NextTryAt   time.Time    `gorm:"not null;index:idx_logout_deliveries_due,priority:2" json:"next_try_at"`
	LastError   string       `gorm:"size:255;not null;default:''" json:"last_error"`
	DeliveredAt *time.Time   `json:"delivered_at,omitempty"`
	ExpiresAt   time.Time    `gorm:"not null;index" json:"-"`
}

func (LogoutDelivery) TableName() string {
	return "logout_deliveries"
}
