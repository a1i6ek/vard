package model

import (
	"github.com/google/uuid"
)

// SessionApplication says an application was given an ID token in a session,
// so that ending the session can tell it (OpenID Connect Back-Channel Logout
// 1.0). It goes when either does.
type SessionApplication struct {
	Base

	SessionID     uuid.UUID    `gorm:"type:uuid;not null;uniqueIndex:idx_session_applications_pair"`
	Session       *UserSession `gorm:"constraint:OnDelete:CASCADE"`
	ApplicationID uuid.UUID    `gorm:"type:uuid;not null;uniqueIndex:idx_session_applications_pair;index"`
	Application   *Application `gorm:"constraint:OnDelete:CASCADE"`
}

func (SessionApplication) TableName() string {
	return "session_applications"
}
