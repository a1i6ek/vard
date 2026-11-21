package oauth

import (
	"testing"

	"github.com/a1i6ek/vard/internal/api/spec"
	"github.com/a1i6ek/vard/internal/oauth"
)

func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "ClientMetadata", oauth.ClientMetadata{})
}
