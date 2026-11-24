package mfa

import (
	"testing"

	"github.com/a1i6ek/vard/internal/application/admin"
	"github.com/a1i6ek/vard/internal/transport/http/spec"
)

// Each answer has the fields its schema in api/openapi declares, which both
// apps' types are generated from.
func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "MfaStatus", auth.MFAStatus{})
	spec.Match(t, "MfaEnrolment", auth.Enrolment{})
	spec.Match(t, "AdminPasskey", auth.Passkey{})
}
