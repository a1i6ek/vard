// Package application holds the provider's use cases, layered by ADR 0003:
// authreq (the authorization request every part shares) → signin → oauth →
// oidc and federation, which never import each other; admin is the panel's.
// Services take the store they are given and know nothing of HTTP.
package application
