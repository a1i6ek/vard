package mfa

import (
	"testing"

	"github.com/a1i6ek/vard/internal/admin"
	"github.com/a1i6ek/vard/internal/api/spec"
)

// Each answer has the fields its schema in api/openapi declares, which both
// apps' types are generated from.
func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "MfaStatus", auth.MFAStatus{})
	spec.Match(t, "MfaEnrolment", auth.Enrolment{})
	spec.Match(t, "AdminPasskey", auth.Passkey{})
}
