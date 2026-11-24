package keys

import (
	"testing"

	"github.com/a1i6ek/vard/internal/api/spec"
	"github.com/a1i6ek/vard/internal/signing"
)

func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "SigningKey", signing.KeyInfo{})
}
