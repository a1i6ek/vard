package keys

import (
	"testing"

	"github.com/a1i6ek/vard/internal/transport/http/spec"
	"github.com/a1i6ek/vard/internal/infrastructure/signing"
)

func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "SigningKey", signing.KeyInfo{})
}
