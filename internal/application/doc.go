// Package application is where use cases live. Each folder is one bounded
// context of the provider, layered the way ADR 0003 sets out: authreq is
// the shared language of an authorization request, sign-in speaks it to
// walk someone past the factors, oauth turns the result into tokens, and
// oidc and federation project it onto the wire — OpenID Connect and
// federation side by side, neither importing the other. admin is the
// console's own use cases.
//
// A service here owns a slice of behaviour, takes the store it is given,
// and returns domain errors; it knows nothing of HTTP.
package application
