// Package infrastructure adapts the outside world to the domain. store is
// the Postgres persistence every service is handed; database owns the
// connection and runs the migrations; cache is Redis; mail sends; passkey
// speaks WebAuthn; signing holds the keys; outbound is the federation
// client that refuses more than the domain asks.
//
// These packages depend on the domain, never on an application use case or
// on transport — swap Postgres for something else and only this layer
// should feel it.
package infrastructure
