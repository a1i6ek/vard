package account

import (
	"testing"

	"github.com/a1i6ek/vard/internal/application/oauth"
	"github.com/a1i6ek/vard/internal/transport/http/spec"
)

// Each answer has the fields its schema in api/openapi declares, which both
// apps' types are generated from.
func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "DevicePage", oauth.DevicePage{})
	spec.Match(t, "DeviceOrigin", oauth.DeviceOrigin{})
	spec.Match(t, "ConsentPage", oauth.ConsentPage{})
}
