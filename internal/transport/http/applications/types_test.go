package applications

import (
	"testing"

	"github.com/a1i6ek/vard/internal/transport/http/spec"
)

// Each answer has the fields its schema in api/openapi declares, which both
// apps' types are generated from.
func TestAnswersMatchTheDescription(t *testing.T) {
	spec.Match(t, "Application", applicationResponse{})
	spec.Match(t, "ApplicationWithSecret", secretResponse{})
	spec.Match(t, "ApplicationPage", pageResponse{})
	spec.Match(t, "APIAccess", accessAPI{})
	spec.Match(t, "LogoutDelivery", logoutDelivery{})
}
