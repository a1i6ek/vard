package outbound

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
)

// KeyRefetchEvery is how often a token that does not verify may make the
// server fetch a key set again: often enough to follow a rotation, too seldom
// for forged tokens to make it fetch on each one.
const KeyRefetchEvery = time.Minute

// KeySets holds the key sets published at addresses someone else controls
// (an identity provider's, a client's jwks_uri) while they are fresh.
// forced is when each was last fetched again because a token did not verify,
// which anyone can cause by sending a bad one.
type KeySets struct {
	lifetime time.Duration
	timeout  time.Duration

	mu     sync.Mutex
	keys   map[string]fetchedKeys
	forced map[string]time.Time
}

type fetchedKeys struct {
	keys    []jose.Key
	fetched time.Time
}

// NewKeySets keeps each key set for lifetime, and gives a fetch timeout.
func NewKeySets(lifetime, timeout time.Duration) *KeySets {
	return &KeySets{
		lifetime: lifetime,
		timeout:  timeout,
		keys:     map[string]fetchedKeys{},
		forced:   map[string]time.Time{},
	}
}

// Verify checks a token's signature against the keys published at jwksURI
// and decodes its claims. A token naming a key not held, or naming none, may
// be signed with one just rotated in, so the keys are fetched again, at most
// once a KeyRefetchEvery. One naming a key held is simply refused: fetching
// again cannot help it, and it would spend the refetch a real rotation may
// need. client must be a NewFederationClient.
func (k *KeySets) Verify(ctx context.Context, client *http.Client, now func() time.Time, jwksURI, token string, claims any) error {
	keys, err := k.Keys(ctx, client, now, jwksURI, false)
	if err != nil {
		return err
	}

	// kid is optional in a JWS: a token without one may be signed by any of
	// the keys of its algorithm, and one with a kid only by that.
	candidates := func(keys []jose.Key) func(jose.Header) []jose.Key {
		return func(header jose.Header) []jose.Key {
			if header.KeyID == "" {
				return keys
			}
			for _, key := range keys {
				if key.ID == header.KeyID {
					return []jose.Key{key}
				}
			}
			return nil
		}
	}

	read, held := false, false
	first := func(header jose.Header) []jose.Key {
		found := candidates(keys)(header)
		read, held = true, header.KeyID != "" && len(found) > 0
		return found
	}
	if _, err := jose.VerifyAmong(token, first, claims); err == nil {
		return nil
	} else if !read || held {
		return fmt.Errorf("signature: %w", err)
	}
	if keys, err = k.Keys(ctx, client, now, jwksURI, true); err != nil {
		return err
	}
	if _, err := jose.VerifyAmong(token, candidates(keys), claims); err != nil {
		return fmt.Errorf("signature: %w", err)
	}

	return nil
}

// Keys is the key set published at jwksURI, from what is held while it is
// fresh. refresh asks for it to be fetched again, which happens at most once
// a KeyRefetchEvery per address; in between, the keys held are the answer.
func (k *KeySets) Keys(ctx context.Context, client *http.Client, now func() time.Time, jwksURI string, refresh bool) ([]jose.Key, error) {
	k.mu.Lock()
	hit, ok := k.keys[jwksURI]
	switch {
	case ok && !refresh && now().Sub(hit.fetched) < k.lifetime:
		k.mu.Unlock()
		return hit.keys, nil
	case refresh && now().Sub(k.forced[jwksURI]) < KeyRefetchEvery:
		k.mu.Unlock()
		if !ok {
			return nil, fmt.Errorf("the keys were fetched again less than %s ago", KeyRefetchEvery)
		}
		return hit.keys, nil
	case refresh:
		// Noted before fetching, so requests arriving while it is under way
		// do not fetch too, and a failed fetch counts as much as one that
		// worked.
		k.forced[jwksURI] = now()
	}
	k.mu.Unlock()

	var set struct {
		Keys []jose.JWK `json:"keys"`
	}
	if err := k.fetch(ctx, client, jwksURI, &set); err != nil {
		return nil, err
	}

	// A list, not a map by kid: keys published without one must not
	// overwrite each other.
	var keys []jose.Key
	for _, jwk := range set.Keys {
		if jwk.Use != "" && jwk.Use != "sig" {
			continue
		}
		// A key of a kind this server cannot verify with is skipped, not an
		// error: the publisher may publish others beside the one it uses.
		if key, err := jose.ParseJWK(jwk); err == nil {
			keys = append(keys, key)
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no key this server can verify with is published")
	}

	k.mu.Lock()
	k.keys[jwksURI] = fetchedKeys{keys: keys, fetched: now()}
	k.mu.Unlock()

	return keys, nil
}

func (k *KeySets) fetch(ctx context.Context, client *http.Client, address string, into any) error {
	ctx, cancel := context.WithTimeout(ctx, k.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %d: %.200s", request.URL.Host, response.StatusCode, body)
	}

	return json.Unmarshal(body, into)
}
