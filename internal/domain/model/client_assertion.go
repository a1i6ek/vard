package model

import "time"

// ClientAssertionLifetime is the furthest ahead a client assertion's exp may
// be (RFC 7523 §3): long enough for a clock that runs a little apart, short
// enough that its jti need not be remembered for long.
const ClientAssertionLifetime = 5 * time.Minute

// ClientAssertion is a private_key_jwt assertion already used: the client
// and its jti, hashed, so the same assertion never authenticates twice
// (RFC 7523 §3, item 7). It is kept until the assertion would have expired.
type ClientAssertion struct {
	Base

	Hash      string    `gorm:"size:64;not null;uniqueIndex"`
	ExpiresAt time.Time `gorm:"not null;index"`
}

func (ClientAssertion) TableName() string {
	return "client_assertions"
}
