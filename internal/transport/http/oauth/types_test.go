package oauth

import (
	"testing"

	"github.com/a1i6ek/vard/internal/application/oauth"
	"github.com/a1i6ek/vard/internal/transport/http/spec"
)

func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "ClientMetadata", oauth.ClientMetadata{})
}
