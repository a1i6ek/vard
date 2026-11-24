package sso

import (
	"testing"

	"github.com/a1i6ek/vard/internal/transport/http/spec"
)

// Each answer has the fields its schema in api/openapi declares, which both
// apps' types are generated from.
func TestAnswersMatchTheDescription(t *testing.T) {
	// The model sends its certificate as sp_certificate too; the panel reads
	// the copy under service_provider.
	spec.Match(t, "SSOConnection", connectionResponse{}, "sp_certificate")
	spec.Match(t, "SSOIdentityProvider", identityProvider{})
	spec.Match(t, "SSOTestResult", testResponse{})
	spec.Match(t, "SSODomainProof", domainProof{})
}
