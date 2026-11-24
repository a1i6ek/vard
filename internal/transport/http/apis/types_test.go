package apis

import (
	"testing"

	"github.com/a1i6ek/vard/internal/transport/http/spec"
)

// Each answer has the fields its schema in api/openapi declares, which both
// apps' types are generated from.
func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "API", apiResponse{})
	spec.Match(t, "APIScope", scopeResponse{})
	spec.Match(t, "APIApplication", applicationAccess{})
	spec.Match(t, "APIIntrospector", introspector{})
	spec.Match(t, "APIExchangeRule", exchangeRule{})
	spec.Match(t, "APILogEntry", logEntry{})
}
