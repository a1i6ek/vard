// Package infrastructure adapts the outside world to the domain: store
// (every query), database (connection, migrations), cache (Redis), mail,
// passkey (WebAuthn), signing (keys) and outbound (the guarded HTTP client).
// It depends on the domain only.
package infrastructure
