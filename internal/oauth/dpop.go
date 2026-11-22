package oauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/crypto/jose"
	"github.com/a1i6ek/vard/internal/model"
)

// DPoP (RFC 9449): a client proves, with each request, that it holds a key,
// and the tokens it gets are bound to that key. An access token carries the
// key's thumbprint in cnf.jkt and is accepted, at userinfo, only with a proof
// from that key; a public client's refresh tokens need one too. A copied token
// is no use without the key.

const (
	ErrInvalidDPoPProof = "invalid_dpop_proof"
	// ErrUseDPoPNonce asks the client to send its proof again with the nonce
	// in the DPoP-Nonce header (RFC 9449 §8, §9).
	ErrUseDPoPNonce = "use_dpop_nonce"
	TokenTypeDPoP   = "DPoP"

	dpopProofType = "dpop+jwt"
	maxDPoPJTI    = 256
)

// DPoPAlgorithms are the proof signatures accepted, as the discovery
// document lists them.
var DPoPAlgorithms = []string{jose.ES256, jose.RS256, jose.PS256}

// Presentation is how a protected request carried its token: the scheme of
// its Authorization header, its DPoP proof if any, and what it was for.
type Presentation struct {
	Scheme string
	Proof  string
	Method string
	URL    string
}

type dpopClaims struct {
	JTI         string      `json:"jti"`
	Method      string      `json:"htm"`
	Target      string      `json:"htu"`
	IssuedAt    json.Number `json:"iat"`
	AccessToken string      `json:"ath"`
	Nonce       string      `json:"nonce"`
}

// dpopNonceStep is how long a DPoP nonce is the current one; it is taken
// for that step and the one before, so a nonce lives two to four minutes.
const dpopNonceStep = 2 * time.Minute

// DPoPNonce is the nonce a DPoP proof must carry now (RFC 9449 §8): the time
// step, and a keyed MAC of it and the issuer under the installation's own
// key. It is kept nowhere, so any instance of the server checks any other's.
func (s *Service) DPoPNonce() string {
	return s.nonceAt(s.now().Unix() / int64(dpopNonceStep/time.Second))
}

func (s *Service) nonceAt(step int64) string {
	var at [8]byte
	binary.BigEndian.PutUint64(at[:], uint64(step))
	mac := s.sealer.Tag("dpop-nonce", append([]byte(s.issuer+" "), at[:]...))
	return base64.RawURLEncoding.EncodeToString(append(at[:], mac[:16]...))
}

// freshNonce says a nonce is this server's, for the current step or the one
// before.
func (s *Service) freshNonce(nonce string) bool {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(nonce)
	if err != nil || len(raw) != 24 {
		return false
	}
	step := int64(binary.BigEndian.Uint64(raw[:8]))
	now := s.now().Unix() / int64(dpopNonceStep/time.Second)
	if step != now && step != now-1 {
		return false
	}
	return hmac.Equal([]byte(s.nonceAt(step)), []byte(nonce))
}

// checkDPoP verifies a proof for one request (RFC 9449 §4.3) and returns its
// key's thumbprint. accessToken is the token it must name (ath), "" at the
// token endpoint. withNonce asks for this server's nonce in it (§8, §9), at
// the token, PAR and userinfo endpoints. A proof is accepted once.
func (s *Service) checkDPoP(ctx context.Context, proof, method, target, accessToken string, withNonce bool) (string, error) {
	key, err := s.verifyDPoP(ctx, proof, method, target, accessToken, withNonce)
	if err != nil {
		code := ErrInvalidDPoPProof
		if oauthErr, ok := errors.AsType[*authreq.Error](err); ok {
			code = oauthErr.Code
		}
		s.authreq.Metrics().DPoPFailure(ctx, code)
	}
	return key, err
}

func (s *Service) verifyDPoP(ctx context.Context, proof, method, target, accessToken string, withNonce bool) (string, error) {
	refused := func(why string) (string, error) {
		return "", authreq.OAuthError(ErrInvalidDPoPProof, why)
	}

	if proof == "" || strings.Contains(proof, ",") {
		return refused("one DPoP proof is required")
	}

	var claims dpopClaims
	header, err := jose.VerifyEmbedded(proof, &claims)
	switch {
	case err != nil:
		return refused("the DPoP proof's signature does not verify against its own key")
	case header.Type != dpopProofType:
		return refused("the DPoP proof's typ must be dpop+jwt")
	case !slices.Contains(DPoPAlgorithms, header.Algorithm):
		return refused("the DPoP proof is signed with an algorithm not offered")
	case claims.JTI == "" || len(claims.JTI) > maxDPoPJTI:
		return refused("the DPoP proof needs a jti")
	case claims.Method != method:
		return refused("the DPoP proof is for another method")
	case !sameTarget(claims.Target, target):
		return refused("the DPoP proof is for another address")
	}

	issued, err := claims.IssuedAt.Int64()
	if err != nil {
		return refused("the DPoP proof needs an iat")
	}
	if now := s.now(); time.Unix(issued, 0).Before(now.Add(-model.DPoPProofWindow)) || time.Unix(issued, 0).After(now.Add(model.DPoPProofWindow)) {
		return refused("the DPoP proof was not made just now")
	}

	// After every other check, so a client is asked for a nonce only for a
	// proof that would otherwise do; before its jti is spent, so the retry
	// with the nonce may use the same one.
	if withNonce && !s.freshNonce(claims.Nonce) {
		return "", authreq.OAuthError(ErrUseDPoPNonce, "a DPoP nonce is required: send the proof again with the one in the DPoP-Nonce header")
	}

	if accessToken != "" {
		sum := sha256.Sum256([]byte(accessToken))
		if claims.AccessToken != base64.RawURLEncoding.EncodeToString(sum[:]) {
			return refused("the DPoP proof is for another access token")
		}
	}

	thumbprint, err := jose.Thumbprint(*header.JWK)
	if err != nil {
		return refused("the DPoP proof's key is not one offered")
	}

	seen := sha256.Sum256([]byte(thumbprint + " " + claims.JTI))
	fresh, err := s.store.UseDPoPProof(ctx, hex.EncodeToString(seen[:]), s.now().Add(2*model.DPoPProofWindow))
	if err != nil {
		return "", err
	}
	if !fresh {
		return refused("the DPoP proof has already been used")
	}

	return thumbprint, nil
}

// isThumbprint says whether a value has the shape of a JWK SHA-256
// thumbprint (RFC 7638): 43 characters of unpadded base64url.
func isThumbprint(value string) bool {
	if len(value) != 43 {
		return false
	}
	sum, err := base64.RawURLEncoding.Strict().DecodeString(value)
	return err == nil && len(sum) == sha256.Size
}

// presented checks that a token bound to a key came with a proof from it,
// and that one not bound to a key did not come as DPoP. withNonce asks for
// this server's nonce in the proof (userinfo, RFC 9449 §9).
func (s *Service) presented(ctx context.Context, token string, raw map[string]any, how Presentation, withNonce bool) error {
	bound := boundKey(raw)

	switch {
	case bound == "" && strings.EqualFold(how.Scheme, TokenTypeDPoP):
		return authreq.OAuthError(authreq.ErrInvalidToken, "the access token is not bound to a DPoP key")
	case bound == "":
		return nil
	case !strings.EqualFold(how.Scheme, TokenTypeDPoP):
		return authreq.OAuthError(authreq.ErrInvalidToken, "the access token is bound to a DPoP key: send it as DPoP, with a proof")
	}

	thumbprint, err := s.checkDPoP(ctx, how.Proof, how.Method, how.URL, token, withNonce)
	if err != nil {
		return err
	}
	if thumbprint != bound {
		return authreq.OAuthError(ErrInvalidDPoPProof, "the DPoP proof is from another key than the token is bound to")
	}

	return nil
}

// boundKey is the thumbprint an access token is bound to, "" when it is not.
func boundKey(raw map[string]any) string {
	cnf, _ := raw["cnf"].(map[string]any)
	jkt, _ := cnf["jkt"].(string)
	return jkt
}

// sameTarget compares the address a proof names with the one it was sent to,
// without query or fragment (RFC 9449 §4.3, step 9).
func sameTarget(claimed, actual string) bool {
	a, errA := url.Parse(claimed)
	b, errB := url.Parse(actual)
	if errA != nil || errB != nil {
		return false
	}

	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host) && a.Path == b.Path
}
