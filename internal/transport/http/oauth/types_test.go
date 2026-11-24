package oauth

import (
	"testing"

	"github.com/a1i6ek/vard/internal/transport/http/spec"
	"github.com/a1i6ek/vard/internal/application/oauth"
)

func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "ClientMetadata", oauth.ClientMetadata{})
}
