package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/crypto/jose"
	"github.com/a1i6ek/vard/internal/model"
)

// ClientAssertionAlgorithms are what a client assertion may be signed with.
var ClientAssertionAlgorithms = []string{jose.ES256, jose.PS256, jose.RS256}

// ClientAssertionType is the only client_assertion_type taken (RFC 7523 §2.2).
const ClientAssertionType = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"

const (
	// assertionLeeway is how far apart the client's clock and this one may
	// be when an assertion's times are compared.
	assertionLeeway = 30 * time.Second
	maxJTILength    = 256
)

type assertionClaims struct {
	Issuer    string          `json:"iss"`
	Subject   string          `json:"sub"`
	Audience  json.RawMessage `json:"aud"`
	ExpiresAt *json.Number    `json:"exp"`
	NotBefore *json.Number    `json:"nbf"`
	IssuedAt  *json.Number    `json:"iat"`
	ID        string          `json:"jti"`
}

// assertedClient is who an assertion says it is from, read before anything
// is verified, only to find the client whose keys verify it.
func assertedClient(assertion string) string {
	parts := strings.Split(assertion, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Issuer string `json:"iss"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	return claims.Issuer
}

// verifyAssertion authenticates a private_key_jwt client (RFC 7523 §3,
// OpenID Connect Core §9): signed by one of its registered keys with ES256,
// PS256 or RS256; iss and sub its client_id; addressed to this server; not
// expired, and expiring within ClientAssertionLifetime; and its jti never
// used before. The jti is spent only once everything else holds, so a
// forged assertion cannot use up a real one's.
func (s *Service) verifyAssertion(ctx context.Context, app *model.Application, assertion, endpoint string) error {
	refused := func(why string) error {
		return authreq.OAuthError(authreq.ErrInvalidClient, why)
	}

	var claims assertionClaims
	if err := s.verifyClientSignature(ctx, app, assertion, &claims); err != nil {
		s.log.Info("a client assertion did not verify", "client_id", app.ClientID, "error", err)
		return refused("the client assertion is not signed by one of the client's keys")
	}

	if claims.Issuer != app.ClientID || claims.Subject != app.ClientID {
		return refused("the client assertion's iss and sub must both be the client_id")
	}
	if !s.addressedHere(claims.Audience, endpoint) {
		return refused("the client assertion is not addressed to this server: aud must be the issuer")
	}

	now := s.now()
	expires, ok := unixTime(claims.ExpiresAt)
	switch {
	case !ok:
		return refused("the client assertion has no exp")
	case !now.Before(expires.Add(assertionLeeway)):
		return refused("the client assertion has expired")
	case expires.After(now.Add(model.ClientAssertionLifetime + assertionLeeway)):
		return refused("the client assertion expires too far ahead: at most five minutes")
	}
	if notBefore, ok := unixTime(claims.NotBefore); ok && notBefore.After(now.Add(assertionLeeway)) {
		return refused("the client assertion is not valid yet")
	}
	if issued, ok := unixTime(claims.IssuedAt); ok && issued.After(now.Add(assertionLeeway)) {
		return refused("the client assertion was issued in the future")
	}
	if claims.ID == "" || len(claims.ID) > maxJTILength {
		return refused("the client assertion needs a jti")
	}

	seen := sha256.Sum256([]byte(app.ClientID + " " + claims.ID))
	fresh, err := s.store.UseClientAssertion(ctx, hex.EncodeToString(seen[:]), expires.Add(assertionLeeway))
	if err != nil {
		return err
	}
	if !fresh {
		return refused("the client assertion has already been used")
	}

	return nil
}

// verifyClientSignature checks the assertion against the client's inline
// keys, or the ones at its jwks_uri, fetched again at most once a minute for
// one naming a key not held (outbound.KeySets).
func (s *Service) verifyClientSignature(ctx context.Context, app *model.Application, assertion string, claims *assertionClaims) error {
	if app.JWKSURI != "" {
		return s.clientKeys.Verify(ctx, s.outbound, s.now, app.JWKSURI, assertion, claims)
	}

	var keys []jose.Key
	for _, jwk := range app.JWKS {
		if key, err := jose.ParseJWK(jwk); err == nil {
			keys = append(keys, key)
		}
	}

	_, err := jose.VerifyAmong(assertion, func(header jose.Header) []jose.Key {
		var out []jose.Key
		for _, key := range keys {
			if header.KeyID == "" || key.ID == header.KeyID {
				out = append(out, key)
			}
		}
		return out
	}, claims)
	return err
}

// addressedHere is whether aud names this server: its issuer (RFC 7523bis),
// or, as clients written before it do, the token endpoint's address or that
// of the endpoint the assertion was sent to (RFC 9126 §2).
func (s *Service) addressedHere(raw json.RawMessage, endpoint string) bool {
	var audiences []string
	var one string
	if json.Unmarshal(raw, &one) == nil {
		audiences = []string{one}
	} else if json.Unmarshal(raw, &audiences) != nil {
		return false
	}

	accepted := []string{s.issuer, s.issuer + authreq.PathToken}
	if endpoint != "" {
		accepted = append(accepted, s.issuer+endpoint)
	}
	for _, audience := range audiences {
		if slices.Contains(accepted, audience) {
			return true
		}
	}
	return false
}

func unixTime(n *json.Number) (time.Time, bool) {
	if n == nil {
		return time.Time{}, false
	}
	seconds, err := n.Float64()
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(int64(seconds), 0), true
}
