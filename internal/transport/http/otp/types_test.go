package otp

import (
	"testing"

	"github.com/a1i6ek/vard/internal/transport/http/spec"
)

// Each answer has the fields its schema in api/openapi declares, which both
// apps' types are generated from.
func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "OTPSettings", settingsResponse{})
	spec.Match(t, "OTPFlow", usingFlow{})
	spec.Match(t, "OTPLimits", limits{})
	spec.Match(t, "OTPResponse", response{})
}
